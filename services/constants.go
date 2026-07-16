package services

const ErrorPhoneColNotFound = "Не найдена колонка с телефоном"

// Контакты.
const (
	ErrorContactsNotRead     = "Не удалось прочитать контакты из базы данных"
	ErrorContactNotUpdated   = "Не удалось сохранить изменения контакта"
	ErrorContactUIDInvalid   = "Некорректный идентификатор контакта"
	ErrorContactQueryTooLong = "Поисковый запрос не должен превышать 200 символов"
)

const (
	ErrorCSVNotCreated = "Не удалось сформировать CSV"
)

// Файлы.
const (
	ErrorFileNotOpened      = "Не удалось прочитать данные файла"
	ErrorFileExcessiveSize  = "Файл слишком большой. Максимальный размер: "
	ErrorFileAbsent         = "Файл не передан"
	ErrorFileNotSaved       = "Не удалось сохранить файл"
	ErrorImportIDNotCreated = "Не удалось создать идентификатор импорта"
)

// Уведомление
const (
	ErrorTemplateEmpty = "Шаблон уведомления не может быть пустым"
)

// Общие ошибки HTTP и бизнес-логики, не привязанные к конкретному ресурсу.
const (
	ErrorMethodNotAllowed = "Метод не поддерживается"
	ErrorBadRequest       = "Неверный формат запроса"
)
