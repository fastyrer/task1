// Package models хранит структуры данных

package models

// FileData – структура для хранения информации о загруженных файлах
/*
	ID – уникальный id файла
	Headers – список заголовков колонок из файла
	Rows – словарь данных из файла, ключи – заголовки колонок, значения – данные
		этих колонок
*/
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
	Stats            ProcessingStats     `json:"stats"`
	Warnings         []ProcessingWarning `json:"warnings,omitempty"`
	InvalidRows      []InvalidRow        `json:"invalidRows,omitempty"`
}

type ProcessingStats struct {
	RowCount        int `json:"rowCount"`
	ColumnCount     int `json:"columnCount"`
	ValidRowCount   int `json:"validRowCount"`
	InvalidRowCount int `json:"invalidRowCount"`
	EmptyRowCount   int `json:"emptyRowCount"`
	SkippedRowCount int `json:"skippedRowCount"`
	WarningCount    int `json:"warningCount"`
}

type ProcessingWarning struct {
	Row     int    `json:"row,omitempty"`
	Column  string `json:"column,omitempty"`
	Message string `json:"message"`
}

type InvalidRow struct {
	Row    int                 `json:"row"`
	Values map[string]string   `json:"values"`
	Errors []ProcessingWarning `json:"errors"`
}
