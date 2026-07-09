package utils

import (
	"errors"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestDecodeCSVContent(t *testing.T) {
	text, encoding, err := DecodeCSVContent([]byte{0xEF, 0xBB, 0xBF, 'a', ',', 'b'})
	if err != nil || text != "a,b" || encoding != "UTF-8" {
		t.Fatalf("unexpected UTF-8 result: %q, %q, %v", text, encoding, err)
	}

	encoded, err := charmap.Windows1251.NewEncoder().Bytes([]byte("Имя;Город"))
	if err != nil {
		t.Fatalf("encode Windows-1251 fixture: %v", err)
	}
	text, encoding, err = DecodeCSVContent(encoded)
	if err != nil || text != "Имя;Город" || encoding != "Windows-1251" {
		t.Fatalf("unexpected Windows-1251 result: %q, %q, %v", text, encoding, err)
	}

	_, _, err = DecodeCSVContent([]byte{'a', 0x00, 'b'})
	if !errors.Is(err, ErrInvalidEncoding) {
		t.Fatalf("expected invalid encoding error, got %v", err)
	}
}
