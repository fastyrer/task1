-- Содержимое удалённого payload восстановить невозможно; откат возвращает только nullable-колонку.
ALTER TABLE uploaded_files ADD COLUMN IF NOT EXISTS payload JSONB;
