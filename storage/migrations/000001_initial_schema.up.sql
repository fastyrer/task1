-- Поиск подстроки по содержимому строк файла использует trigram-индекс.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- переделал логику решения конфликтов. Теперь мы не сохраняем ситорию изменений профиля 

-- Метаданные загрузки. Сами строки хранятся отдельно в file_rows.
CREATE TABLE uploaded_files (
	id TEXT PRIMARY KEY,
	original_filename TEXT NOT NULL DEFAULT '',
	size BIGINT NOT NULL DEFAULT 0,
	mime_type TEXT NOT NULL DEFAULT '',
	detected_mime_type TEXT NOT NULL DEFAULT '',
	format TEXT NOT NULL,
	encoding TEXT NOT NULL DEFAULT '',
	sheet_name TEXT NOT NULL DEFAULT '',
	sheets JSONB NOT NULL DEFAULT '[]'::jsonb,
	header_row INTEGER NOT NULL DEFAULT 0,
	headers JSONB NOT NULL DEFAULT '[]'::jsonb,
	row_count INTEGER NOT NULL DEFAULT 0,
	column_count INTEGER NOT NULL DEFAULT 0,
	stats JSONB NOT NULL DEFAULT '{}'::jsonb,
	warnings JSONB NOT NULL DEFAULT '[]'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT chk_uploaded_files_size CHECK (size >= 0),
	CONSTRAINT chk_uploaded_files_format CHECK (format IN ('csv', 'xls', 'xlsx')),
	CONSTRAINT chk_uploaded_files_header_row CHECK (header_row >= 0),
	CONSTRAINT chk_uploaded_files_row_count CHECK (row_count >= 0),
	CONSTRAINT chk_uploaded_files_column_count CHECK (column_count >= 0),
	CONSTRAINT chk_uploaded_files_sheets CHECK (jsonb_typeof(sheets) = 'array'),
	CONSTRAINT chk_uploaded_files_headers CHECK (jsonb_typeof(headers) = 'array'),
	CONSTRAINT chk_uploaded_files_stats CHECK (jsonb_typeof(stats) = 'object'),
	CONSTRAINT chk_uploaded_files_warnings CHECK (jsonb_typeof(warnings) = 'array')
);

-- Каждая строка файла хранится отдельно для выборки, исправления и поиска.
CREATE TABLE file_rows (
	id BIGSERIAL PRIMARY KEY,
	file_id TEXT NOT NULL REFERENCES uploaded_files(id) ON DELETE CASCADE,
	position INTEGER NOT NULL,
	row_number INTEGER NOT NULL,
	values JSONB NOT NULL,
	is_valid BOOLEAN NOT NULL DEFAULT TRUE,
	errors JSONB NOT NULL DEFAULT '[]'::jsonb,
	search_text TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT uq_file_rows_position UNIQUE (file_id, position),
	CONSTRAINT chk_file_rows_position CHECK (position > 0),
	CONSTRAINT chk_file_rows_row_number CHECK (row_number > 0),
	CONSTRAINT chk_file_rows_values CHECK (jsonb_typeof(values) = 'object'),
	CONSTRAINT chk_file_rows_errors CHECK (jsonb_typeof(errors) = 'array')
);

CREATE INDEX file_rows_file_id_row_number_idx
	ON file_rows (file_id, row_number);
CREATE INDEX file_rows_search_text_trgm_idx
	ON file_rows USING GIN (search_text gin_trgm_ops);

-- contacts хранит только текущее состояние контакта.
-- Внутренний SERIAL используется в связях, публичный UUID генерирует PostgreSQL.
CREATE TABLE contacts (
	id SERIAL PRIMARY KEY,
	uid UUID NOT NULL DEFAULT gen_random_uuid(),
	phone TEXT NOT NULL,
	email TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL DEFAULT '',
	discount TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT uq_contacts_uid UNIQUE (uid),
	CONSTRAINT uq_contacts_phone UNIQUE (phone),
	CONSTRAINT chk_contacts_phone
		CHECK (phone ~ '^\+7 \([0-9]{3}\) [0-9]{3}-[0-9]{2}-[0-9]{2}$'),
	CONSTRAINT chk_contacts_discount CHECK (
		discount = '' OR
		(discount ~ '^[0-9]+([.][0-9]+)?$' AND discount::numeric BETWEEN 0 AND 100)
	)
);

-- Одна актуальная связь на контакт и строку файла.
-- Повторное решение конфликта обновляет action, не создавая историю изменений.
CREATE TABLE contact_sources (
	id BIGSERIAL PRIMARY KEY,
	contact_id INTEGER NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
	file_id TEXT NOT NULL REFERENCES uploaded_files(id) ON DELETE CASCADE,
	row_number INTEGER NOT NULL,
	action TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT uq_contact_sources_row UNIQUE (contact_id, file_id, row_number),
	CONSTRAINT chk_contact_sources_row_number CHECK (row_number > 0),
	CONSTRAINT chk_contact_sources_action CHECK (
		action IN ('created', 'matched', 'skipped', 'replaced', 'merged', 'fixed')
	)
);

CREATE INDEX contact_sources_file_id_idx
	ON contact_sources (file_id, row_number);
CREATE INDEX contact_sources_contact_id_idx
	ON contact_sources (contact_id, updated_at DESC, id DESC);

-- Общая trigger-функция поддерживает updated_at рабочих сущностей.
CREATE FUNCTION set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $function$
BEGIN
	NEW.updated_at = now();
	RETURN NEW;
END
$function$;

CREATE TRIGGER uploaded_files_set_updated_at
	BEFORE UPDATE ON uploaded_files
	FOR EACH ROW
	EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER contacts_set_updated_at
	BEFORE UPDATE ON contacts
	FOR EACH ROW
	EXECUTE FUNCTION set_updated_at();
