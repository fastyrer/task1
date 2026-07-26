-- Производное поле поиска поддерживается триггером и индексируется pg_trgm.
-- Это заменяет полный пересчёт concat_ws/lower для каждой строки каждого запроса.
ALTER TABLE contacts
ADD COLUMN search_text TEXT NOT NULL DEFAULT '';

CREATE FUNCTION set_contact_search_text()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $function$
BEGIN
	NEW.search_text = lower(concat_ws(
		' ',
		NEW.uid::text,
		NEW.phone,
		NEW.email,
		NEW.name,
		NEW.discount
	));
	RETURN NEW;
END
$function$;

CREATE TRIGGER contacts_set_search_text
	BEFORE INSERT OR UPDATE ON contacts
	FOR EACH ROW
	EXECUTE FUNCTION set_contact_search_text();

UPDATE contacts
SET search_text = lower(concat_ws(' ', uid::text, phone, email, name, discount));

CREATE INDEX contacts_search_text_trgm_idx
	ON contacts USING GIN (search_text gin_trgm_ops);
