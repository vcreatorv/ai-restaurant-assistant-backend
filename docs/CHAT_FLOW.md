# Chat — генерация ответа ассистента

Документ описывает, что происходит между моментом, когда пользователь
отправил сообщение в `POST /chats/{id}/messages`, и моментом, когда
ассистент ответил полным SSE-стримом.

Соответствующий код:
- `internal/chat/usecase/chat.go` — оркестрация
- `internal/chat/usecase/prompt.go` — сборка system+user-prompt'а
- `internal/pkg/cohere/cohere.go` — embed + rerank HTTP-клиент
- `internal/pkg/qdrant/qdrant.go` — векторный поиск
- `internal/pkg/llm/llm.go` — общий HTTP/SSE-клиент chat-completions
- `internal/pkg/openrouter/openrouter.go` — фабрика llm.Client для OpenRouter
- `internal/pkg/nvidia/nvidia.go` — фабрика llm.Client для NVIDIA NIM (build.nvidia.com)
- `internal/chat/delivery/v1/http/chat.go` — SSE на клиента (через io.Pipe)

## Высокоуровневая схема

```
                       POST /chats/{id}/messages
                         body: { content: "..." }
                                  │
                                  ▼
              ┌──────────────────────────────────────┐
              │ delivery/v1/http: parse, auth, csrf  │
              └──────────────────────────────────────┘
                                  │
                                  ▼
              ┌──────────────────────────────────────┐
              │ usecase.SendMessage:                 │
              │  1. validate content                 │
              │  2. AppendMessage(user)  ──────► PG  │
              │  3. Users.GetByID         ──────► PG │
              │  4. loadHistory:                     │
              │     - FindFirstUserMessage ──► PG    │
              │     - ListMessages(N*2)    ──► PG    │
              │  5. runRAG ───┐                      │
              │  6. buildPrompt                      │
              │  7. LLM stream  ──────► onDelta(SSE) │
              │  8. parseLLMTail                     │
              │  9. AppendMessage(assistant) ──► PG  │
              │ 10. OnDone(SSE)                      │
              └──────────────────────────────────────┘
                                  │
                                  ▼
                    ┌──────────────────────┐
                    │ runRAG (см. ниже)    │
                    └──────────────────────┘
```

## RAG-pipeline (детали)

```
                   currentMessage (только current, без истории)
                                │
                                ▼
                ┌──────────────────────────────┐
                │ Cohere.Embed                 │
                │   model: embed-multilingual  │
                │   input_type: search_query   │
                │   → vector[1024]             │
                └──────────────────────────────┘
                                │
                                ▼
                ┌──────────────────────────────┐
                │ Qdrant.Search (PRIMARY)      │
                │   filter:                    │
                │     must: is_available=true  │
                │           dietary=<profile>  │
                │     must_not: allergens=...  │
                │   top_k: 20                  │
                │   → 20 hits                  │
                └──────────────────────────────┘
                                │
                                ▼
                ┌──────────────────────────────┐
                │ Menu.GetDishesByIDs (PG)     │
                │   подгрузка имён, описаний,  │
                │   составов, цен, кухонь,     │
                │   категорий                  │
                └──────────────────────────────┘
                                │
                                ▼
                ┌──────────────────────────────┐
                │ Cohere.Rerank                │
                │   model: rerank-multilingual │
                │   docs: 20 текстов dish      │
                │   top_n: 5                   │
                │   → 5 ID + scores            │
                │   fallback: если все scores  │
                │     ниже min_score, всё равно│
                │     возвращаем top-N         │
                └──────────────────────────────┘
                                │
                                ▼
                ┌──────────────────────────────┐
                │ Diversify main (НА ШИРОКИХ ЗАПРОСАХ) │
                │  Если в reranked top-N покрыто       │
                │  < cfg.MainMinCategories разных      │
                │  main-категорий → для каждой         │
                │  непокрытой Qdrant.Search top-1      │
                │  с filter category_id, добавляем     │
                │  в main только если score >=         │
                │  cfg.MainDiversifyMinScore.          │
                │  Лимит cfg.MainMaxAdded.             │
                │  На узких запросах ничего не добав-  │
                │  ляет (порог отсекает нерелевантное).│
                └──────────────────────────────┘
                                │
                ┌───────────────┴─────────────────┐
                ▼                                 ▼
       Main: top-N + diversified         For each companion category:
                                          (Соусы, Гарниры, Закуски,
                                           Десерты, Напитки безалк.)
                                                  │
                                                  ▼
                                       Qdrant.Search (COMPANION)
                                          base filter +
                                          must: category_id=<cat>
                                          top_k: 5
                                          take top-1, dedup by id with main
                                          SKIP, если категория уже в main
                                          (mainCatSet) — не «два хлеба»
                                                  │
                                                  ▼
                                          Menu.GetDishesByIDs (PG)
                                                  │
                                                  ▼
                                       Companion top-1 per category
                │                                 │
                └───────────────┬─────────────────┘
                                ▼
                       ragResult{main, companions, retrievedIDs, rerankedIDs}
```

