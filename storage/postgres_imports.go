// postgres_imports.go - единственная транзакция записи подтверждённого импорта.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"task1/models"
)

// CommitImport атомарно сохраняет проверенный файл и применяет контакты.
// Все SQL-запросы операции объявлены здесь, чтобы транзакцию можно было прочитать сверху вниз.
func (s *PostgresStorage) CommitImport(
	ctx context.Context,
	data models.FileData,
	contacts []models.Contact,
	decisions map[string]models.ImportDecision,
) (models.ImportCommitResult, error) {
	const insertFileQuery = `
		INSERT INTO uploaded_files (
			id, original_filename, size, mime_type, detected_mime_type,
			format, encoding, sheet_name, sheets, header_row, headers,
			row_count, column_count, stats, warnings
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (id) DO NOTHING
		RETURNING id
	`
	const lockPhoneQuery = `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`
	const getContactForUpdateQuery = `
		SELECT id, uid::text, phone, email, name, discount, created_at, updated_at
		FROM contacts
		WHERE phone = $1
		FOR UPDATE
	`
	const insertContactQuery = `
		INSERT INTO contacts (phone, email, name, discount)
		VALUES ($1, $2, $3, $4)
		RETURNING id, uid::text, created_at, updated_at
	`
	const updateContactQuery = `
		UPDATE contacts
		SET email = $1, name = $2, discount = $3
		WHERE id = $4
		RETURNING updated_at
	`
	const upsertContactSourceQuery = `
		INSERT INTO contact_sources (contact_id, file_id, row_number, action)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (contact_id, file_id, row_number) DO UPDATE SET
			action = EXCLUDED.action,
			updated_at = now()
	`

	data.ID = strings.TrimSpace(data.ID)
	if data.ID == "" {
		return models.ImportCommitResult{}, ErrImportChanged
	}

	sheetsJSON, err := marshalSlice(data.Sheets)
	if err != nil {
		return models.ImportCommitResult{}, fmt.Errorf("marshal import sheets: %w", err)
	}
	headersJSON, err := marshalSlice(data.Headers)
	if err != nil {
		return models.ImportCommitResult{}, fmt.Errorf("marshal import headers: %w", err)
	}
	statsJSON, err := json.Marshal(data.Stats)
	if err != nil {
		return models.ImportCommitResult{}, fmt.Errorf("marshal import stats: %w", err)
	}
	warningsJSON, err := marshalSlice(data.Warnings)
	if err != nil {
		return models.ImportCommitResult{}, fmt.Errorf("marshal import warnings: %w", err)
	}
	copyRows, err := prepareFileRows(data, data.ID)
	if err != nil {
		return models.ImportCommitResult{}, err
	}

	// Одинаковый порядок блокировок телефонов предотвращает взаимные блокировки импортов.
	orderedContacts := append([]models.Contact(nil), contacts...)
	sort.Slice(orderedContacts, func(i, j int) bool {
		return orderedContacts[i].Phone < orderedContacts[j].Phone
	})

	queryCtx, cancel := s.withImportTimeout(ctx)
	defer cancel()
	tx, err := s.pool.BeginTx(queryCtx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return models.ImportCommitResult{}, fmt.Errorf("begin import transaction: %w", err)
	}
	defer tx.Rollback(queryCtx)

	var storedImportID string
	err = tx.QueryRow(
		queryCtx,
		insertFileQuery,
		data.ID,
		data.OriginalFilename,
		data.Size,
		data.MIMEType,
		data.DetectedMIMEType,
		data.Format,
		data.Encoding,
		data.SheetName,
		sheetsJSON,
		data.HeaderRow,
		headersJSON,
		data.Stats.RowCount,
		data.Stats.ColumnCount,
		statsJSON,
		warningsJSON,
	).Scan(&storedImportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ImportCommitResult{}, ErrImportAlreadyCommitted
	}
	if err != nil {
		return models.ImportCommitResult{}, fmt.Errorf("insert confirmed file: %w", err)
	}

	if len(copyRows) > 0 {
		_, err = tx.CopyFrom(
			queryCtx,
			pgx.Identifier{"file_rows"},
			[]string{"file_id", "position", "row_number", "values", "is_valid", "errors", "search_text"},
			pgx.CopyFromRows(copyRows),
		)
		if err != nil {
			return models.ImportCommitResult{}, fmt.Errorf("copy confirmed file rows: %w", err)
		}
	}

	result := models.ImportCommitResult{ImportID: storedImportID}
	saveSource := func(contactID int64, rowNumber int, action models.ContactSourceAction) error {
		_, err := tx.Exec(queryCtx, upsertContactSourceQuery, contactID, data.ID, rowNumber, action)
		if err != nil {
			return fmt.Errorf("save import contact source: %w", err)
		}
		return nil
	}

	for _, incoming := range orderedContacts {
		if _, err := tx.Exec(queryCtx, lockPhoneQuery, incoming.Phone); err != nil {
			return models.ImportCommitResult{}, fmt.Errorf("lock import phone: %w", err)
		}

		existing, err := scanCoreContact(tx.QueryRow(queryCtx, getContactForUpdateQuery, incoming.Phone))
		if errors.Is(err, pgx.ErrNoRows) {
			// Если preview видел конфликт, исчезновение телефона означает изменение справочника.
			if _, wasConflict := decisions[incoming.Phone]; wasConflict {
				return models.ImportCommitResult{}, ErrImportChanged
			}
			incoming.FileID = data.ID
			err = tx.QueryRow(
				queryCtx,
				insertContactQuery,
				incoming.Phone,
				incoming.Email,
				incoming.Name,
				incoming.Discount,
			).Scan(&incoming.ID, &incoming.UID, &incoming.CreatedAt, &incoming.UpdatedAt)
			if err != nil {
				if importDataChanged(err) {
					return models.ImportCommitResult{}, ErrImportChanged
				}
				return models.ImportCommitResult{}, fmt.Errorf("insert import contact: %w", err)
			}
			if err := saveSource(incoming.ID, incoming.SourceRow, models.ContactSourceCreated); err != nil {
				return models.ImportCommitResult{}, err
			}
			result.Created++
			continue
		}
		if err != nil {
			return models.ImportCommitResult{}, fmt.Errorf("lock existing import contact: %w", err)
		}

		if contactCoreEqual(existing, incoming) {
			if err := saveSource(existing.ID, incoming.SourceRow, models.ContactSourceMatched); err != nil {
				return models.ImportCommitResult{}, err
			}
			result.Matched++
			continue
		}

		decision, exists := decisions[incoming.Phone]
		currentVersion := existing.UpdatedAt.UTC().Format(time.RFC3339Nano)
		if !exists || decision.Version != currentVersion {
			return models.ImportCommitResult{}, ErrImportChanged
		}

		switch decision.Action {
		case models.ConflictActionSkip:
			if err := saveSource(existing.ID, incoming.SourceRow, models.ContactSourceSkipped); err != nil {
				return models.ImportCommitResult{}, err
			}
			result.Skipped++
		case models.ConflictActionReplace, models.ConflictActionMerge:
			if decision.Action == models.ConflictActionMerge {
				mergeContact(&incoming, existing)
			}
			if err := tx.QueryRow(
				queryCtx,
				updateContactQuery,
				incoming.Email,
				incoming.Name,
				incoming.Discount,
				existing.ID,
			).Scan(&incoming.UpdatedAt); err != nil {
				return models.ImportCommitResult{}, fmt.Errorf("update import contact: %w", err)
			}
			action := models.ContactSourceReplaced
			if decision.Action == models.ConflictActionMerge {
				action = models.ContactSourceMerged
				result.Merged++
			} else {
				result.Replaced++
			}
			if err := saveSource(existing.ID, incoming.SourceRow, action); err != nil {
				return models.ImportCommitResult{}, err
			}
		default:
			return models.ImportCommitResult{}, ErrImportChanged
		}
	}

	if err := tx.Commit(queryCtx); err != nil {
		if importDataChanged(err) {
			return models.ImportCommitResult{}, ErrImportChanged
		}
		return models.ImportCommitResult{}, fmt.Errorf("commit confirmed import: %w", err)
	}
	return result, nil
}

func importDataChanged(err error) bool {
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) {
		return false
	}
	return pgError.Code == "23505" || pgError.Code == "40001"
}

func contactCoreEqual(a, b models.Contact) bool {
	return a.Phone == b.Phone &&
		a.Email == b.Email &&
		a.Name == b.Name &&
		a.Discount == b.Discount
}

// mergeContact сохраняет непустые значения файла и дополняет их текущими значениями БД.
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
