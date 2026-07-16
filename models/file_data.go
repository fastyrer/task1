// Package models содержит структуры данных, используемые всеми пакетами проекта.
//
// file_data.go – модели для хранения результатов парсинга файлов.

package models

// FileData – структура для хранения информации о загруженных файлах
type FileData struct {
	ID               string              `json:"id"`
	OriginalFilename string              `json:"originalFilename,omitempty"`
	Size             int64               `json:"size,omitempty"`
	MIMEType         string              `json:"mimeType,omitempty"`
	DetectedMIMEType string              `json:"detectedMimeType,omitempty"`
	Format           string              `json:"format,omitempty"`
	Encoding         string              `json:"encoding,omitempty"`
	SheetName        string              `json:"sheetName,omitempty"`
	Sheets           []string            `json:"sheets,omitempty"`
	HeaderRow        int                 `json:"headerRow,omitempty"`
	Headers          []string            `json:"headers"`
	Rows             []map[string]string `json:"rows"`
	RowNumbers       []int               `json:"-"`
	Stats            ProcessingStats     `json:"stats"`
	Warnings         []ProcessingWarning `json:"warnings,omitempty"`
	InvalidRows      []InvalidRow        `json:"invalidRows,omitempty"`
}

// ProcessingStats хранит все количественные данные (счетчики) для сводки
type ProcessingStats struct {
	RowCount        int `json:"rowCount" example:"100"`
	ColumnCount     int `json:"columnCount" example:"4"`
	ValidRowCount   int `json:"validRowCount" example:"96"`
	InvalidRowCount int `json:"invalidRowCount" example:"3"`
	EmptyRowCount   int `json:"emptyRowCount" example:"1"`
	SkippedRowCount int `json:"skippedRowCount" example:"1"`
	WarningCount    int `json:"warningCount" example:"3"`
}

// ProcessingWarning хранит данные конкретной ошибки или поля
type ProcessingWarning struct {
	Row     int    `json:"row,omitempty" example:"4"`
	Column  string `json:"column,omitempty" example:"Email"`
	Message string `json:"message" example:"Некорректный email."`
}

// InvalidRow хранит данные строки, не прошедшей валидацию
type InvalidRow struct {
	Row    int                 `json:"row" example:"4"`
	Values map[string]string   `json:"values"`
	Errors []ProcessingWarning `json:"errors"`
}
