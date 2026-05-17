-- Декомпозиция промптов: system_main → system + добавление classification и refusal.
--
-- Pipeline чата теперь:
--   user_msg → classifier (LLM, classification prompt) → intent
--     recommend/clarify → RAG + system
--     chitchat          → без RAG, system
--     off_topic         → без RAG, refusal
--
-- Существующие черновики мигрируют автоматически.

UPDATE prompts        SET name = 'system' WHERE name = 'system_main';
UPDATE prompt_drafts  SET prompt_name = 'system' WHERE prompt_name = 'system_main';

-- Сидируем classification v1 — короткий промпт-классификатор.
-- Результат — одно слово: recommend / clarify / chitchat / off_topic.
INSERT INTO prompts (name, version, content, published_by)
SELECT
    'classification',
    1,
    'Ты — классификатор намерений в чате гостя с ассистентом ресторана.
По сообщению пользователя определи одну из четырёх категорий:
- recommend — пользователь хочет получить новые рекомендации блюд (запрос про еду, бюджет, тип кухни, диету и т.п.)
- clarify — пользователь уточняет/расспрашивает про уже обсуждаемые блюда или просит дополнительную деталь
- chitchat — приветствие, благодарность, прощание, светская болтовня БЕЗ вопроса про меню
- off_topic — запрос совсем не про ресторан/еду/меню (математика, код, политика и т.п.)

Если в сообщении есть И приветствие/благодарность, И вопрос про меню — это recommend (либо clarify, если речь про уже обсуждавшиеся блюда).

Сообщение пользователя:
{{user_message}}

Ответь СТРОГО одним словом: recommend, clarify, chitchat или off_topic. Без точек, без пояснений.',
    (SELECT id FROM users WHERE role = 'admin' ORDER BY created_at LIMIT 1)
WHERE EXISTS (SELECT 1 FROM users WHERE role = 'admin');

-- Сидируем refusal v1 — короткий вежливый отказ для off_topic.
INSERT INTO prompts (name, version, content, published_by)
SELECT
    'refusal',
    1,
    'Пользователь задал вопрос, не относящийся к ресторану или меню.
Вежливо откажи в одну фразу. Скажи, что ты — ассистент ресторана и можешь помочь с подбором блюд, информацией о меню или заказом.
Не комментируй сам вопрос пользователя по существу. Не давай советов по теме, которую он затронул.

Сообщение пользователя:
{{user_message}}

В самом конце ответа отдельной строкой ОБЯЗАТЕЛЬНО добавь блок (ничего после него):
```json
{"recommended_dish_ids":[]}
```',
    (SELECT id FROM users WHERE role = 'admin' ORDER BY created_at LIMIT 1)
WHERE EXISTS (SELECT 1 FROM users WHERE role = 'admin');
