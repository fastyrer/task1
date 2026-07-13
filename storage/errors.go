// errors.go - ожидаемые доменные ошибки PostgreSQL-хранилища.
package storage

import "errors"

var (
	// ErrContactAlreadyExists означает, что UNIQUE-ограничение телефона не дало создать дубликат.
	ErrContactAlreadyExists = errors.New("contact with this phone already exists")
	// ErrContactNotFound возвращается, когда UPDATE или разрешение конфликта не нашли контакт.
	ErrContactNotFound = errors.New("contact not found")
)
