ALTER TABLE users DROP CONSTRAINT IF EXISTS users_dietary_whitelist;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_allergens_whitelist;
-- Обратное преобразование значений не делаем: фронт всё равно должен жить
-- на канонических кодах, восстанавливать русские строки из кодов нет смысла.
