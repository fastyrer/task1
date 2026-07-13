package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"task1/models"
	"task1/services"
	"task1/storage"
	"task1/utils"
)

type apiHolder struct {
	store    storage.Store
	contacts storage.ContactStore
}

func RegisterHumaRoutes(api huma.API, store storage.Store, contacts storage.ContactStore) {
	h := &apiHolder{store: store, contacts: contacts}

	huma.Register(api, huma.Operation{
		OperationID: "health-check",
		Method:      http.MethodGet,
		Path:        "/api/health",
		Summary:     "Проверка состояния сервера",
		Description: "Возвращает статус сервера и используемый драйвер хранилища. " +
			"Позволяет мониторить доступность базы данных через ping.",
		Tags: []string{"Мониторинг"},
	}, h.health)

	huma.Register(api, huma.Operation{
		OperationID: "upload-file",
		Method:      http.MethodPost,
		Path:        "/api/upload",
		Summary:     "Загрузка CSV/XLS/XLSX файла",
		Description: "Принимает файл и опционально имя листа (для Excel). " +
			"Парсит заголовки, данные, определяет формат/кодировку, возвращает " +
			"предпросмотр первых строк и предлагает колонку с телефоном.",
		Tags: []string{"Файлы"},
	}, h.upload)

	huma.Register(api, huma.Operation{
		OperationID: "search-data",
		Method:      http.MethodPost,
		Path:        "/api/search",
		Summary:     "Поиск по данным файла",
		Description: "Ищет строки по всем колонкам загруженного файла, " +
			"возвращает совпадения с подсветкой найденного текста. " +
			"Поддерживает ограничение количества результатов.",
		Tags: []string{"Файлы"},
	}, h.search)

	huma.Register(api, huma.Operation{
		OperationID: "preview-notifications",
		Method:      http.MethodPost,
		Path:        "/api/preview",
		Summary:     "Предпросмотр уведомлений",
		Description: "Генерирует уведомления по шаблону для указанной колонки с телефоном. " +
			"Шаблон может содержать плейсхолдеры в фигурных скобках, " +
			"которые заменяются на значения из соответствующих колонок строки. " +
			"Возвращает сгенерированные сообщения без сохранения.",
		Tags: []string{"Уведомления"},
	}, h.preview)

	huma.Register(api, huma.Operation{
		OperationID: "export-notifications",
		Method:      http.MethodPost,
		Path:        "/api/export",
		Summary:     "Экспорт уведомлений в CSV",
		Description: "Генерирует уведомления по шаблону и возвращает их в виде " +
			"CSV-файла с BOM (UTF-8). Столбцы: Телефон, Сообщение. " +
			"Файл отдаётся как вложение (Content-Disposition: attachment).",
		Tags: []string{"Уведомления"},
	}, h.export)

	huma.Register(api, huma.Operation{
		OperationID: "save-contacts",
		Method:      http.MethodPost,
		Path:        "/api/contacts/save",
		Summary:     "Сохранение контактов в базу",
		Description: "Извлекает номера телефонов из загруженного файла и сохраняет " +
			"их в базу данных. При обнаружении дубликатов возвращает список конфликтов " +
			"с указанием различающихся полей для ручного разрешения.",
		Tags: []string{"Контакты"},
	}, h.save)

	huma.Register(api, huma.Operation{
		OperationID: "resolve-conflict",
		Method:      http.MethodPost,
		Path:        "/api/contacts/resolve",
		Summary:     "Разрешение одного конфликта",
		Description: "Разрешает конфликт для указанного телефона. " +
			"Доступные действия: skip — пропустить (оставить существующую запись), " +
			"replace — заменить существующую запись, " +
			"merge — объединить поля (существующие + новые).",
		Tags: []string{"Контакты"},
	}, h.resolve)

	huma.Register(api, huma.Operation{
		OperationID: "resolve-all-conflicts",
		Method:      http.MethodPost,
		Path:        "/api/contacts/resolve-all",
		Summary:     "Массовое разрешение всех конфликтов",
		Description: "Применяет выбранное действие (skip / replace / merge) " +
			"ко всем конфликтам в файле автоматически. " +
			"Пропускает строки без телефона и записи, совпадающие с существующими.",
		Tags: []string{"Контакты"},
	}, h.resolveAll)

	huma.Register(api, huma.Operation{
		OperationID: "fix-invalid-rows",
		Method:      http.MethodPost,
		Path:        "/api/rows/fix",
		Summary:     "Исправление невалидных строк",
		Description: "Принимает список исправленных строк из интерфейса редактирования " +
			"и сохраняет их в базу данных. Каждая строка проверяется и " +
			"при успехе помечается как исправленная.",
		Tags: []string{"Контакты"},
	}, h.fixRows)
}

