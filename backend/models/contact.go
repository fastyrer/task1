package models

/*
	Сохранение строк из загруженных файлов как структурированных записей с дедупликацией по телефону.
*/

import "time"

// Один контакт — одна строка из файла, приведённая к фиксированным полям
type Contact struct {
	ID        string            `json:"id"`
	Phone     string            `json:"phone"`
	Email     string            `json:"email,omitempty"`
	Name      string            `json:"name,omitempty"`
	Discount  string            `json:"discount,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
	FileID    string            `json:"fileId"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type ConflictAction string // Тип для действия при конфликте (псевдоним)

// Убрать?
// Что делать, если контакт с таким телефоном уже есть
const (
	ConflictActionSkip    ConflictAction = "skip"
	ConflictActionReplace ConflictAction = "replace"
	ConflictActionMerge   ConflictAction = "merge"
)

// Информация об одном конфликте
type ConflictInfo struct {
	Row        int               `json:"row"`
	Phone      string            `json:"phone"`
	Existing   map[string]string `json:"existing"` // То, что уже лежит в базе
	Incoming   map[string]string `json:"incoming"`	// То, что прислал фронт
	Differences []string         `json:"differences"` // Отличные поля
	Actions    []ConflictAction  `json:"actions"`
}

// Запросы на разрешение

// ResolveRequest разрешает конфликт для одного телефона
type ResolveRequest struct {
	FileID string        `json:"fileId"`
	Phone  string        `json:"phone"`
	Action ConflictAction `json:"action"`
}

// BatchResolveRequest разрешает все конфликты в одном файле за один раз
type BatchResolveRequest struct {
	FileID string        `json:"fileId"`
	Action ConflictAction `json:"action"`
}
