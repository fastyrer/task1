-- Старый payload дублирует метаданные uploaded_files и строки из file_rows.
-- Перед удалением проверяем, что все строки JSON уже перенесены в нормализованную таблицу.
DO $migration$
BEGIN
	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'uploaded_files'
		  AND column_name = 'payload'
	) THEN
		IF EXISTS (
			SELECT 1
			FROM uploaded_files AS file
			WHERE CASE
				WHEN jsonb_typeof(file.payload->'rows') = 'array'
					THEN jsonb_array_length(file.payload->'rows')
				ELSE 0
			END > (
				SELECT count(*)
				FROM file_rows AS row
				WHERE row.file_id = file.id
			)
		) THEN
			RAISE EXCEPTION 'legacy uploaded_files.payload contains rows that are absent from file_rows';
		END IF;

		ALTER TABLE uploaded_files DROP COLUMN payload;
	END IF;
END
$migration$;
