// errors.go - ожидаемые доменные ошибки PostgreSQL-хранилища.
package storage

import "errors"

var (
	// ErrContactAlreadyExists означает, что UNIQUE-ограничение телефона не дало создать дубликат.
	ErrContactAlreadyExists = errors.New("Контакт с таким телефоном уже есть")
	// ErrContactNotFound возвращается, когда UPDATE или разрешение конфликта не нашли контакт.
	ErrContactNotFound = errors.New("Контакт не найден")
	// ErrFileRowNotFixable защищает endpoint исправления от изменения отсутствующей или уже валидной строки.
	ErrFileRowNotFixable = errors.New("Строка не найдена или уже является валидной")
)
