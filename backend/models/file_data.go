package models

type FileData struct {
	ID      string              `json:"id"`
	Headers []string            `json:"headers"`
	Rows    []map[string]string `json:"rows"`
}
