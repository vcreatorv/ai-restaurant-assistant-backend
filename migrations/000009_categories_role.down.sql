DROP INDEX IF EXISTS idx_categories_role;

ALTER TABLE categories DROP COLUMN IF EXISTS role;
