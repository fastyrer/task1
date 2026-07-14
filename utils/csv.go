// csv.go – автоопределение разделителя CSV.

package utils

import "strings"

// DetectCSVDelimiter определяет наиболее вероятный разделитель CSV.
func DetectCSVDelimiter(content string) rune {
	// Первые 10 строк в нормализованном виде
	lines := sampleLines(content, 10)
	if len(lines) == 0 {
		return ','
	}

	for _, delim := range []rune{';', ',', '\t', '|'} {
		var count int
		consistent := true
		for i, line := range lines {
			c := strings.Count(line, string(delim))
			if c == 0 {
				consistent = false
				break
			}
			if i == 0 {
				count = c
			} else if c != count {
				consistent = false
				break
			}
		}
		if consistent {
			return delim
		}
	}
	// fallback-разделитель
	return ','
}

// ============================================================================
// sampleLines можно оптимизировать: принимать []byte вместо string, 
// сканировать \n через bytes.IndexByte или bufio.Scanner.
// ============================================================================

// sampleLines извлекает до limit непустых строк из содержимого,
// убирая \r и лишние пробелы.
func sampleLines(content string, limit int) []string {
	lines := make([]string, 0, limit)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) == limit {
			break
		}
	}
	return lines
}
