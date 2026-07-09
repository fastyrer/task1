package storage

import (
	"context"
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

func DefaultStorage() FileStore {
	return defaultStorage
}

func SaveFileData(ctx context.Context, data models.FileData) (string, error) {
	return defaultStorage.SaveFileData(ctx, data)
}

func GetFileData(ctx context.Context, fileID string) (models.FileData, bool, error) {
	return defaultStorage.GetFileData(ctx, fileID)
}

func (s *MemoryStorage) SaveFileData(_ context.Context, data models.FileData) (string, error) {
	fileID := strings.TrimSpace(data.ID)
	if fileID == "" {
		fileID = generateFileID()
	}

	data.ID = fileID

	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[fileID] = data

	return fileID, nil
}

func (s *MemoryStorage) GetFileData(_ context.Context, fileID string) (models.FileData, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.files[fileID]
	return data, ok, nil
}

func (s *MemoryStorage) Ping(_ context.Context) error {
	return nil
}

func (s *MemoryStorage) Close() {
}

func (s *MemoryStorage) Driver() string {
	return "memory"
}

func generateFileID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}

	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
