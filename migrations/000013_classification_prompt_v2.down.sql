-- Откат classification prompt v2 — удаляем версию 2, активной становится v1
-- (она остаётся в таблице нетронутой, т.к. up-миграция только INSERT'ит).

DELETE FROM prompts WHERE name = 'classification' AND version = 2;