### Параметры (в `configs/config.yaml`)

```yaml
rag:
  cohere:
    embed_model:    embed-multilingual-v3.0     # ↦ vector[1024]
    rerank_model:   rerank-multilingual-v3.0
    embed_batch_size: 96
    retry_attempts:   2
    retry_delay:      500ms
  qdrant:
    collection:       dishes
    distance:         Cosine
    hnsw: { m: 16, ef_construct: 100 }
  search:
    top_k:            20    # сколько кандидатов из Qdrant до rerank
    rerank_top_n:     5     # сколько остаётся после rerank
    rerank_min_score: 0.01  # минимум; ниже → fallback к top-N без min
  chat:
    history_recent_pairs: 2
    companions:
      - "Соусы"
      - "Гарниры"
      - "Закуски"
      - "Десерты"
      - "Напитки безалкогольные"
    main_categories:                  # «обеденные» категории для диверсификации main
      - "Супы"
      - "Горячее — мясо"
      - "Горячее — рыба и морепродукты"
      - "Стейки"
      - "Гриль (хоспер)"
      - "Бургеры"
      - "Пицца и паста"
      - "Салаты"
    main_min_categories: 3            # если в reranked top-N покрыто < 3 категорий → диверсифицируем
    main_max_added: 3                 # сколько максимум блюд добавлять
    main_diversify_min_score: 0.4     # cosine-порог: ниже — категория не релевантна запросу
  llm:
    provider: openrouter         # openrouter | nvidia (переопр. LLM_PROVIDER env)
    common:
      temperature:          0.4
      max_tokens:           1500
      request_timeout:      90s
      first_token_timeout:  30s
    openrouter:
      base_url: https://openrouter.ai/api/v1
      api_key: ""                # OPENROUTER_API_KEY
      model: openrouter/free
      referer: http://localhost:8081
      title: AI Restaurant Assistant
    nvidia:
      base_url: https://integrate.api.nvidia.com/v1
      api_key: ""                # NVIDIA_API_KEY (nvapi-... с build.nvidia.com)
      model: meta/llama-3.3-70b-instruct
```

### Переключение LLM-провайдера

В `cmd/app/app/app.go::buildLLMClient` switch по `cfg.RAG.LLM.Provider`:

| provider     | фабрика                      | базовый URL                                     | специфика                                  |
|--------------|------------------------------|-------------------------------------------------|--------------------------------------------|
| `openrouter` | `internal/pkg/openrouter`    | `https://openrouter.ai/api/v1`                  | headers `HTTP-Referer`, `X-Title`          |
| `nvidia`     | `internal/pkg/nvidia`        | `https://integrate.api.nvidia.com/v1`           | без extra headers; ключ `nvapi-...`        |

Сам HTTP/SSE-клиент один (`internal/pkg/llm`), отличаются только `Provider` (для логов), `BaseURL`, `APIKey`, `Model`, `ExtraHeaders`. Переключение в runtime — `LLM_PROVIDER=nvidia make docker-up` (или поправь yaml + перезапуск).

## Qdrant — payload и индексы

В Qdrant у каждой точки `id == dish_id` (uint64). **Тексты блюд (name,
description, composition) НЕ хранятся в Qdrant** — за ними мы ходим в PG.
Это критично: pre-filter работает по payload, а не по семантике.

