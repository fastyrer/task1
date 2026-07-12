// csv.go – автоопределение разделителя CSV.
//
// CSV-файлы могут использовать разные разделители: запятая (,),
// точка с запятой (;), табуляция (\t), вертикальная черта (|).
// Этот файл анализирует первые строки и выбирает разделитель,
// который даёт наиболее согласованное количество полей.
package utils

import "strings"

// DetectCSVDelimiter выбирает наиболее вероятный разделитель по первым строкам CSV.
//
// Алгоритм:
//  1. Берёт до 10 непустых строк из начала файла.
//  2. Для каждого кандидата (,, ;, \t, |) считает score:
//     score = суммарное количество полей во всех строках
//           + (самая частая длина строки) × 5
//  3. Если у разделителя суммарное число полей == количество строк
//     (значит он не нашёл ни одного разделителя в строках), score = -1.
//  4. Побеждает кандидат с максимальным score.
//
// Если файл пустой или ни один разделитель не подошёл – возвращает запятую.
//
// Пример: строки "a;b;c" и "d;e;f" с разделителем ";":
//
//	fields = [3, 3], totalFields = 6, consistency = 2 (обе строки по 3 поля)
//	score = 6 + 2*5 = 16
func DetectCSVDelimiter(content string) rune {
	candidates := []rune{',', ';', '\t', '|'}
	lines := sampleCSVLines(content, 10)
	if len(lines) == 0 {
		return ','
	}

	bestDelimiter := ','
	bestScore := -1 << 30 // очень маленькое число, чтобы первый кандидат точно победил
	for _, delimiter := range candidates {
		score := delimiterScore(lines, delimiter)
		if score > bestScore {
			bestDelimiter = delimiter
			bestScore = score
		}
	}

	return bestDelimiter
}

// sampleCSVLines извлекает до limit непустых строк из CSV-контента,
// отбрасывая символы \r и лишние пробелы по краям.
//
// Возвращает пустой слайс, если в файле нет непустых строк.
// Используется только внутри DetectCSVDelimiter.
func sampleCSVLines(content string, limit int) []string {
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

// delimiterScore оценивает, насколько хорошо candidate-разделитель подходит
// для переданного набора строк.
//
// Идея: хороший разделитель даёт одинаковое количество полей в каждой строке
// (согласованность + штраф за редкие совпадения).
//
// Параметры:
//   - lines – выборка непустых строк из файла
//   - delimiter – тестируемый разделитель (, ; \t |)
//
// Возвращает score. Чем выше – тем лучше разделитель подходит.
func delimiterScore(lines []string, delimiter rune) int {
	counts := make(map[int]int) // сколько раз встретилось каждое значение fieldCount
	totalFields := 0
	for _, line := range lines {
		fieldCount := strings.Count(line, string(delimiter)) + 1
		counts[fieldCount]++
		totalFields += fieldCount
	}

	// Если разделитель ни разу не встретился – каждая строка даёт fieldCount=1,
	// totalFields == len(lines). Такой разделитель заведомо плох.
	if totalFields == len(lines) {
		return -1
	}

	// Самое частое количество полей – мера согласованности.
	bestConsistency := 0
	for _, count := range counts {
		if count > bestConsistency {
			bestConsistency = count
		}
	}

	return totalFields + bestConsistency*5
}
