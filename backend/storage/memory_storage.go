package storage

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"

	"task1/backend/models"
)

type MemoryStorage struct {
	mu    sync.RWMutex
	files map[string]models.FileData
}

var defaultStorage = NewMemoryStorage()

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		files: make(map[string]models.FileData),
	}
}

func DefaultStorage() *MemoryStorage {
	return defaultStorage
}

func SaveFileData(data models.FileData) string {
	return defaultStorage.SaveFileData(data)
}

func GetFileData(fileID string) (models.FileData, bool) {
	return defaultStorage.GetFileData(fileID)
}

func (s *MemoryStorage) SaveFileData(data models.FileData) string {
	fileID := strings.TrimSpace(data.ID)
	if fileID == "" {
		fileID = generateFileID()
	}

	data.ID = fileID

	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[fileID] = data

	return fileID
}

func (s *MemoryStorage) GetFileData(fileID string) (models.FileData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.files[fileID]
	return data, ok
}

func generateFileID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}

	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
