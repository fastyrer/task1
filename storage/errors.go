// errors.go - ожидаемые доменные ошибки PostgreSQL-хранилища.
package storage

import "errors"

var (
	// ErrImportAlreadyCommitted защищает от повторной отправки одного локального черновика.
	ErrImportAlreadyCommitted = errors.New("Этот импорт уже был сохранён")
	// ErrImportChanged требует повторного preview, если контакты изменились перед commit.
	ErrImportChanged = errors.New("Контакты изменились после проверки конфликтов; выполните проверку ещё раз")
	// ErrContactChanged защищает ручное редактирование от потери параллельных изменений.
	ErrContactChanged = errors.New("Контакт уже изменился; обновите список и повторите редактирование")
	// ErrContactPhoneExists сообщает о нарушении уникальности рабочего номера телефона.
	ErrContactPhoneExists = errors.New("Контакт с таким номером телефона уже существует")
)
