package services

const (
	// DefaultSearchResultLimit – лимит поиска по умолчанию (1000 строк).
	// Переопределяется через переменную окружения SEARCH_RESULT_LIMIT.
	DefaultSearchResultLimit = 1000
)

// Телефон
const (
	
	ErrorPhoneEmpty       = "Пустой номер телефона"
	ErrorPhoneColNotFound = "Не найдена колонка с таким телефоном"
	ErrorPhoneNotFound    = "Запись с таким телефоном не найдена в файле"
	ErrorPhoneIncorrect = "Некорректный номер телефона"
)

// email
const (
	ErrorEmailIncorrect = "Некорректный email"
)

// Скидка
const (
	ErrorDiscountIncorrect = "Скидка должна быть числом от 0 до 100"
)

// Контакты.
const (
	ErrorContactsNotRead = "Не удалось прочитать контакты из базы данных"
	ErrorContactAlreadyExists = "Контакт с таким телефоном уже существует"
)

const (
	ErrorCSVNotCreated = "Не удалось сформировать CSV"
)

// Файлы.
const (
	ErrorFileNotOpened     = "Не удалось прочитать данные файла"
	ErrorFileNotFound      = "Файл не найден. Загрузите файл снова"
	ErrorFileExcessiveSize = "Файл слишком большой. Максимальный размер: "
	ErrorFileAbsent        = "Файл не передан"
	ErrorFileNotSaved      = "Не удалось сохранить файл"
)

// Уведомление
const (
	ErrorTemplateEmpty    = "Шаблон уведомления не может быть пустым"

)

// Общие ошибки HTTP и бизнес-логики, не привязанные к конкретному ресурсу.
const (
	ErrorMethodNotAllowed  = "Метод не поддерживается"
	ErrorBadRequest        = "Неверный формат запроса"
	ErrorEmptyRequestLine  = "Введите строку поиска"
	ErrorUnsupportedAction = "Неизвестное действие. Допустимые: skip, replace, merge"
	ErrorConflictNotSolved = "Не удалось разрешить конфликт"
)
