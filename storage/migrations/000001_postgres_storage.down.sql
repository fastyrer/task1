DROP TRIGGER IF EXISTS contacts_set_updated_at ON contacts;
DROP TRIGGER IF EXISTS uploaded_files_set_updated_at ON uploaded_files;
DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS contact_sources;
DROP TABLE IF EXISTS file_rows;

ALTER TABLE contact_versions DROP CONSTRAINT IF EXISTS chk_contact_versions_action;
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS chk_contacts_data;
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS chk_contacts_discount;
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS chk_contacts_phone;
ALTER TABLE uploaded_files DROP CONSTRAINT IF EXISTS chk_uploaded_files_size;
ALTER TABLE uploaded_files DROP CONSTRAINT IF EXISTS chk_uploaded_files_format;
