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
	mu       sync.RWMutex
	files    map[string]models.FileData
	contacts map[string]models.Contact
}

var defaultStorage = NewMemoryStorage()

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		files:    make(map[string]models.FileData),
		contacts: make(map[string]models.Contact),
	}
}

func DefaultStorage() *MemoryStorage {
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

func (s *MemoryStorage) SaveContact(_ context.Context, contact models.Contact) (string, error) {
	contactID := strings.TrimSpace(contact.ID)
	if contactID == "" {
		contactID = generateFileID()
	}
	contact.ID = contactID
	now := time.Now()
	if contact.CreatedAt.IsZero() {
		contact.CreatedAt = now
	}
	contact.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()
	s.contacts[contact.Phone] = contact

	return contactID, nil
}

func (s *MemoryStorage) GetContactByPhone(_ context.Context, phone string) (models.Contact, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	contact, ok := s.contacts[phone]
	return contact, ok, nil
}

func (s *MemoryStorage) ListContactsByFileID(_ context.Context, fileID string) ([]models.Contact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]models.Contact, 0)
	for _, c := range s.contacts {
		if c.FileID == fileID {
			result = append(result, c)
		}
	}

	return result, nil
}

func (s *MemoryStorage) UpdateContact(_ context.Context, contact models.Contact) error {
	contact.UpdatedAt = time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.contacts[contact.Phone] = contact

	return nil
}

func (s *MemoryStorage) ResolveConflict(_ context.Context, phone string, action models.ConflictAction, incoming models.Contact) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.contacts[phone]
	if !exists {
		incoming.ID = generateFileID()
		incoming.CreatedAt = time.Now()
		incoming.UpdatedAt = time.Now()
		s.contacts[phone] = incoming
		return nil
	}

	switch action {
	case models.ConflictActionSkip:
		return nil
	case models.ConflictActionReplace:
		incoming.ID = existing.ID
		incoming.CreatedAt = existing.CreatedAt
		incoming.UpdatedAt = time.Now()
		s.contacts[phone] = incoming
	case models.ConflictActionMerge:
		if incoming.Name == "" {
			incoming.Name = existing.Name
		}
		if incoming.Email == "" {
			incoming.Email = existing.Email
		}
		if incoming.Discount == "" {
			incoming.Discount = existing.Discount
		}
		if incoming.Data == nil {
			incoming.Data = existing.Data
		} else {
			for k, v := range existing.Data {
				if _, ok := incoming.Data[k]; !ok {
					incoming.Data[k] = v
				}
			}
		}
		incoming.ID = existing.ID
		incoming.CreatedAt = existing.CreatedAt
		incoming.UpdatedAt = time.Now()
		s.contacts[phone] = incoming
	}

	return nil
}

func generateFileID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}

	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
