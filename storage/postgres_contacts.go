// postgres_contacts.go - CRUD контактов, дедупликация по телефону,
// разрешение конфликтов и связь контактов со строками файлов.
package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"task1/models"
)

// rowScanner - общий минимальный интерфейс для pgx.Row и pgx.Rows.
// Благодаря ему одна функция scanContact читает как одну, так и много строк.
type rowScanner interface {
	Scan(dest ...any) error
}

// SaveContact - создаёт новый контакт.
//
// Алгоритм:
//  1. PostgreSQL генерирует внутренний serial ID и публичный UID.
//  2. Вставляет фиксированные поля через ON CONFLICT (phone) DO NOTHING.
//  3. Если телефон уже есть, возвращает ErrContactAlreadyExists.
//  4. В той же транзакции связывает контакт со строкой файла.
func (s *PostgresStorage) SaveContact(ctx context.Context, contact models.Contact) (string, error) {
	const insertContactQuery = `
		INSERT INTO contacts (phone, email, name, discount)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (phone) DO NOTHING
		RETURNING id, uid::text, created_at, updated_at
	`

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	tx, err := s.pool.Begin(queryCtx)
	if err != nil {
		return "", fmt.Errorf("begin save contact transaction: %w", err)
	}
	defer tx.Rollback(queryCtx)

	err = tx.QueryRow(queryCtx, insertContactQuery,
		contact.Phone,
		contact.Email,
		contact.Name,
		contact.Discount,
	).Scan(
		&contact.ID,
		&contact.UID,
		&contact.CreatedAt,
		&contact.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrContactAlreadyExists
	}
	if err != nil {
		return "", fmt.Errorf("insert contact: %w", err)
	}

	if err := saveContactSourceTx(queryCtx, tx, contact, models.ContactSourceCreated); err != nil {
		return "", err
	}
	if err := tx.Commit(queryCtx); err != nil {
		return "", fmt.Errorf("commit save contact transaction: %w", err)
	}

	return contact.UID, nil
}

