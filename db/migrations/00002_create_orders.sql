-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS orders (
    id          UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID           NOT NULL,
    status      VARCHAR(50)    NOT NULL DEFAULT 'pending',
    total_price DECIMAL(12,2)  NOT NULL DEFAULT 0,
    currency    VARCHAR(3)     NOT NULL DEFAULT 'USD',
    note        TEXT,
    version     INT            NOT NULL DEFAULT 1,       -- optimistic locking
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_orders_user_id ON orders(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_orders_status  ON orders(status)   WHERE deleted_at IS NULL;
CREATE INDEX idx_orders_deleted ON orders(deleted_at);

CREATE TABLE IF NOT EXISTS order_items (
    id          UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID           NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id  UUID           NOT NULL,
    name        VARCHAR(255)   NOT NULL,
    quantity    INT            NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price  DECIMAL(12,2)  NOT NULL CHECK (unit_price >= 0),
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
-- +goose StatementEnd