Payload каждой точки:

| Поле            | Тип       | Индексирован | Зачем                                 |
|-----------------|-----------|--------------|---------------------------------------|
| `dish_id`       | int       | ✓            | для дедупа / debug                    |
| `category_id`   | int       | ✓            | companion-фильтр (категория)          |
| `cuisine`       | keyword   | ✓            | для фильтров «итальянская» и т.п.     |
| `allergens`     | keyword[] | ✓            | hard-filter `must_not` из профиля     |
| `dietary`       | keyword[] | ✓            | hard-filter `must` из профиля         |
| `tag_ids`       | int[]     | ✓            | теги (Острое / Шеф рекомендует / ...) |
| `price_minor`   | int       | ✓            | для возможных бюджет-фильтров         |
| `is_available`  | bool      | ✓            | hard-filter `must=true` (стоп-лист)   |
| `calories_kcal` | int       | ✓            | для возможных диет-фильтров           |
| `portion_weight_g` | int    | ✓            | для возможных диет-фильтров           |

Индексы создаются один раз `cmd/embed-menu` через `EnsurePayloadIndexes`.
Без индекса pre-filter в Qdrant работает медленно (full scan).

## Hard-фильтры (профиль гостя → Qdrant)

Из `User.Allergens` и `User.Dietary` в `buildPrefilter` собирается фильтр:

```json
{
  "must": [
    {"key": "is_available", "match": {"value": true}},
    {"key": "dietary",      "match": {"value": "vegan"}},      // если в профиле
    {"key": "dietary",      "match": {"value": "halal"}}       // и т.д.
  ],
  "must_not": [
    {"key": "allergens", "match": {"value": "dairy"}},          // если в профиле
    {"key": "allergens", "match": {"value": "gluten"}}          // и т.д.
  ]
}
```

Семантика:
- `must dietary=X` — у блюда массив `dietary` должен содержать X
  (Qdrant трактует match по keyword[] как «contains»).
- `must_not allergens=X` — у блюда в `allergens` НЕ должно быть X.
- `must is_available=true` — блюдо не в стоп-листе.

Этот же базовый фильтр используется и в companion-search'ах, плюс к нему
добавляется `must category_id=<X>`.

## Контекст для LLM (prompt)

`buildPrompt` собирает массив `messages` для chat-completions:

```
[
  {role: "system",    content: <systemPrompt>},                  // 22 правила
  {role: "user"|"assistant", content: <anchor>},                 // если есть
  {role: ..., content: <recent[0]>},                             // последние
  {role: ..., content: <recent[1]>},                             // pairs * 2
  {role: ..., content: <recent[2]>},                             // сообщений
  {role: ..., content: <recent[3]>},
  {role: "user",      content: <buildUserContent(in)>}           // RAG + current
]
```

### Anchor + recent pairs

- **Anchor** — хронологически первое user-msg чата (`FindFirstUserMessage`).
  Закрепляет рамку диалога: даже после 10 ходов ассистент видит
  «изначальный запрос» в начале.
- **Recent pairs** — последние `chat.history_recent_pairs * 2` сообщений
  (по умолчанию 4 = 2 пары).
- Если anchor совпадает с `recent[0]` (короткий чат) — anchor
  выкидывается, чтобы не дублировать.

### `buildUserContent` (последний user-msg)

```
=== Меню (наиболее подходящие блюда) ===

1. <name> [id=<N>] — <category>, <cuisine>. Цена: <minor/100> ₽.
   <description>
   Состав: <composition>

2. ...
   ...

5. ...

id блюд использовать ТОЛЬКО в финальном JSON-блоке. В тексте называй
блюда по имени.

=== Сопровождение (на выбор, если уместно к запросу) ===
- <name> [id=<N>] (<category>) — <description> Состав: <composition>.
- <name> [id=<N>] (<category>) — ...
...
Используй эти позиции как аккомпанемент к основным блюдам по правилам
Сочетаний (см. system).

=== Ранее рекомендовано в этом диалоге ===
<id1>, <id2>, <id3>                           # если есть

=== Вопрос гостя ===
<currentUserText>
```

