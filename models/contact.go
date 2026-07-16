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
	FileID   string `json:"-"`
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
	Row         int               `json:"row" example:"4"`
	Phone       string            `json:"phone" example:"+79991234567"`
	Version     string            `json:"version" example:"2026-07-16T08:30:00.123456Z"`
	Existing    map[string]string `json:"existing"`    // То, что уже лежит в базе
	Incoming    map[string]string `json:"incoming"`    // То, что прислал фронт
	Differences []string          `json:"differences"` // Отличные поля
	Actions     []ConflictAction  `json:"actions"`
}

// ContactPage содержит одну страницу подтверждённого справочника PostgreSQL.
type ContactPage struct {
	Items      []Contact `json:"items"`
	Page       int       `json:"page" example:"1"`
	PageSize   int       `json:"pageSize" example:"25"`
	Total      int64     `json:"total" example:"106"`
	TotalPages int       `json:"totalPages" example:"5"`
	Query      string    `json:"query,omitempty" example:"+7999"`
}

// ContactUpdateRequest содержит только изменяемые поля справочника.
// Version соответствует updatedAt, который пользователь видел перед редактированием.
type ContactUpdateRequest struct {
	Phone    string `json:"phone" example:"+79991234567"`
	Email    string `json:"email" example:"client@example.com"`
	Name     string `json:"name" example:"Иванов Иван"`
	Discount string `json:"discount" example:"10"`
	Version  string `json:"version" example:"2026-07-16T08:30:00.123456Z"`
}
