package models

import "time"

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
	// SourceRow хранит исходный номер строки файла для аудита и не отдаётся в JSON API.
	SourceRow int       `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ContactEventAction - тип события, которое сохраняется в истории и источниках контакта.
type ContactEventAction string

// Константы совпадают с CHECK-ограничениями contact_sources и contact_versions в PostgreSQL.
const (
	ContactEventCreated  ContactEventAction = "created"
	ContactEventMatched  ContactEventAction = "matched"
	ContactEventSkipped  ContactEventAction = "skipped"
	ContactEventReplaced ContactEventAction = "replaced"
	ContactEventMerged   ContactEventAction = "merged"
	ContactEventFixed    ContactEventAction = "fixed"
)

// ConflictAction - действие пользователя при несовпадении данных одного телефона.
// Тип остаётся в модели, потому что это общий контракт handlers, services и storage.
type ConflictAction string

const (
	ConflictActionSkip    ConflictAction = "skip"
	ConflictActionReplace ConflictAction = "replace"
	ConflictActionMerge   ConflictAction = "merge"
)

type ConflictInfo struct {
	Row         int               `json:"row"`
	Phone       string            `json:"phone"`
	Existing    map[string]string `json:"existing"`    // То, что уже лежит в базе
	Incoming    map[string]string `json:"incoming"`    // То, что прислал фронт
	Differences []string          `json:"differences"` // Отличные поля
	Actions     []ConflictAction  `json:"actions"`
}

type ResolveRequest struct {
	FileID string         `json:"fileId"`
	Phone  string         `json:"phone"`
	Action ConflictAction `json:"action"`
}

type BatchResolveRequest struct {
	FileID string         `json:"fileId"`
	Action ConflictAction `json:"action"`
}