### Системный промпт (правила)

Полный текст в `internal/chat/usecase/prompt.go`. Краткое резюме:

| #     | Правило                                                                |
|-------|------------------------------------------------------------------------|
| 1     | Только из списка ниже, не выдумывать                                   |
| 2     | Если нет — сказать честно                                              |
| 3     | Не упоминать аллергены/диету (контекст уже отфильтрован)               |
| 4     | Без id и цен в тексте                                                  |
| 5     | На «спасибо» — короткий ответ без рекомендаций                         |
| 6     | На off-topic — переключить на меню                                     |
| 7     | Без штампов («изумительный», «разогнать аппетит» и т.п.)               |
| 8     | Стиль: эпитеты вкуса/текстуры, не голый состав                         |
| 9     | Незнакомое слово через тире («фокачча — итальянская лепёшка с …»)      |
| 10    | Широкий запрос → 3-4 блюда; узкий → 1-3                                |
| 11    | Связки варьировать («На старт», «К ней», «В финал» …)                  |
| 12    | По-русски, естественно; длина по запросу                               |
| 13-19 | Сочетания: суп→хлеб, мясо→соус, рыба→лимонад/сок, десерт→чай и т.п.    |
| 20    | Алкоголь — только по явному запросу гостя                              |
| 21    | Сопровождение — короткой фразой, не навязывать                         |
| 22    | В конце — JSON-блок `{"recommended_dish_ids":[...]}`                   |

## Парсинг ответа LLM

`parseLLMTail` ищет JSON-блок в конце ответа:

1. **Fenced**: ```` ```json\n{...}\n``` ```` — основной формат;
2. **Bare tail**: голый `{...}` в самом конце (на случай, если модель
   не обернула в markdown-блок);
3. Если не нашли — `recommended_dish_ids = []`.

Из ответа JSON-блок **вырезается**, в БД сохраняется чистый текст.
`recommended_dish_ids` парсится в массив int.

В meta для аналитики сохраняется три набора id:
- `retrieved_ids` — top-K от Qdrant ДО rerank;
- `reranked_ids`  — top-N ПОСЛЕ rerank (что ушло в LLM);
- `companion_ids` — id companion-блюд (что ушло в LLM как сопровождение);
- `recommended_dish_ids` (на колонке, не в meta) — то, что LLM **реально**
  упомянула в JSON-блоке.

## Логи (что искать в `docker compose logs -f app`)

Все логи json, во всех — `request_id` (через `logger.ForCtx(ctx)`).

Полная цепочка одного `SendMessage`:

| msg                              | level | stage                     | главные поля                              |
|----------------------------------|-------|---------------------------|-------------------------------------------|
| `chat pipeline start`            | INFO  | `begin`                   | content_preview, content_len              |
| `chat history loaded`            | DEBUG | `history`                 | anchor_present, recent_count              |
| `cohere embed ok`                | INFO  | (cohere)                  | batch, dim, duration_ms                   |
| `rag embed done`                 | DEBUG | `rag.embed`               | embed_ms, dim                             |
| `qdrant search ok`               | INFO  | (qdrant)                  | hits, top_score, filter_summary, duration |
| `rag primary search done`        | DEBUG | `rag.search.primary`      | hits, top_score                           |
| `cohere rerank ok`               | INFO  | (cohere)                  | docs, top_n, top_score, duration_ms       |
| `rag rerank scores`              | DEBUG | `rag.rerank`              | scores, kept, dropped_below_min, min      |
| `rag rerank all below min …`     | WARN  | `rag.rerank`              | top_score, min_score                      |
| `rag rerank done`                | DEBUG | `rag.rerank`              | input, output, rerank_ms                  |
| `rag main coverage`              | DEBUG | `rag.diversify`           | covered_main_categories, min_required     |
| `rag diversify search done`      | DEBUG | `rag.diversify`           | category, category_id, top_score, min_score |
| `rag main diversified`           | INFO  | `rag.diversify`           | added_ids, added_count                    |
| `rag companion skipped (...)`    | DEBUG | `rag.search.companion`    | category, category_id (категория уже в main) |
| `qdrant search ok` × N           | INFO  | (qdrant — companion)      | hits, top_score, filter_summary           |
| `rag companion search done` × N  | DEBUG | `rag.search.companion`    | category, category_id, hits, top_score    |
| `chat rag retrieved`             | INFO  | `rag`                     | retrieved_ids, reranked_ids, companion_ids|
| `chat llm prompt assembled`      | DEBUG | `llm.prompt`              | messages_count, main_dishes, companions   |
| `openrouter chat ok`             | INFO  | (openrouter)              | model_actual, ttft_ms, duration_ms, tokens|
| `chat llm response`              | INFO  | `llm.stream`              | model_actual, llm_ms, finish_reason       |
| `chat llm parsed`                | DEBUG | `llm.parse`               | clean_text_preview, llm_recommended       |
| `http`                           | INFO  | (middleware)              | method, path, status, duration_ms         |

