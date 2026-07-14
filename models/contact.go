package models

import "time"

type Contact struct {
	ID        string    `json:"id"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email,omitempty"`
	Name      string    `json:"name,omitempty"`
	Discount  string    `json:"discount,omitempty"`
	FileID    string    `json:"fileId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ContactEventAction - тип события, которое сохраняется в истории и источниках контакта.
type ContactEventAction string

// Константы совпадают с CHECK-ограничениями contact_sources и contact_versions в PostgreSQL.
const (
	ContactEventCreated  ContactEventAction = "created"
	ContactEventUpdated  ContactEventAction = "updated"
	ContactEventMatched  ContactEventAction = "matched"
	ContactEventSkipped  ContactEventAction = "skipped"
	ContactEventReplaced ContactEventAction = "replaced"
	ContactEventMerged   ContactEventAction = "merged"
	ContactEventFixed    ContactEventAction = "fixed"
)

type ConflictAction string // Тип для действия при конфликте (псевдоним)

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
