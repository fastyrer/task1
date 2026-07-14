package services

const (
	// DefaultSearchResultLimit – лимит поиска по умолчанию (1000 строк).
	// Переопределяется через переменную окружения SEARCH_RESULT_LIMIT.
	DefaultSearchResultLimit = 1000
)

// CSV


// Телефон
const (
	ErrorPhoneEmptyTemplate = "Шаблон уведомления не может быть пустым"
	ErrorPhoneEmpty         = "Пустой номер телефона"
	ErrorPhoneTooShort      = "Номер телефона слишком короткий (менее 7 цифр)"
	ErrorPhoneColNotFound   = "Не найдена колонка с телефоном"
	ErrorPhoneNotFound      = "Запись с таким телефоном не найдена в файле"
)

// Файлы.
const (
	ErrorFileNotOpened       = "Не удалось прочитать данные файла"
	ErrorFileNotFound        = "Файл не найден. Загрузите файл снова"
	ErrorFileExcessiveSize   = "Файл слишком большой. Максимальный размер: "
	ErrorFileAbsent          = "Файл не передан"
	ErrorFileNotSaved        = "Не удалось сохранить файл"
)

// Общие ошибки HTTP и бизнес-логики, не привязанные к конкретному ресурсу.
const (
	ErrorMethodNotAllowed    = "Метод не поддерживается"
	ErrorBadRequest          = "Неверный формат запроса"
	ErrorEmptyRequestLine    = "Введите строку поиска"
	ErrorUnsupportedAction   = "Неизвестное действие. Допустимые: skip, replace, merge"
	ErrorConflictNotSolved   = "Не удалось разрешить конфликт"
)