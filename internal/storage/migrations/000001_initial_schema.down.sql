-- Объекты удаляются в порядке, обратном зависимостям начальной схемы.
DROP TRIGGER IF EXISTS contacts_set_updated_at ON contacts;
DROP TRIGGER IF EXISTS uploaded_files_set_updated_at ON uploaded_files;
DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS contact_sources;
DROP TABLE IF EXISTS contacts;
DROP TABLE IF EXISTS file_rows;
DROP TABLE IF EXISTS uploaded_files;

-- pg_trgm не удаляется: расширение может использоваться другими схемами этой БД.
