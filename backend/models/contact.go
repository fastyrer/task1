package models

import "time"

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

type ConflictAction string

const (
	ConflictActionSkip    ConflictAction = "skip"
	ConflictActionReplace ConflictAction = "replace"
	ConflictActionMerge   ConflictAction = "merge"
)

type ConflictInfo struct {
	Row        int               `json:"row"`
	Phone      string            `json:"phone"`
	Existing   map[string]string `json:"existing"`
	Incoming   map[string]string `json:"incoming"`
	Differences []string         `json:"differences"`
	Actions    []ConflictAction  `json:"actions"`
}

type ResolveRequest struct {
	FileID string        `json:"fileId"`
	Phone  string        `json:"phone"`
	Action ConflictAction `json:"action"`
}

type BatchResolveRequest struct {
	FileID string        `json:"fileId"`
	Action ConflictAction `json:"action"`
}