// --- Health ---

type healthOutput struct {
	Body struct {
		Status  string `json:"status" example:"ok" doc:"ok или degraded"`
		Storage string `json:"storage" example:"memory" doc:"используемый драйвер хранилища"`
		Error   string `json:"error,omitempty" doc:"описание ошибки при статусе degraded"`
	}
}

func (h *apiHolder) health(ctx context.Context, input *struct{}) (*healthOutput, error) {
	status := "ok"
	driver := "postgres"

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := h.store.Ping(pingCtx); err != nil {
		status = "degraded"
	}

	errMsg := ""
	if status == "degraded" {
		errMsg = "storage unavailable"
	}

	return &healthOutput{
		Body: struct {
			Status  string `json:"status" example:"ok" doc:"ok или degraded"`
			Storage string `json:"storage" example:"memory" doc:"используемый драйвер хранилища"`
			Error   string `json:"error,omitempty" doc:"описание ошибки при статусе degraded"`
		}{
			Status:  status,
			Storage: driver,
			Error:   errMsg,
		},
	}, nil
}

// --- Upload ---

type uploadFormData struct {
	File  huma.FormFile `form:"file" required:"true" contentType:"text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-excel" doc:"CSV, XLS или XLSX файл"`
	Sheet string        `form:"sheet" doc:"имя или номер листа для Excel (опционально)"`
}

type uploadInput struct {
	RawBody huma.MultipartFormFiles[uploadFormData]
}

type uploadOutput struct {
	Body uploadResponse
}

func (h *apiHolder) upload(ctx context.Context, input *uploadInput) (*uploadOutput, error) {
	form := input.RawBody.Data()
	if !form.File.IsSet {
		return nil, huma.Error400BadRequest("Файл не передан.")
	}
	defer form.File.Close()

	data, err := services.ParseByFilenameWithOptions(form.File, form.File.Filename, services.ParseOptions{
		SheetName: form.Sheet,
	})
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	data.OriginalFilename = form.File.Filename
	data.Size = form.File.Size

	fileID, err := h.store.SaveFileData(ctx, data)
	if err != nil {
		return nil, huma.Error500InternalServerError("Не удалось сохранить данные файла.")
	}
	data.ID = fileID

	return &uploadOutput{
		Body: uploadResponse{
			FileID:              fileID,
			OriginalFilename:    data.OriginalFilename,
			Size:                data.Size,
			MIMEType:            data.MIMEType,
			DetectedMIMEType:    data.DetectedMIMEType,
			Format:              data.Format,
			Encoding:            data.Encoding,
			SheetName:           data.SheetName,
			Sheets:              data.Sheets,
			HeaderRow:           data.HeaderRow,
			Headers:             data.Headers,
			PreviewRows:         previewRows(data.Rows),
			Stats:               data.Stats,
			Warnings:            data.Warnings,
			InvalidRows:         data.InvalidRows,
			DetectedPhoneColumn: utils.DetectPhoneColumn(data.Headers),
		},
	}, nil
}

// --- Search ---

type searchInput struct {
	Body struct {
		FileID string `json:"fileId" required:"true" minLength:"1" doc:"ID загруженного файла"`
		Query  string `json:"query"  required:"true" minLength:"1" doc:"поисковый запрос"`
		Limit  int    `json:"limit,omitempty" minimum:"1" default:"1000" doc:"максимум результатов"`
	}
}

type searchOutput struct {
	Body searchResponse
}

func (h *apiHolder) search(ctx context.Context, input *searchInput) (*searchOutput, error) {
	query := strings.TrimSpace(input.Body.Query)
	limit := searchLimit(input.Body.Limit)
	result, ok, err := h.store.SearchFileRows(ctx, input.Body.FileID, query, limit)
	if err != nil {
		return nil, huma.Error500InternalServerError("Не удалось выполнить поиск.")
	}
	if !ok {
		return nil, huma.Error404NotFound("Файл не найден. Загрузите файл снова.")
	}
	sr := searchStoredRows(result, query, limit)
	return &searchOutput{Body: sr}, nil
}

// --- Preview ---

type previewInput struct {
	Body struct {
		FileID      string `json:"fileId"      required:"true" minLength:"1" doc:"ID загруженного файла"`
		PhoneColumn string `json:"phoneColumn" required:"true" minLength:"1" doc:"название колонки с телефонами"`
		Template    string `json:"template"    required:"true" doc:"шаблон сообщения, например: «{Имя}, ваш код: {Код}»"`
	}
}

