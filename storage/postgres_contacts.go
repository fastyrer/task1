package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"task1/models"
)

type rowScanner interface {
	Scan(dest ...any) error
}

func (s *PostgresStorage) SaveContact(ctx context.Context, contact models.Contact) (string, error) {
	const insertContactQuery = `
		INSERT INTO contacts (id, phone, email, name, discount, data)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (phone) DO NOTHING
		RETURNING created_at, updated_at
	`

	contactID := strings.TrimSpace(contact.ID)
	if contactID == "" {
		var err error
		contactID, err = generateID()
		if err != nil {
			return "", err
		}
	}
	contact.ID = contactID

	dataJSON, err := marshalContactData(contact.Data)
	if err != nil {
		return "", err
	}

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	tx, err := s.pool.Begin(queryCtx)
	if err != nil {
		return "", fmt.Errorf("begin save contact transaction: %w", err)
	}
	defer tx.Rollback(queryCtx)

	err = tx.QueryRow(queryCtx, insertContactQuery,
		contact.ID,
		contact.Phone,
		contact.Email,
		contact.Name,
		contact.Discount,
		dataJSON,
	).Scan(
		&contact.CreatedAt,
		&contact.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrContactAlreadyExists
	}
	if err != nil {
		return "", fmt.Errorf("insert contact: %w", err)
	}

	if err := saveContactVersionTx(queryCtx, tx, contact, models.ContactEventCreated); err != nil {
		return "", err
	}
	if err := saveContactSourceTx(queryCtx, tx, contact, models.ContactEventCreated); err != nil {
		return "", err
	}
	if err := tx.Commit(queryCtx); err != nil {
		return "", fmt.Errorf("commit save contact transaction: %w", err)
	}

	return contact.ID, nil
}

func (s *PostgresStorage) GetContactByPhone(ctx context.Context, phone string) (models.Contact, bool, error) {
	const getContactByPhoneQuery = `
		SELECT
			c.id,
			c.phone,
			c.email,
			c.name,
			c.discount,
			c.data,
			c.created_at,
			c.updated_at,
			COALESCE(source.file_id, ''),
			COALESCE(source.row_number, 0)
		FROM contacts AS c
		LEFT JOIN LATERAL (
			SELECT file_id, row_number
			FROM contact_sources
			WHERE contact_id = c.id
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) AS source ON TRUE
		WHERE c.phone = $1
	`

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	contact, err := scanContact(s.pool.QueryRow(queryCtx, getContactByPhoneQuery, strings.TrimSpace(phone)))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Contact{}, false, nil
	}
	if err != nil {
		return models.Contact{}, false, fmt.Errorf("get contact by phone: %w", err)
	}
	return contact, true, nil
}

