ALTER TABLE contacts DROP CONSTRAINT IF EXISTS chk_contacts_phone;
ALTER TABLE contacts ADD CONSTRAINT chk_contacts_phone CHECK (phone ~ '^\+7 \([0-9]{3}\) [0-9]{3}-[0-9]{2}-[0-9]{2}$');
