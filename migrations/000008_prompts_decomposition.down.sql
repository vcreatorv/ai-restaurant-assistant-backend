DELETE FROM prompts        WHERE name IN ('classification', 'refusal');
DELETE FROM prompt_drafts  WHERE prompt_name IN ('classification', 'refusal');
UPDATE prompts             SET name = 'system_main' WHERE name = 'system';
UPDATE prompt_drafts       SET prompt_name = 'system_main' WHERE prompt_name = 'system';
