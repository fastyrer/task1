// id.go - генерация идентификаторов локальных импортов без обращения к базе данных.
package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// GenerateUUID создаёт UUID v4 для идемпотентного финального импорта.
// Идентификатор генерируется до записи в PostgreSQL и хранится вместе с локальным черновиком.
func GenerateUUID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}

	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(buffer)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

// IsUUID проверяет формат идентификатора перед использованием в SQL-запросе импорта.
func IsUUID(value string) bool {
	return uuidPattern.MatchString(value)
}
