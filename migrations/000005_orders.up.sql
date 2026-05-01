-- Заказы пользователя.
--
-- order_status: жизненный цикл заказа.
--   accepted  — принят (создан гостем; кухня видит и берёт в работу автоматически)
--   cooking   — готовится
--   ready     — готов к выдаче / отправке курьеру
--   in_delivery — в доставке (для fulfillment_type=delivery)
--   closed    — финал: гость получил / съел в зале
--   cancelled — отменён (admin'ом до closed; в B2 — без customer-cancel endpoint)
--
-- fulfillment_type: способ выдачи (delivery / pickup / dine_in)
-- payment_method: способ оплаты (on_delivery / online_stub — заглушка под будущее)
CREATE TYPE order_status AS ENUM ('accepted', 'cooking', 'ready', 'in_delivery', 'closed', 'cancelled');

CREATE TYPE order_fulfillment AS ENUM ('delivery', 'pickup', 'dine_in');

CREATE TYPE order_payment_method AS ENUM ('on_delivery', 'online_stub');

CREATE TABLE orders (
    id                  uuid                 PRIMARY KEY,
    user_id             uuid                 NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status              order_status         NOT NULL DEFAULT 'accepted',
    fulfillment_type    order_fulfillment    NOT NULL,
    payment_method      order_payment_method NOT NULL,
    total_minor         integer              NOT NULL CHECK (total_minor >= 0),
    currency            text                 NOT NULL DEFAULT 'RUB',
    -- snapshot контактов гостя на момент заказа
    customer_first_name text                 NOT NULL,
    customer_last_name  text                 NOT NULL,
    customer_phone      text                 NOT NULL,
    customer_email      text,
    -- delivery-specific (NULL для pickup / dine_in)
    delivery_address    text,
    delivery_notes      text,
    -- комментарий гостя к заказу
    notes               text,
    created_at          timestamptz          NOT NULL DEFAULT now(),
    updated_at          timestamptz          NOT NULL DEFAULT now()
);

CREATE INDEX orders_user_created_idx   ON orders (user_id, created_at DESC);
CREATE INDEX orders_status_created_idx ON orders (status, created_at DESC);

-- Позиции заказа: snapshot названия и цены на момент создания, чтобы изменение
-- меню после оформления не пересчитывало старые заказы. dish_id остаётся FK для
-- аналитики, но даже если когда-нибудь блюдо физически удалят — RESTRICT не даст
-- (плюс контент в name_snapshot всё равно сохранён).
CREATE TABLE order_items (
    order_id             uuid     NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    dish_id              integer  NOT NULL REFERENCES dishes(id) ON DELETE RESTRICT,
    name_snapshot        text     NOT NULL,
    price_minor_snapshot integer  NOT NULL CHECK (price_minor_snapshot >= 0),
    quantity             integer  NOT NULL CHECK (quantity > 0),
    sort_order           integer  NOT NULL DEFAULT 0,
    PRIMARY KEY (order_id, dish_id)
);

CREATE INDEX order_items_order_idx ON order_items (order_id, sort_order);
