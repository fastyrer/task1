DROP INDEX IF EXISTS contacts_search_text_trgm_idx;
DROP TRIGGER IF EXISTS contacts_set_search_text ON contacts;
DROP FUNCTION IF EXISTS set_contact_search_text();
ALTER TABLE contacts DROP COLUMN IF EXISTS search_text;
