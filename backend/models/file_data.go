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
	ID      string              `json:"id"`
	Headers []string            `json:"headers"`
	Rows    []map[string]string `json:"rows"`
}
