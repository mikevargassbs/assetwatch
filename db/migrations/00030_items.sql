-- +goose Up
CREATE TABLE items (
    id                 SERIAL PRIMARY KEY,
    make               TEXT NOT NULL,
    model              TEXT NOT NULL,
    description        TEXT,
    qty                INTEGER NOT NULL DEFAULT 0,
    sales_order_number TEXT,
    active             BOOLEAN NOT NULL DEFAULT TRUE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE items;