type previewOutput struct {
	Body previewResponse
}

func (h *apiHolder) preview(ctx context.Context, input *previewInput) (*previewOutput, error) {
	fd, ok, err := h.store.GetFileData(ctx, input.Body.FileID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Не удалось прочитать данные файла.")
	}
	if !ok {
		return nil, huma.Error404NotFound("Файл не найден. Загрузите файл снова.")
	}

	nh := &NotificationHandler{store: h.store}
	req := previewRequest{
		FileID:      input.Body.FileID,
		PhoneColumn: input.Body.PhoneColumn,
		Template:    input.Body.Template,
	}
	resp, err := nh.generate(fd, req)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	return &previewOutput{Body: resp}, nil
}

// --- Export ---

type exportInput struct {
	Body struct {
		FileID      string `json:"fileId"      required:"true" minLength:"1" doc:"ID загруженного файла"`
		PhoneColumn string `json:"phoneColumn" required:"true" minLength:"1" doc:"название колонки с телефонами"`
		Template    string `json:"template"    required:"true" doc:"шаблон сообщения"`
	}
}

func (h *apiHolder) export(ctx context.Context, input *exportInput) (*huma.StreamResponse, error) {
	fd, ok, err := h.store.GetFileData(ctx, input.Body.FileID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Не удалось прочитать данные файла.")
	}
	if !ok {
		return nil, huma.Error404NotFound("Файл не найден. Загрузите файл снова.")
	}

	nh := &NotificationHandler{store: h.store}
	req := previewRequest{
		FileID:      input.Body.FileID,
		PhoneColumn: input.Body.PhoneColumn,
		Template:    input.Body.Template,
	}
	gen, err := nh.generate(fd, req)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	return &huma.StreamResponse{
		Body: func(ctx huma.Context) {
			ctx.SetHeader("Content-Type", "text/csv; charset=utf-8")
			ctx.SetHeader("Content-Disposition", "attachment; filename=notifications.csv")

			var buf bytes.Buffer
			buf.Write([]byte{0xef, 0xbb, 0xbf})
			writer := csv.NewWriter(&buf)
			writer.Write([]string{"Телефон", "Сообщение"})
			for _, n := range gen.Notifications {
				writer.Write([]string{n.Phone, n.Text})
			}
			writer.Flush()

			ctx.BodyWriter().Write(buf.Bytes())
		},
	}, nil
}

// --- Save ---

type saveInput struct {
	Body struct {
		FileID string `json:"fileId" required:"true" minLength:"1" doc:"ID загруженного файла"`
	}
}

type saveOutput struct {
	Body struct {
		Saved     int                   `json:"saved"     doc:"количество сохранённых контактов"`
		Skipped   int                   `json:"skipped"   doc:"количество пропущенных дубликатов"`
		Conflicts []models.ConflictInfo `json:"conflicts,omitempty" doc:"список конфликтов для ручного разрешения"`
	}
}

func (h *apiHolder) save(ctx context.Context, input *saveInput) (*saveOutput, error) {
	fd, ok, err := h.store.GetFileData(ctx, input.Body.FileID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Не удалось прочитать данные файла.")
	}
	if !ok {
		return nil, huma.Error404NotFound("Файл не найден. Загрузите файл снова.")
	}

	phoneColumn := utils.DetectPhoneColumn(fd.Headers)
	if phoneColumn == "" {
		return nil, huma.Error400BadRequest("Не найдена колонка с телефоном.")
	}

	result, err := services.ProcessContacts(ctx, h.contacts, fd, phoneColumn)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := &saveOutput{}
	resp.Body.Saved = result.Saved
	resp.Body.Skipped = result.Skipped
	resp.Body.Conflicts = result.Conflicts
	return resp, nil
}

// --- Resolve ---

type resolveInput struct {
	Body struct {
		FileID string `json:"fileId" required:"true" minLength:"1" doc:"ID загруженного файла"`
		Phone  string `json:"phone"  required:"true" minLength:"1" doc:"номер телефона для разрешения конфликта"`
		Action string `json:"action" required:"true" enum:"skip,replace,merge" doc:"действие: skip — пропустить, replace — заменить, merge — объединить"`
	}
}

type resolveOutput struct {
	Body struct {
		Status string `json:"status" doc:"результат операции (ok)"`
		Phone  string `json:"phone"  doc:"номер телефона"`
		Action string `json:"action" doc:"применённое действие"`
	}
}

