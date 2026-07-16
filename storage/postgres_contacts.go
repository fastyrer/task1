// postgres_contacts.go - поиск и конкурентно-безопасное изменение справочника.
package storage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"task1/models"
)

// rowScanner позволяет одной функции Scan работать с pgx.Row и pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// ListContactsPage возвращает запрошенный фрагмент справочника и общее количество
// совпадений. Поиск выполняется по всем видимым полям одной строки контакта.
func (s *PostgresStorage) ListContactsPage(ctx context.Context, query string, limit, offset int) ([]models.Contact, int64, error) {
	const countContactsQuery = `
		SELECT count(*)
		FROM contacts
		WHERE $1 = '' OR strpos(
			lower(concat_ws(' ', uid::text, phone, email, name, discount)),
			lower($1)
		) > 0
	`
	const listContactPageQuery = `
		SELECT id, uid::text, phone, email, name, discount, created_at, updated_at
		FROM contacts
		WHERE $1 = '' OR strpos(
			lower(concat_ws(' ', uid::text, phone, email, name, discount)),
			lower($1)
		) > 0
		ORDER BY id
		LIMIT $2 OFFSET $3
	`

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	var total int64
	if err := s.pool.QueryRow(queryCtx, countContactsQuery, query).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count contacts: %w", err)
	}

	rows, err := s.pool.Query(queryCtx, listContactPageQuery, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list contacts page: %w", err)
	}
	defer rows.Close()

	contacts := make([]models.Contact, 0, limit)
	for rows.Next() {
		contact, err := scanCoreContact(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan contact page: %w", err)
		}
		contacts = append(contacts, contact)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate contact page: %w", err)
	}
	return contacts, total, nil
}

// UpdateContact сохраняет ручные изменения и блокирует старый и новый телефоны
// в том же порядке, что операции импорта. SQL операции виден целиком в методе.
func (s *PostgresStorage) UpdateContact(
	ctx context.Context,
	uid string,
	expectedUpdatedAt time.Time,
	contact models.Contact,
) (models.Contact, error) {
	const selectCurrentPhoneQuery = `SELECT phone FROM contacts WHERE uid = $1::uuid`
	const lockPhoneQuery = `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`
	const getContactForUpdateQuery = `
		SELECT id, uid::text, phone, email, name, discount, created_at, updated_at
		FROM contacts
		WHERE uid = $1::uuid
		FOR UPDATE
	`
	const updateContactQuery = `
		UPDATE contacts
		SET phone = $1, email = $2, name = $3, discount = $4
		WHERE id = $5
		RETURNING id, uid::text, phone, email, name, discount, created_at, updated_at
	`

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()
	tx, err := s.pool.BeginTx(queryCtx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return models.Contact{}, fmt.Errorf("begin contact update: %w", err)
	}
	defer tx.Rollback(queryCtx)

	var currentPhone string
	if err := tx.QueryRow(queryCtx, selectCurrentPhoneQuery, uid).Scan(&currentPhone); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Contact{}, ErrContactChanged
		}
		return models.Contact{}, fmt.Errorf("read contact phone for update: %w", err)
	}

	phones := []string{currentPhone, contact.Phone}
	sort.Strings(phones)
	for index, phone := range phones {
		if index > 0 && phone == phones[index-1] {
			continue
		}
		if _, err := tx.Exec(queryCtx, lockPhoneQuery, phone); err != nil {
			return models.Contact{}, fmt.Errorf("lock edited contact phone: %w", err)
		}
	}

	existing, err := scanCoreContact(tx.QueryRow(queryCtx, getContactForUpdateQuery, uid))
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !existing.UpdatedAt.Equal(expectedUpdatedAt) {
		return models.Contact{}, ErrContactChanged
	}
	if err != nil {
		return models.Contact{}, fmt.Errorf("lock contact for update: %w", err)
	}
	if existing.Phone != currentPhone {
		return models.Contact{}, ErrContactChanged
	}

	updated, err := scanCoreContact(tx.QueryRow(
		queryCtx,
		updateContactQuery,
		contact.Phone,
		contact.Email,
		contact.Name,
		contact.Discount,
		existing.ID,
	))
	if err != nil {
		if mapped := contactUpdateError(err); mapped != nil {
			return models.Contact{}, mapped
		}
		return models.Contact{}, fmt.Errorf("update contact: %w", err)
	}

	if err := tx.Commit(queryCtx); err != nil {
		if mapped := contactUpdateError(err); mapped != nil {
			return models.Contact{}, mapped
		}
		return models.Contact{}, fmt.Errorf("commit contact update: %w", err)
	}
	return updated, nil
}

func contactUpdateError(err error) error {
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) {
		return nil
	}
	if pgError.Code == "23505" && pgError.ConstraintName == "uq_contacts_phone" {
		return ErrContactPhoneExists
	}
	if pgError.Code == "40001" {
		return ErrContactChanged
	}
	return nil
}

// ListContacts возвращает все подтверждённые контакты для общей рассылки.
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

// FindContactsByPhones читает только записи, участвующие в предпросмотре импорта.
// Здесь нет FOR UPDATE: preview не должен блокировать или изменять рабочий справочник.
func (s *PostgresStorage) FindContactsByPhones(ctx context.Context, phones []string) (map[string]models.Contact, error) {
	const findContactsByPhonesQuery = `
		SELECT id, uid::text, phone, email, name, discount, created_at, updated_at
		FROM contacts
		WHERE phone = ANY($1::text[])
	`

	result := make(map[string]models.Contact)
	if len(phones) == 0 {
		return result, nil
	}

	queryCtx, cancel := s.withImportTimeout(ctx)
	defer cancel()
	rows, err := s.pool.Query(queryCtx, findContactsByPhonesQuery, phones)
	if err != nil {
		return nil, fmt.Errorf("find contacts by phones: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		contact, err := scanCoreContact(rows)
		if err != nil {
			return nil, fmt.Errorf("scan import preview contact: %w", err)
		}
		result[contact.Phone] = contact
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate import preview contacts: %w", err)
	}
	return result, nil
}

// scanCoreContact читает фиксированные поля contacts в порядке SELECT выше.
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
