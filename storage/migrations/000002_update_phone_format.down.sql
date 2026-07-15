-- Сначала снимаем новое ограничение, затем возвращаем номера к формату
-- +7 (XXX) XXX-XX-XX и восстанавливаем прежний CHECK.
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS chk_contacts_phone;

UPDATE contacts
SET phone = regexp_replace(
	right(regexp_replace(phone, '\D', '', 'g'), 10),
	'^([0-9]{3})([0-9]{3})([0-9]{2})([0-9]{2})$',
	'+7 (\1) \2-\3-\4'
)
WHERE phone !~ '^\+7 \([0-9]{3}\) [0-9]{3}-[0-9]{2}-[0-9]{2}$';

ALTER TABLE contacts ADD CONSTRAINT chk_contacts_phone CHECK (phone ~ '^\+7 \([0-9]{3}\) [0-9]{3}-[0-9]{2}-[0-9]{2}$');