func (h *apiHolder) resolve(ctx context.Context, input *resolveInput) (*resolveOutput, error) {
	fd, ok, err := h.store.GetFileData(ctx, input.Body.FileID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Не удалось прочитать данные файла.")
	}
	if !ok {
		return nil, huma.Error404NotFound("Файл не найден.")
	}

	phoneColumn := utils.DetectPhoneColumn(fd.Headers)
	if phoneColumn == "" {
		return nil, huma.Error400BadRequest("Не найдена колонка с телефоном.")
	}

	var incoming models.Contact
	var found bool
	for _, row := range fd.Rows {
		if row[phoneColumn] == input.Body.Phone {
			incoming = services.RowToContact(row, input.Body.Phone, fd.ID)
			found = true
			break
		}
	}
	if !found {
		return nil, huma.Error404NotFound("Запись с таким телефоном не найдена в файле.")
	}

	if err := h.contacts.ResolveConflict(ctx, input.Body.Phone, models.ConflictAction(input.Body.Action), incoming); err != nil {
		return nil, huma.Error500InternalServerError("Не удалось разрешить конфликт.")
	}

	resp := &resolveOutput{}
	resp.Body.Status = "ok"
	resp.Body.Phone = input.Body.Phone
	resp.Body.Action = input.Body.Action
	return resp, nil
}

// --- Resolve All ---

type resolveAllInput struct {
	Body struct {
		FileID string `json:"fileId" required:"true" minLength:"1" doc:"ID загруженного файла"`
		Action string `json:"action" required:"true" enum:"skip,replace,merge" doc:"действие для всех конфликтов"`
	}
}

type resolveAllOutput struct {
	Body struct {
		Status   string `json:"status"   doc:"результат операции (ok)"`
		Resolved int    `json:"resolved" doc:"количество разрешённых конфликтов"`
		Action   string `json:"action"   doc:"применённое действие"`
	}
}

func (h *apiHolder) resolveAll(ctx context.Context, input *resolveAllInput) (*resolveAllOutput, error) {
	fd, ok, err := h.store.GetFileData(ctx, input.Body.FileID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Не удалось прочитать данные файла.")
	}
	if !ok {
		return nil, huma.Error404NotFound("Файл не найден.")
	}

	phoneColumn := utils.DetectPhoneColumn(fd.Headers)
	if phoneColumn == "" {
		return nil, huma.Error400BadRequest("Не найдена колонка с телефоном.")
	}

	resolved := 0
	action := models.ConflictAction(input.Body.Action)
	for _, row := range fd.Rows {
		phone := row[phoneColumn]
		if phone == "" {
			continue
		}
		existing, exists, err := h.contacts.GetContactByPhone(ctx, phone)
		if err != nil {
			return nil, huma.Error500InternalServerError("Не удалось проверить контакт.")
		}
		if !exists {
			continue
		}
		incoming := services.RowToContact(row, phone, fd.ID)
		if services.ContactsEqual(existing, incoming) {
			continue
		}
		if err := h.contacts.ResolveConflict(ctx, phone, action, incoming); err != nil {
			continue
		}
		resolved++
	}

	r := &resolveAllOutput{}
	r.Body.Status = "ok"
	r.Body.Resolved = resolved
	r.Body.Action = input.Body.Action
	return r, nil
}

// --- Fix Rows ---

type fixRowsInput struct {
	Body struct {
		FileID string                 `json:"fileId" required:"true" minLength:"1" doc:"ID загруженного файла"`
		Rows   []services.FixRowInput `json:"rows"   required:"true" minLength:"1" doc:"список исправленных строк"`
	}
}

type fixRowsOutput struct {
	Body services.FixRowResult
}

func (h *apiHolder) fixRows(ctx context.Context, input *fixRowsInput) (*fixRowsOutput, error) {
	fd, ok, err := h.store.GetFileData(ctx, input.Body.FileID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Не удалось прочитать данные файла.")
	}
	if !ok {
		return nil, huma.Error404NotFound("Файл не найден.")
	}

	phoneColumn := utils.DetectPhoneColumn(fd.Headers)
	if phoneColumn == "" {
		return nil, huma.Error400BadRequest("Не найдена колонка с телефоном.")
	}

	result := services.FixRowResult{}
	for _, row := range input.Body.Rows {
		if err := services.FixAndSaveRow(ctx, h.contacts, row, fd.Headers, phoneColumn, fd.ID); err != nil {
			result.Failed = append(result.Failed, *err)
		} else {
			result.Fixed++
		}
	}

	return &fixRowsOutput{Body: result}, nil
}
