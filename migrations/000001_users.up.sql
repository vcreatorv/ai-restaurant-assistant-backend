CREATE TABLE users (
    id            UUID        PRIMARY KEY,
    email         TEXT,
    password_hash TEXT,
    role          TEXT        NOT NULL CHECK (role IN ('guest', 'customer', 'admin')),
    first_name    TEXT,
    last_name     TEXT,
    phone         TEXT,
    allergens     TEXT[]      NOT NULL DEFAULT '{}',
    dietary       TEXT[]      NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Не-guest обязан иметь email и password_hash.
    CONSTRAINT users_registered_has_creds
        CHECK (role = 'guest' OR (email IS NOT NULL AND password_hash IS NOT NULL))
);

-- Email уникален среди не-guest. Partial index чтобы guest'ы могли иметь email = NULL.
CREATE UNIQUE INDEX users_email_unique ON users (email) WHERE email IS NOT NULL;
