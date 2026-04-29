CREATE TABLE chats (
    id              uuid        PRIMARY KEY,
    user_id         uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title           text,
    last_message_at timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX chats_user_last_msg_idx ON chats (user_id, last_message_at DESC);

CREATE TABLE chat_messages (
    id                   uuid        PRIMARY KEY,
    chat_id              uuid        NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    role                 text        NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content              text        NOT NULL,
    recommended_dish_ids int[]       NOT NULL DEFAULT '{}',
    meta                 jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at           timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX chat_messages_chat_created_idx ON chat_messages (chat_id, created_at DESC);
