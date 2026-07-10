package utils

import "testing"

func TestDetectCSVDelimiter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    rune
	}{
		{name: "comma", content: "Имя,Город\nАнна,Москва\n", want: ','},
		{name: "semicolon", content: "Имя;Город\nАнна;Москва\n", want: ';'},
		{name: "tab", content: "Имя\tГород\nАнна\tМосква\n", want: '\t'},
		{name: "pipe", content: "Имя|Город\nАнна|Москва\n", want: '|'},
		{name: "empty", content: "\n\r\n", want: ','},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DetectCSVDelimiter(test.content); got != test.want {
				t.Fatalf("DetectCSVDelimiter() = %q; want %q", got, test.want)
			}
		})
	}
}
