-- Корзина пользователя: один активный cart на user_id (UNIQUE).
-- Содержимое: позиции (cart_items) с FK на dishes. При удалении блюда из меню
-- (в нашей системе блюда не удаляются физически — только is_available=false),
-- запись в cart_items сохраняется, но в выдаче помечается available=false.
CREATE TABLE carts (
    id          uuid        PRIMARY KEY,
    user_id     uuid        NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- Позиции корзины. PRIMARY KEY (cart_id, dish_id) гарантирует, что одно блюдо
-- не дублируется — добавление того же dish_id увеличивает quantity.
-- ON DELETE RESTRICT для dish_id: блюдо нельзя физически удалить, если оно в чьей-то корзине;
-- в реальности is_available переводят в false, но строка cart_item остаётся.
CREATE TABLE cart_items (
    cart_id    uuid        NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    dish_id    integer     NOT NULL REFERENCES dishes(id) ON DELETE RESTRICT,
    quantity   integer     NOT NULL CHECK (quantity > 0 AND quantity <= 50),
    note       text,
    sort_order integer     NOT NULL DEFAULT 0,
    added_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (cart_id, dish_id)
);

CREATE INDEX cart_items_cart_sort_idx ON cart_items (cart_id, sort_order, added_at);
