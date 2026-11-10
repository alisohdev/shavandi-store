-- +goose Up
ALTER TABLE shavandi.variants ADD COLUMN reserved_quantity integer NOT NULL DEFAULT 0,
 ADD CONSTRAINT variants_reservations CHECK (reserved_quantity BETWEEN 0 AND stock_on_hand);
CREATE TABLE shavandi.orders (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 user_id uuid NOT NULL REFERENCES shavandi.users(id),
 request_key uuid NOT NULL,
 quote_token text NOT NULL CHECK (quote_token ~ '^[a-f0-9]{64}$'),
 status text NOT NULL CHECK (status IN ('pending_confirmation','cancelled','expired')),
 subtotal_toman bigint NOT NULL CHECK (subtotal_toman BETWEEN 0 AND 9007199254740991),
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT (now()+interval '15 minutes'),
 UNIQUE(user_id,request_key), CHECK(expires_at>created_at)
);
CREATE UNIQUE INDEX orders_one_pending_per_customer ON shavandi.orders(user_id) WHERE status='pending_confirmation';
CREATE INDEX orders_expiring ON shavandi.orders(expires_at) WHERE status='pending_confirmation';
CREATE TABLE shavandi.order_lines (
 order_id uuid NOT NULL REFERENCES shavandi.orders(id),
 variant_id uuid NOT NULL REFERENCES shavandi.variants(id),
 name text NOT NULL,
 size text,
 color text,
 quantity integer NOT NULL CHECK (quantity BETWEEN 1 AND 9),
 unit_price_toman bigint NOT NULL CHECK (unit_price_toman BETWEEN 0 AND 9007199254740991),
 PRIMARY KEY(order_id,variant_id)
);
-- +goose Down
DROP TABLE shavandi.order_lines;
DROP TABLE shavandi.orders;
ALTER TABLE shavandi.variants DROP CONSTRAINT variants_reservations, DROP COLUMN reserved_quantity;
