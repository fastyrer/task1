-- Сначала снимаем старое ограничение, иначе оно отклонит номера,
-- пока мы переводим их в плоский формат +7XXXXXXXXXX.
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS chk_contacts_phone;

UPDATE contacts
SET phone = '+7' || right(regexp_replace(phone, '\D', '', 'g'), 10)
WHERE phone !~ '^\+7[0-9]{10}$';

ALTER TABLE contacts ADD CONSTRAINT chk_contacts_phone CHECK (phone ~ '^\+7[0-9]{10}$');
