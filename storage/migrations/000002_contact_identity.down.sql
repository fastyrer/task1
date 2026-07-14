-- Откатываем схему идентификаторов, сохраняя фиксированную модель контакта без data.
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS chk_contacts_phone;
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS chk_contacts_discount;
ALTER TABLE contact_versions DROP CONSTRAINT IF EXISTS contact_versions_file_id_fkey;
ALTER TABLE contact_versions DROP CONSTRAINT IF EXISTS chk_contact_versions_action;

-- Готовим текстовые ключи на основе публичного uid.
ALTER TABLE contacts ADD COLUMN legacy_id TEXT;
UPDATE contacts SET legacy_id = uid::text;
ALTER TABLE contacts ALTER COLUMN legacy_id SET NOT NULL;

ALTER TABLE contact_sources ADD COLUMN legacy_contact_id TEXT;
UPDATE contact_sources AS source
SET legacy_contact_id = contact.uid::text
FROM contacts AS contact
WHERE source.contact_id = contact.id;
ALTER TABLE contact_sources ALTER COLUMN legacy_contact_id SET NOT NULL;

ALTER TABLE contact_versions ADD COLUMN legacy_contact_id TEXT;
UPDATE contact_versions AS version
SET legacy_contact_id = contact.uid::text
FROM contacts AS contact
WHERE version.contact_id = contact.id;
ALTER TABLE contact_versions ALTER COLUMN legacy_contact_id SET NOT NULL;

ALTER TABLE contact_sources DROP CONSTRAINT IF EXISTS contact_sources_contact_id_fkey;
ALTER TABLE contact_sources DROP CONSTRAINT IF EXISTS uq_contact_sources_event;
ALTER TABLE contact_versions DROP CONSTRAINT IF EXISTS contact_versions_contact_id_fkey;
DROP INDEX IF EXISTS contact_sources_contact_id_idx;
DROP INDEX IF EXISTS contact_versions_contact_id_idx;

ALTER TABLE contact_sources DROP COLUMN contact_id;
ALTER TABLE contact_sources RENAME COLUMN legacy_contact_id TO contact_id;
ALTER TABLE contact_versions DROP COLUMN contact_id;
ALTER TABLE contact_versions RENAME COLUMN legacy_contact_id TO contact_id;

ALTER TABLE contacts DROP CONSTRAINT IF EXISTS contacts_pkey;
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS uq_contacts_uid;
ALTER TABLE contacts DROP COLUMN id;
ALTER TABLE contacts RENAME COLUMN legacy_id TO id;
ALTER TABLE contacts ADD CONSTRAINT contacts_pkey PRIMARY KEY (id);
ALTER TABLE contacts DROP COLUMN uid;

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
