// import.go - HTTP- и доменные модели безопасного импорта контактов.
package models

// ImportRow хранит одну строку локального черновика вместе с номером в исходном файле.
type ImportRow struct {
	RowNumber int               `json:"rowNumber" example:"2"`
	Values    map[string]string `json:"values"`
}

// ImportDraft содержит данные, которые до подтверждения хранятся только в браузере.
// Поля Stats, Warnings и InvalidRows сюда намеренно не входят: сервер вычисляет их заново.
type ImportDraft struct {
	ImportID         string      `json:"importId" example:"2f656bc0-6227-49d3-9d09-b2d59bd21c52"`
	OriginalFilename string      `json:"originalFilename" example:"clients.xlsx"`
	Size             int64       `json:"size" example:"18432"`
	MIMEType         string      `json:"mimeType,omitempty"`
	DetectedMIMEType string      `json:"detectedMimeType,omitempty"`
	Format           string      `json:"format" enums:"csv,xls,xlsx" example:"xlsx"`
	Encoding         string      `json:"encoding,omitempty" example:"UTF-8"`
	SheetName        string      `json:"sheetName,omitempty" example:"Клиенты"`
	Sheets           []string    `json:"sheets,omitempty"`
	HeaderRow        int         `json:"headerRow" example:"1"`
	Headers          []string    `json:"headers"`
	Rows             []ImportRow `json:"rows"`
	EmptyRowCount    int         `json:"emptyRowCount,omitempty"`
	SkippedRowCount  int         `json:"skippedRowCount,omitempty"`
}

// ImportValidationResult возвращает повторно нормализованный черновик и диагностику.
type ImportValidationResult struct {
	Draft               ImportDraft         `json:"draft"`
	PreviewRows         []map[string]string `json:"previewRows"`
	Stats               ProcessingStats     `json:"stats"`
	Warnings            []ProcessingWarning `json:"warnings,omitempty"`
	InvalidRows         []InvalidRow        `json:"invalidRows,omitempty"`
	DetectedPhoneColumn string              `json:"detectedPhoneColumn,omitempty" example:"Телефон"`
}

// ImportDecision фиксирует выбранное пользователем решение одного конфликта.
// Version защищает от применения решения к контакту, изменившемуся после предпросмотра.
type ImportDecision struct {
	Phone   string         `json:"phone" example:"+79991234567"`
	Action  ConflictAction `json:"action" enums:"skip,replace,merge" example:"merge"`
	Version string         `json:"version" example:"2026-07-16T08:30:00.123456Z"`
}

// ImportPreviewResult описывает последствия импорта, не изменяя PostgreSQL.
type ImportPreviewResult struct {
	NewCount      int            `json:"newCount" example:"90"`
	MatchedCount  int            `json:"matchedCount" example:"10"`
	SkippedCount  int            `json:"skippedCount" example:"2"`
	ConflictCount int            `json:"conflictCount" example:"4"`
	Conflicts     []ConflictInfo `json:"conflicts,omitempty"`
}

// ImportCommitResult возвращает количество операций завершённой транзакции.
type ImportCommitResult struct {
	ImportID string `json:"importId"`
	Created  int    `json:"created" example:"90"`
	Matched  int    `json:"matched" example:"10"`
	Skipped  int    `json:"skipped" example:"1"`
	Replaced int    `json:"replaced" example:"2"`
	Merged   int    `json:"merged" example:"1"`
}
