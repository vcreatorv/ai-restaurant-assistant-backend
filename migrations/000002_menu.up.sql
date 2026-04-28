CREATE TABLE IF NOT EXISTS categories (
    id           serial      PRIMARY KEY,
    name         text        NOT NULL UNIQUE,
    sort_order   int         NOT NULL DEFAULT 0,
    is_available boolean     NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tags (
    id         serial      PRIMARY KEY,
    name       text        NOT NULL UNIQUE,
    slug       text        NOT NULL UNIQUE,
    color      text        NOT NULL DEFAULT '#888888',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS dishes (
    id               serial       PRIMARY KEY,
    name             text         NOT NULL UNIQUE,
    description      text         NOT NULL DEFAULT '',
    composition      text         NOT NULL DEFAULT '',
    image_url        text         NOT NULL DEFAULT '',

    price_minor      int          NOT NULL CHECK (price_minor BETWEEN 1 AND 100000000),
    currency         text         NOT NULL DEFAULT 'RUB',

    calories_kcal    int          CHECK (calories_kcal IS NULL OR calories_kcal >= 0),
    protein_g        numeric(5,1) CHECK (protein_g    IS NULL OR protein_g    >= 0),
    fat_g            numeric(5,1) CHECK (fat_g        IS NULL OR fat_g        >= 0),
    carbs_g          numeric(5,1) CHECK (carbs_g      IS NULL OR carbs_g      >= 0),
    portion_weight_g int          CHECK (portion_weight_g IS NULL OR portion_weight_g > 0),

    cuisine          text         NOT NULL
                     CHECK (cuisine IN ('russian','italian','japanese','french','asian','european','american')),
    category_id      int          NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,

    allergens        text[]       NOT NULL DEFAULT '{}',
    dietary          text[]       NOT NULL DEFAULT '{}',

    is_available     boolean      NOT NULL DEFAULT true,
    created_at       timestamptz  NOT NULL DEFAULT now(),
    updated_at       timestamptz  NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS dish_tags (
    dish_id int NOT NULL REFERENCES dishes(id) ON DELETE CASCADE,
    tag_id  int NOT NULL REFERENCES tags(id)   ON DELETE CASCADE,
    PRIMARY KEY (dish_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_dishes_category   ON dishes(category_id);
CREATE INDEX IF NOT EXISTS idx_dishes_available  ON dishes(is_available);
CREATE INDEX IF NOT EXISTS idx_dishes_allergens  ON dishes USING gin(allergens);
CREATE INDEX IF NOT EXISTS idx_dishes_dietary    ON dishes USING gin(dietary);
CREATE INDEX IF NOT EXISTS idx_dish_tags_tag     ON dish_tags(tag_id);