// ListContacts - возвращает текущее состояние всех контактов для общей рассылки.
// Уникальное ограничение contacts.phone гарантирует одну запись на номер телефона.
func (s *PostgresStorage) ListContacts(ctx context.Context) ([]models.Contact, error) {
	const listContactsQuery = `
		SELECT id, uid::text, phone, email, name, discount, created_at, updated_at
		FROM contacts
		ORDER BY id
	`

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	rows, err := s.pool.Query(queryCtx, listContactsQuery)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	contacts := make([]models.Contact, 0)
	for rows.Next() {
		contact, err := scanCoreContact(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed contact: %w", err)
		}
		contacts = append(contacts, contact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contacts: %w", err)
	}

	return contacts, nil
}

// GetContactByPhone - возвращает актуальное состояние контакта по уникальному телефону.
// LATERAL-подзапрос добавляет к ответу последний файл и номер строки-источника.
func (s *PostgresStorage) GetContactByPhone(ctx context.Context, phone string) (models.Contact, bool, error) {
	const getContactByPhoneQuery = `
		SELECT
			c.id,
			c.uid::text,
			c.phone,
			c.email,
			c.name,
			c.discount,
			c.created_at,
			c.updated_at,
			COALESCE(source.file_id, ''),
			COALESCE(source.row_number, 0)
		FROM contacts AS c
		LEFT JOIN LATERAL (
			SELECT file_id, row_number
			FROM contact_sources
			WHERE contact_id = c.id
			ORDER BY updated_at DESC, id DESC
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

// RecordContactMatch сохраняет связь с новым файлом, когда поля контакта совпали полностью.
func (s *PostgresStorage) RecordContactMatch(ctx context.Context, existing, incoming models.Contact) error {
	const upsertMatchedSourceQuery = `
		INSERT INTO contact_sources (contact_id, file_id, row_number, action)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (contact_id, file_id, row_number) DO UPDATE SET
			action = EXCLUDED.action,
			updated_at = now()
	`

	if existing.ID <= 0 {
		return ErrContactNotFound
	}
	if strings.TrimSpace(incoming.FileID) == "" {
		return nil
	}
	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()
	if _, err := s.pool.Exec(queryCtx, upsertMatchedSourceQuery,
		existing.ID,
		incoming.FileID,
		incoming.SourceRow,
		models.ContactSourceMatched,
	); err != nil {
		return fmt.Errorf("save matched contact source: %w", err)
	}
	return nil
}

// ResolveConflict - атомарно применяет skip, replace или merge к конфликту телефона.
//
// Алгоритм:
//  1. SELECT ... FOR UPDATE блокирует контакт до конца транзакции.
//  2. Проверка contact_sources делает повторный запрос идемпотентным.
//  3. skip записывает только решение, replace заменяет данные, merge дополняет пустые поля.
//  4. Решение и текущее состояние контакта изменяются в одной транзакции.
func (s *PostgresStorage) ResolveConflict(ctx context.Context, phone string, action models.ConflictAction, incoming models.Contact) error {
	const lockContactByPhoneQuery = `
		SELECT id, uid::text, phone, email, name, discount, created_at, updated_at
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

	sourceAction, err := sourceActionForConflict(action)
	if err != nil {
		return err
	}
	if incoming.FileID != "" {
		var alreadyApplied bool
		if err := tx.QueryRow(queryCtx, conflictAlreadyAppliedQuery,
			existing.ID,
			incoming.FileID,
			incoming.SourceRow,
			sourceAction,
		).Scan(&alreadyApplied); err != nil {
			return fmt.Errorf("check conflict idempotency: %w", err)
		}
		if alreadyApplied {
			return tx.Commit(queryCtx)
		}
	}

	incoming.ID = existing.ID
	incoming.UID = existing.UID
	incoming.CreatedAt = existing.CreatedAt

	switch action {
	case models.ConflictActionSkip:
		if err := saveContactSourceTx(queryCtx, tx, incoming, models.ContactSourceSkipped); err != nil {
			return err
		}
	case models.ConflictActionReplace:
		if err := updateResolvedContact(queryCtx, tx, &incoming, models.ContactSourceReplaced); err != nil {
			return err
		}
	case models.ConflictActionMerge:
		mergeContact(&incoming, existing)
		if err := updateResolvedContact(queryCtx, tx, &incoming, models.ContactSourceMerged); err != nil {
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

// scanContact - считывает контакт вместе с последним файлом-источником.
// Порядок Scan должен точно совпадать с SELECT в вызывающем методе.
func scanContact(row rowScanner) (models.Contact, error) {
	var contact models.Contact
	if err := row.Scan(
		&contact.ID,
		&contact.UID,
		&contact.Phone,
		&contact.Email,
		&contact.Name,
		&contact.Discount,
		&contact.CreatedAt,
		&contact.UpdatedAt,
		&contact.FileID,
		&contact.SourceRow,
	); err != nil {
		return models.Contact{}, err
	}
	return contact, nil
}

// scanCoreContact - считывает только поля contacts без данных об источнике.
func scanCoreContact(row rowScanner) (models.Contact, error) {
	var contact models.Contact
	if err := row.Scan(
		&contact.ID,
		&contact.UID,
		&contact.Phone,
		&contact.Email,
		&contact.Name,
		&contact.Discount,
		&contact.CreatedAt,
		&contact.UpdatedAt,
	); err != nil {
		return models.Contact{}, err
	}
	return contact, nil
}

// updateResolvedContact - обновляет контакт при replace/merge и сохраняет выбранное действие.
// Транзакция передаётся снаружи, поэтому helper не может случайно закоммитить часть операции.
func updateResolvedContact(ctx context.Context, tx pgx.Tx, contact *models.Contact, action models.ContactSourceAction) error {
	const updateResolvedContactQuery = `
		UPDATE contacts
		SET phone = $1, email = $2, name = $3, discount = $4
		WHERE id = $5
		RETURNING updated_at
	`

	err := tx.QueryRow(ctx, updateResolvedContactQuery,
		contact.Phone,
		contact.Email,
		contact.Name,
		contact.Discount,
		contact.ID,
	).Scan(&contact.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrContactNotFound
	}
	if err != nil {
		return fmt.Errorf("apply contact conflict: %w", err)
	}

	if err := saveContactSourceTx(ctx, tx, *contact, action); err != nil {
		return err
	}
	return nil
}

// saveContactSourceTx - сохраняет текущую связь контакта со строкой файла.
// Повторное решение для той же строки обновляет действие вместо накопления истории.
func saveContactSourceTx(ctx context.Context, tx pgx.Tx, contact models.Contact, action models.ContactSourceAction) error {
	const upsertContactSourceQuery = `
		INSERT INTO contact_sources (contact_id, file_id, row_number, action)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (contact_id, file_id, row_number) DO UPDATE SET
			action = EXCLUDED.action,
			updated_at = now()
	`

	if strings.TrimSpace(contact.FileID) == "" {
		return nil
	}

	if _, err := tx.Exec(ctx, upsertContactSourceQuery,
		contact.ID,
		contact.FileID,
		contact.SourceRow,
		action,
	); err != nil {
		return fmt.Errorf("save contact source: %w", err)
	}
	return nil
}

// mergeContact - дополняет пустые поля incoming значениями existing.
// Непустые входящие значения имеют приоритет.
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
}

// sourceActionForConflict - преобразует действие API в состояние связи со строкой файла.
func sourceActionForConflict(action models.ConflictAction) (models.ContactSourceAction, error) {
	switch action {
	case models.ConflictActionSkip:
		return models.ContactSourceSkipped, nil
	case models.ConflictActionReplace:
		return models.ContactSourceReplaced, nil
	case models.ConflictActionMerge:
		return models.ContactSourceMerged, nil
	default:
		return "", fmt.Errorf("unknown conflict action: %s", action)
	}
}