func (s *PostgresStorage) ListContactsByFileID(ctx context.Context, fileID string) ([]models.Contact, error) {
	const listContactsByFileQuery = `
		SELECT DISTINCT ON (c.id)
			c.id,
			c.phone,
			c.email,
			c.name,
			c.discount,
			c.data,
			c.created_at,
			c.updated_at,
			COALESCE(source.file_id, ''),
			source.row_number
		FROM contact_sources AS source
		JOIN contacts AS c ON c.id = source.contact_id
		WHERE source.file_id = $1
		ORDER BY c.id, source.created_at DESC, source.id DESC
	`

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	rows, err := s.pool.Query(queryCtx, listContactsByFileQuery, fileID)
	if err != nil {
		return nil, fmt.Errorf("list contacts by file id: %w", err)
	}
	defer rows.Close()

	contacts := make([]models.Contact, 0)
	for rows.Next() {
		contact, err := scanContact(rows)
		if err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		contacts = append(contacts, contact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contacts: %w", err)
	}
	return contacts, nil
}

func (s *PostgresStorage) UpdateContact(ctx context.Context, contact models.Contact) error {
	const updateContactQuery = `
		UPDATE contacts
		SET phone = $1, email = $2, name = $3, discount = $4, data = $5
		WHERE id = $6
		RETURNING created_at, updated_at
	`

	dataJSON, err := marshalContactData(contact.Data)
	if err != nil {
		return err
	}

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	tx, err := s.pool.Begin(queryCtx)
	if err != nil {
		return fmt.Errorf("begin update contact transaction: %w", err)
	}
	defer tx.Rollback(queryCtx)

	err = tx.QueryRow(queryCtx, updateContactQuery,
		contact.Phone,
		contact.Email,
		contact.Name,
		contact.Discount,
		dataJSON,
		contact.ID,
	).Scan(
		&contact.CreatedAt,
		&contact.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrContactNotFound
	}
	if err != nil {
		return fmt.Errorf("update contact: %w", err)
	}

	if err := saveContactVersionTx(queryCtx, tx, contact, models.ContactEventUpdated); err != nil {
		return err
	}
	if err := tx.Commit(queryCtx); err != nil {
		return fmt.Errorf("commit update contact transaction: %w", err)
	}
	return nil
}

func (s *PostgresStorage) ResolveConflict(ctx context.Context, phone string, action models.ConflictAction, incoming models.Contact) error {
	const lockContactByPhoneQuery = `
		SELECT id, phone, email, name, discount, data, created_at, updated_at
		FROM contacts
		WHERE phone = $1
		FOR UPDATE
	`
	const conflictAlreadyAppliedQuery = `
		SELECT EXISTS (
			SELECT 1
			FROM contact_sources
			WHERE contact_id = $1
			  AND file_id = $2
			  AND row_number = $3
			  AND action = $4
		)
	`

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	tx, err := s.pool.Begin(queryCtx)
	if err != nil {
		return fmt.Errorf("begin resolve conflict transaction: %w", err)
	}
	defer tx.Rollback(queryCtx)

	existing, err := scanCoreContact(tx.QueryRow(queryCtx, lockContactByPhoneQuery, strings.TrimSpace(phone)))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrContactNotFound
	}
	if err != nil {
		return fmt.Errorf("lock conflicting contact: %w", err)
	}

	eventAction, err := eventActionForConflict(action)
	if err != nil {
		return err
	}
	if incoming.FileID != "" {
		var alreadyApplied bool
		if err := tx.QueryRow(queryCtx, conflictAlreadyAppliedQuery,
			existing.ID,
			incoming.FileID,
			incoming.SourceRow,
			eventAction,
		).Scan(&alreadyApplied); err != nil {
			return fmt.Errorf("check conflict idempotency: %w", err)
		}
		if alreadyApplied {
			return tx.Commit(queryCtx)
		}
	}

	incoming.ID = existing.ID
	incoming.CreatedAt = existing.CreatedAt

	switch action {
	case models.ConflictActionSkip:
		if err := saveContactSourceTx(queryCtx, tx, incoming, models.ContactEventSkipped); err != nil {
			return err
		}
	case models.ConflictActionReplace:
		if err := updateResolvedContact(queryCtx, tx, &incoming, models.ContactEventReplaced); err != nil {
			return err
		}
	case models.ConflictActionMerge:
		mergeContact(&incoming, existing)
		if err := updateResolvedContact(queryCtx, tx, &incoming, models.ContactEventMerged); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown conflict action: %s", action)
	}

	if err := tx.Commit(queryCtx); err != nil {
		return fmt.Errorf("commit resolve conflict transaction: %w", err)
	}
	return nil
}

func scanContact(row rowScanner) (models.Contact, error) {
	var contact models.Contact
	var dataJSON []byte
	if err := row.Scan(
		&contact.ID,
		&contact.Phone,
		&contact.Email,
		&contact.Name,
		&contact.Discount,
		&dataJSON,
		&contact.CreatedAt,
		&contact.UpdatedAt,
		&contact.FileID,
		&contact.SourceRow,
	); err != nil {
		return models.Contact{}, err
	}
	if err := unmarshalJSON(dataJSON, &contact.Data, "contact data"); err != nil {
		return models.Contact{}, err
	}
	return contact, nil
}

