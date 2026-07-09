package utils

import "strings"

// DetectCSVDelimiter выбирает наиболее вероятный разделитель по первым строкам CSV.
func DetectCSVDelimiter(content string) rune {
	candidates := []rune{',', ';', '\t', '|'}
	lines := sampleCSVLines(content, 10)
	if len(lines) == 0 {
		return ','
	}

	bestDelimiter := ','
	bestScore := -1 << 30
	for _, delimiter := range candidates {
		score := delimiterScore(lines, delimiter)
		if score > bestScore {
			bestDelimiter = delimiter
			bestScore = score
		}
	}

	return bestDelimiter
}

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

func delimiterScore(lines []string, delimiter rune) int {
	counts := make(map[int]int)
	totalFields := 0
	for _, line := range lines {
		fieldCount := strings.Count(line, string(delimiter)) + 1
		counts[fieldCount]++
		totalFields += fieldCount
	}

	if totalFields == len(lines) {
		return -1
	}

	bestConsistency := 0
	for _, count := range counts {
		if count > bestConsistency {
			bestConsistency = count
		}
	}

	return totalFields + bestConsistency*5
}
