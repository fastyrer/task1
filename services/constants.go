package services

const (
	// Максимальное количество строк, в которых проходит поиск
	DefaultSearchResultLimit = 1000

	// Телефоны
	ErrorPhoneEmptyTemplate = "Шаблон уведомления не может быть пустым"
	ErrorPhoneEmpty = "Пустой номер телефона"
	ErrorPhoneTooShort = "Номер телефона слишком короткий (менее 7 цифр)"
	ErrorPhoneColNotFound = "Не найдена колонка с телефоном"
	ErrorPhoneNotFound = "Запись с таким телефоном не найдена в файле"

	// 
	ErrorMethodNotAllowed = "Метод не поддерживается"
	ErrorBadRequest = "Неверный формат запроса"
	ErrorEmptyRequestLine = "Введите строку поиска"
	ErrorUnsupportedAction = "Неизвестное действие. Допустимые: skip, replace, merge"
	ErrorConflictNotSolved = "Не удалось разрешить конфликт"

	// Файлы
	ErrorFileNotOpened = "Не удалось прочитать данные файла"
	ErrorFileNotFound = "Файл не найден. Загрузите файл снова"
	ErrorFileExcessiveSize = "Файл слишком большой. Максимальный размер: "
	ErrorFileAbsent = "Файл не передан"
	ErrorFileNotSaved = "Не удалось сохранить файл"
)