func scanCoreContact(row rowScanner) (models.Contact, error) {
	var contact models.Contact
	var dataJSON []byte
	if err := row.Scan(
		&contact.ID,
		&contact.Phone,
		&contact.Email,
		&contact.Name,
		&contact.Discount,
		&dataJSON,
		&contact.CreatedAt,
		&contact.UpdatedAt,
	); err != nil {
		return models.Contact{}, err
	}
	if err := unmarshalJSON(dataJSON, &contact.Data, "contact data"); err != nil {
		return models.Contact{}, err
	}
	return contact, nil
}

func updateResolvedContact(ctx context.Context, tx pgx.Tx, contact *models.Contact, action models.ContactEventAction) error {
	const updateResolvedContactQuery = `
		UPDATE contacts
		SET phone = $1, email = $2, name = $3, discount = $4, data = $5
		WHERE id = $6
		RETURNING updated_at
	`

	dataJSON, err := marshalContactData(contact.Data)
	if err != nil {
		return err
	}

	err = tx.QueryRow(ctx, updateResolvedContactQuery,
		contact.Phone,
		contact.Email,
		contact.Name,
		contact.Discount,
		dataJSON,
		contact.ID,
	).Scan(&contact.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrContactNotFound
	}
	if err != nil {
		return fmt.Errorf("apply contact conflict: %w", err)
	}

	if err := saveContactVersionTx(ctx, tx, *contact, action); err != nil {
		return err
	}
	if err := saveContactSourceTx(ctx, tx, *contact, action); err != nil {
		return err
	}
	return nil
}

func saveContactVersionTx(ctx context.Context, tx pgx.Tx, contact models.Contact, action models.ContactEventAction) error {
	const insertContactVersionQuery = `
		INSERT INTO contact_versions (
			contact_id, phone, email, name, discount, data, file_id, action
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8)
	`

	dataJSON, err := marshalContactData(contact.Data)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, insertContactVersionQuery,
		contact.ID,
		contact.Phone,
		contact.Email,
		contact.Name,
		contact.Discount,
		dataJSON,
		contact.FileID,
		action,
	); err != nil {
		return fmt.Errorf("save contact version: %w", err)
	}
	return nil
}

func saveContactSourceTx(ctx context.Context, tx pgx.Tx, contact models.Contact, action models.ContactEventAction) error {
	const insertContactSourceQuery = `
		INSERT INTO contact_sources (contact_id, file_id, row_number, action, incoming)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (contact_id, file_id, row_number, action) DO NOTHING
	`

	if strings.TrimSpace(contact.FileID) == "" {
		return nil
	}

	snapshotJSON, err := marshalContactSnapshot(contact)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, insertContactSourceQuery,
		contact.ID,
		contact.FileID,
		contact.SourceRow,
		action,
		snapshotJSON,
	); err != nil {
		return fmt.Errorf("save contact source: %w", err)
	}
	return nil
}

func marshalContactData(data map[string]string) ([]byte, error) {
	if data == nil {
		data = map[string]string{}
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal contact data: %w", err)
	}
	return encoded, nil
}

func marshalContactSnapshot(contact models.Contact) ([]byte, error) {
	snapshot := map[string]any{
		"phone":     contact.Phone,
		"email":     contact.Email,
		"name":      contact.Name,
		"discount":  contact.Discount,
		"data":      contact.Data,
		"fileId":    contact.FileID,
		"rowNumber": contact.SourceRow,
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal contact snapshot: %w", err)
	}
	return encoded, nil
}

func mergeContact(incoming *models.Contact, existing models.Contact) {
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
		incoming.Data = make(map[string]string, len(existing.Data))
	}
	for key, value := range existing.Data {
		if _, ok := incoming.Data[key]; !ok {
			incoming.Data[key] = value
		}
	}
}

func eventActionForConflict(action models.ConflictAction) (models.ContactEventAction, error) {
	switch action {
	case models.ConflictActionSkip:
		return models.ContactEventSkipped, nil
	case models.ConflictActionReplace:
		return models.ContactEventReplaced, nil
	case models.ConflictActionMerge:
		return models.ContactEventMerged, nil
	default:
		return "", fmt.Errorf("unknown conflict action: %s", action)
	}
}
