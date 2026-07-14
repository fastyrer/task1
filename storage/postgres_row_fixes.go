// postgres_row_fixes.go - атомарное исправление строки файла и связанного контакта.
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

// SaveFixedRow сохраняет исправленную строку, контакт и аудит одной транзакцией.
func (s *PostgresStorage) SaveFixedRow(
	ctx context.Context,
	fileID string,
	rowNumber int,
	values map[string]string,
	contact models.Contact,
) error {
	const updateFileRowQuery = `
		UPDATE file_rows
		SET values = $3,
			is_valid = TRUE,
			errors = '[]'::jsonb,
			search_text = $4
		WHERE file_id = $1
		  AND row_number = $2
		  AND is_valid = FALSE
	`
	const insertFixedContactQuery = `
		INSERT INTO contacts (phone, email, name, discount)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (phone) DO NOTHING
		RETURNING id, uid::text, created_at, updated_at
	`
	const lockContactByPhoneQuery = `
		SELECT id, uid::text, phone, email, name, discount, created_at, updated_at
		FROM contacts
		WHERE phone = $1
		FOR UPDATE
	`
	const updateFixedContactQuery = `
		UPDATE contacts
		SET email = $1, name = $2, discount = $3
		WHERE id = $4
		RETURNING updated_at
	`
	const removeFixedRowWarningsQuery = `
		UPDATE uploaded_files AS file
		SET warnings = COALESCE(
			(
				SELECT jsonb_agg(item.warning ORDER BY item.position)
				FROM jsonb_array_elements(file.warnings)
					WITH ORDINALITY AS item(warning, position)
				WHERE CASE
					WHEN COALESCE(item.warning->>'row', '') ~ '^[0-9]+$'
						THEN (item.warning->>'row')::integer <> $2
					ELSE TRUE
				END
			),
			'[]'::jsonb
		)
		WHERE file.id = $1
	`
	const refreshFileStatsQuery = `
		UPDATE uploaded_files AS file
		SET stats = file.stats || jsonb_build_object(
			'validRowCount', counts.valid_rows,
			'invalidRowCount', counts.invalid_rows,
			'warningCount', jsonb_array_length(file.warnings)
		)
		FROM (
			SELECT
				count(*) FILTER (WHERE is_valid) AS valid_rows,
				count(*) FILTER (WHERE NOT is_valid) AS invalid_rows
			FROM file_rows
			WHERE file_id = $1
		) AS counts
		WHERE file.id = $1
	`

	fileID = strings.TrimSpace(fileID)
	if fileID == "" || rowNumber <= 0 {
		return ErrFileRowNotFixable
	}
	if values == nil {
		values = map[string]string{}
	}

	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("marshal fixed row: %w", err)
	}

	queryCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	tx, err := s.pool.Begin(queryCtx)
	if err != nil {
		return fmt.Errorf("begin fix row transaction: %w", err)
	}
	defer tx.Rollback(queryCtx)

	commandTag, err := tx.Exec(
		queryCtx,
		updateFileRowQuery,
		fileID,
		rowNumber,
		valuesJSON,
		rowSearchText(values),
	)
	if err != nil {
		return fmt.Errorf("update fixed file row: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return ErrFileRowNotFixable
	}

	contact.FileID = fileID
	contact.SourceRow = rowNumber
	created := false
	err = tx.QueryRow(
		queryCtx,
		insertFixedContactQuery,
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
	switch {
	case err == nil:
		created = true
	case errors.Is(err, pgx.ErrNoRows):
		existing, lockErr := scanCoreContact(tx.QueryRow(queryCtx, lockContactByPhoneQuery, contact.Phone))
		if lockErr != nil {
			return fmt.Errorf("lock fixed contact: %w", lockErr)
		}
		contact.ID = existing.ID
		contact.UID = existing.UID
		contact.CreatedAt = existing.CreatedAt
		contact.UpdatedAt = existing.UpdatedAt
		if !contactCoreEqual(existing, contact) {
			if err := tx.QueryRow(
				queryCtx,
				updateFixedContactQuery,
				contact.Email,
				contact.Name,
				contact.Discount,
				contact.ID,
			).Scan(&contact.UpdatedAt); err != nil {
				return fmt.Errorf("update fixed contact: %w", err)
			}
			if err := saveContactVersionTx(queryCtx, tx, contact, models.ContactEventFixed); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("insert fixed contact: %w", err)
	}

	if created {
		if err := saveContactVersionTx(queryCtx, tx, contact, models.ContactEventFixed); err != nil {
			return err
		}
	}
	if err := saveContactSourceTx(queryCtx, tx, contact, models.ContactEventFixed); err != nil {
		return err
	}
	if _, err := tx.Exec(queryCtx, removeFixedRowWarningsQuery, fileID, rowNumber); err != nil {
		return fmt.Errorf("remove fixed row warnings: %w", err)
	}
	if _, err := tx.Exec(queryCtx, refreshFileStatsQuery, fileID); err != nil {
		return fmt.Errorf("refresh file stats: %w", err)
	}
	if err := tx.Commit(queryCtx); err != nil {
		return fmt.Errorf("commit fix row transaction: %w", err)
	}
	return nil
}

func contactCoreEqual(a, b models.Contact) bool {
	return a.Phone == b.Phone &&
		a.Email == b.Email &&
		a.Name == b.Name &&
		a.Discount == b.Discount
}