При ошибках сетевых клиентов:

| msg                              | level |
|----------------------------------|-------|
| `cohere http retry`              | WARN  |
| `cohere http call failed`        | ERROR |
| `cohere embed/rerank failed`     | ERROR |
| `cohere embed/rerank non-2xx`    | ERROR |
| `openrouter http retry`          | WARN  |
| `openrouter http call failed`    | ERROR |
| `openrouter chat non-2xx`        | ERROR |
| `openrouter chat stream failed`  | ERROR |
| `qdrant http call failed`        | ERROR |
| `qdrant http non-2xx`            | ERROR |
| `qdrant search failed`           | ERROR |

В каждом retry-логе есть `attempt`, `max_attempts`, `remaining`, `err`, `duration_ms`,
`retry_delay_ms`. По `request_id` можно отфильтровать всю цепочку.

Пример выборки одной цепочки:

```sh
docker compose logs app | grep -F '"request_id":"f064ac7e-…"'
```

## Известные ограничения и trade-off'ы

1. **Embed без истории.** Только current_message эмбеддится. Это
   намеренно: подмешивание истории смазывает вектор и rerank теряет
   фокус («что у вас острого без молочки» + предыдущий «соберите ужин»
   → выдача стейков). Контекст диалога LLM получает через `messages`,
   не через retrieval.
2. **Companion top-1 без rerank.** Companion ищется одним Qdrant.Search
   с category_id фильтром, top_k=5, берётся top-1 по score. Без rerank —
   ради латентности; companion это «лучшее из категории, близкое к
   запросу», точность ±1 позиция нам некритична.
   Companion для категории НЕ запускается, если в main уже есть блюдо
   из этой категории (избегаем «два хлеба» / «два соуса»).
2.5. **Diversify main на широких запросах.** На запросе «что покушать»
   эмбеддинг семантически близок к Салатам/Закускам, и весь top-5 после
   rerank может оказаться из 1-2 категорий. Тогда LLM не может собрать
   «суп→горячее→десерт». Решение: после rerank считаем уникальные
   main-категории в top-N; если меньше `MainMinCategories` — для каждой
   непокрытой делаем Qdrant.Search top-1 и добавляем, если score >=
   `MainDiversifyMinScore`. На узких запросах («хочу пиццу») остальные
   категории дают score ~0.3 и не проходят порог — поведение не меняется.
3. **`finish_reason: length`.** Если max_tokens=800 закончился до того,
   как LLM написала JSON-блок, `parseLLMTail` не находит блок и
   `recommended_dish_ids = []`. Это «тихая» ошибка: ответ есть, но
   аналитика по рекомендациям пустая. Видно в `chat llm response` логе.
4. **Rerank fallback при «всё ниже min_score».** Ставит флаг WARN
   `rag rerank all below min …`. Это сигнал, что запрос пользователя
   не подходит к меню; ответ может быть нерелевантным.
5. **Алкоголь** в companion НЕ извлекается — только в primary, если
   гость явно попросил. См. system prompt п.20.
6. **Дубликаты user-msg при ретрае LLM.** Если LLM упал по таймауту,
   user-msg уже в БД (намеренно — не теряем ввод). Пользователь
   нажимает «отправить» снова → второй user-msg. Сейчас не схлопывается.
