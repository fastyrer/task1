// Package models хранит структуры данных
//
// contact.go – Сохранение строк из загруженных файлов как структурированных
// записей с дедупликацией по телефону.

package models

import "time"

// Один контакт — одна строка из файла, приведённая к фиксированным полям
type Contact struct {
	// ID - внутренний автоинкрементный ключ PostgreSQL; наружу через API не отдаётся.
	ID int64 `json:"-"`
	// UID - публичный UUID контакта, который генерирует PostgreSQL.
	UID      string `json:"uid"`
	Phone    string `json:"phone"`
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
	Discount string `json:"discount,omitempty"`
	FileID   string `json:"fileId"`
	// SourceRow хранит номер строки для связи контакта с файлом-источником.
	SourceRow int       `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ContactSourceAction - результат обработки контакта из конкретной строки файла.
type ContactSourceAction string

// Константы совпадают с CHECK-ограничением contact_sources в PostgreSQL.
const (
	ContactSourceCreated  ContactSourceAction = "created"
	ContactSourceMatched  ContactSourceAction = "matched"
	ContactSourceSkipped  ContactSourceAction = "skipped"
	ContactSourceReplaced ContactSourceAction = "replaced"
	ContactSourceMerged   ContactSourceAction = "merged"
	ContactSourceFixed    ContactSourceAction = "fixed"
)

// ConflictAction - действие пользователя при несовпадении данных одного телефона.
// Тип остаётся в модели, потому что это общий контракт handlers, services и storage.
type ConflictAction string

const (
	ConflictActionSkip    ConflictAction = "skip"
	ConflictActionReplace ConflictAction = "replace"
	ConflictActionMerge   ConflictAction = "merge"
)

// Информация об одном конфликте
type ConflictInfo struct {
	Row         int               `json:"row"`
	Phone       string            `json:"phone"`
	Existing    map[string]string `json:"existing"`    // То, что уже лежит в базе
	Incoming    map[string]string `json:"incoming"`    // То, что прислал фронт
	Differences []string          `json:"differences"` // Отличные поля
	Actions     []ConflictAction  `json:"actions"`
}

// Запросы на разрешение

// ResolveRequest разрешает конфликт для одного телефона
type ResolveRequest struct {
	FileID string         `json:"fileId"`
	Phone  string         `json:"phone"`
	Action ConflictAction `json:"action"`
}

// BatchResolveRequest разрешает все конфликты в одном файле за один раз
type BatchResolveRequest struct {
	FileID string         `json:"fileId"`
	Action ConflictAction `json:"action"`
}
