-- Контакт получает два идентификатора:
-- id - внутренний SERIAL для быстрых JOIN и внешних ключей;
-- uid - публичный UUID, который PostgreSQL генерирует автоматически.
-- Legacy-ограничения временно снимаются: NOT VALID проверяет каждую обновляемую
-- строку, даже если миграция меняет только её идентификатор.
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS chk_contacts_phone;
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS chk_contacts_discount;
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS chk_contacts_data;
ALTER TABLE contact_versions DROP CONSTRAINT IF EXISTS contact_versions_file_id_fkey;
ALTER TABLE contact_versions DROP CONSTRAINT IF EXISTS chk_contact_versions_action;

ALTER TABLE contacts ADD COLUMN new_id SERIAL;
ALTER TABLE contacts ADD COLUMN uid UUID;

-- Старые текстовые UUID сохраняются как uid, поэтому внешние ссылки не меняются.
-- Для некорректного legacy-id генерируется новый UUID.
UPDATE contacts
SET uid = CASE
	WHEN id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
		THEN id::uuid
	ELSE gen_random_uuid()
END;

ALTER TABLE contacts ALTER COLUMN uid SET DEFAULT gen_random_uuid();
ALTER TABLE contacts ALTER COLUMN uid SET NOT NULL;

-- Сначала переносим ссылки дочерних таблиц со старого TEXT id на новый SERIAL id.
ALTER TABLE contact_sources ADD COLUMN new_contact_id INTEGER;
UPDATE contact_sources AS source
SET new_contact_id = contact.new_id,
	incoming = (source.incoming - 'data') || jsonb_build_object('uid', contact.uid::text)
FROM contacts AS contact
WHERE source.contact_id = contact.id;

ALTER TABLE contact_versions ADD COLUMN new_contact_id INTEGER;
UPDATE contact_versions AS version
SET new_contact_id = contact.new_id
FROM contacts AS contact
WHERE version.contact_id = contact.id;

-- Миграция останавливается, если в истории обнаружена потерянная ссылка на контакт.
DO $migration$
BEGIN
	IF EXISTS (SELECT 1 FROM contact_sources WHERE new_contact_id IS NULL) THEN
		RAISE EXCEPTION 'contact_sources contains orphaned contact_id';
	END IF;
	IF EXISTS (SELECT 1 FROM contact_versions WHERE new_contact_id IS NULL) THEN
		RAISE EXCEPTION 'contact_versions contains orphaned contact_id';
	END IF;
END
$migration$;

ALTER TABLE contact_sources ALTER COLUMN new_contact_id SET NOT NULL;
ALTER TABLE contact_versions ALTER COLUMN new_contact_id SET NOT NULL;

-- Удаляем ограничения и индексы, которые завязаны на старые текстовые ключи.
ALTER TABLE contact_sources DROP CONSTRAINT IF EXISTS contact_sources_contact_id_fkey;
ALTER TABLE contact_sources DROP CONSTRAINT IF EXISTS uq_contact_sources_event;
ALTER TABLE contact_versions DROP CONSTRAINT IF EXISTS contact_versions_contact_id_fkey;
DROP INDEX IF EXISTS contact_sources_contact_id_idx;
DROP INDEX IF EXISTS contact_versions_contact_id_idx;

ALTER TABLE contact_sources DROP COLUMN contact_id;
ALTER TABLE contact_sources RENAME COLUMN new_contact_id TO contact_id;
ALTER TABLE contact_versions DROP COLUMN contact_id;
ALTER TABLE contact_versions RENAME COLUMN new_contact_id TO contact_id;

-- Старый contacts.id становится uid, а новый SERIAL - внутренним первичным ключом.
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS contacts_pkey;
ALTER TABLE contacts DROP COLUMN id;
ALTER TABLE contacts RENAME COLUMN new_id TO id;
ALTER SEQUENCE contacts_new_id_seq RENAME TO contacts_id_seq;
ALTER SEQUENCE contacts_id_seq OWNED BY contacts.id;
ALTER TABLE contacts ADD CONSTRAINT contacts_pkey PRIMARY KEY (id);
ALTER TABLE contacts ADD CONSTRAINT uq_contacts_uid UNIQUE (uid);

-- Восстанавливаем связи и индексы уже для числового contact_id.
ALTER TABLE contact_sources
	ADD CONSTRAINT contact_sources_contact_id_fkey
	FOREIGN KEY (contact_id) REFERENCES contacts(id) ON DELETE RESTRICT;
ALTER TABLE contact_sources
	ADD CONSTRAINT uq_contact_sources_event
	UNIQUE (contact_id, file_id, row_number, action);
ALTER TABLE contact_versions
	ADD CONSTRAINT contact_versions_contact_id_fkey
	FOREIGN KEY (contact_id) REFERENCES contacts(id) ON DELETE RESTRICT;

CREATE INDEX contact_sources_contact_id_idx
	ON contact_sources (contact_id, created_at DESC);
CREATE INDEX contact_versions_contact_id_idx
	ON contact_versions (contact_id, created_at DESC);

-- Произвольные данные больше не являются частью контакта или его версии.
ALTER TABLE contacts DROP COLUMN IF EXISTS data;
ALTER TABLE contact_versions DROP COLUMN IF EXISTS data;

-- Прямая legacy-связь с файлом уже перенесена в contact_sources миграцией 000001.
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS contacts_file_id_fkey;
ALTER TABLE contacts DROP COLUMN IF EXISTS file_id;

-- Удаляем legacy-индексы и создаём индекс email в ожидаемом виде.
DROP INDEX IF EXISTS contacts_phone_idx;
DROP INDEX IF EXISTS contacts_file_id_idx;
DROP INDEX IF EXISTS contacts_email_idx;
CREATE INDEX contacts_email_idx ON contacts (lower(email));

-- Ограничения возвращаются как NOT VALID: новые INSERT/UPDATE проверяются сразу,
-- а некорректные legacy-строки можно исправить отдельным контролируемым процессом.
ALTER TABLE contacts
	ADD CONSTRAINT chk_contacts_phone
	CHECK (phone ~ '^\+7 \([0-9]{3}\) [0-9]{3}-[0-9]{2}-[0-9]{2}$') NOT VALID;
ALTER TABLE contacts
	ADD CONSTRAINT chk_contacts_discount
	CHECK (
		discount = '' OR
		(discount ~ '^[0-9]+([.][0-9]+)?$' AND discount::numeric BETWEEN 0 AND 100)
	) NOT VALID;
ALTER TABLE contact_versions
	ADD CONSTRAINT contact_versions_file_id_fkey
	FOREIGN KEY (file_id) REFERENCES uploaded_files(id) ON DELETE SET NULL NOT VALID;
ALTER TABLE contact_versions
	ADD CONSTRAINT chk_contact_versions_action
	CHECK (action IN ('created', 'updated', 'replaced', 'merged', 'fixed')) NOT VALID;
