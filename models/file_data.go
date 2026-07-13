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
	RowCount        int `json:"rowCount"`
	ColumnCount     int `json:"columnCount"`
	ValidRowCount   int `json:"validRowCount"`
	InvalidRowCount int `json:"invalidRowCount"`
	EmptyRowCount   int `json:"emptyRowCount"`
	SkippedRowCount int `json:"skippedRowCount"`
	WarningCount    int `json:"warningCount"`
}

// ProcessingWarning хранит данные конкретной ошибки или поля
type ProcessingWarning struct {
	Row     int    `json:"row,omitempty"`
	Column  string `json:"column,omitempty"`
	Message string `json:"message"`
}

// InvalidRow хранит данные строки, не прошедшей валидацию
type InvalidRow struct {
	Row    int                 `json:"row"`
	Values map[string]string   `json:"values"`
	Errors []ProcessingWarning `json:"errors"`
}

// StoredFileRow - строка, найденная PostgreSQL-поиском внутри загруженного файла.
type StoredFileRow struct {
	Row    int               `json:"row"`    // Номер строки в исходном CSV/XLS/XLSX.
	Values map[string]string `json:"values"` // Полный набор ячеек найденной строки.
}

// FileSearchResult - внутренний результат поиска: заголовки, найденные строки и их полное количество.
type FileSearchResult struct {
	Headers []string        // Заголовки нужны обработчику для поиска совпавших ячеек.
	Rows    []StoredFileRow // Результаты, уже ограниченные переданным LIMIT.
	Total   int             // Полное число совпадений до LIMIT.
}
