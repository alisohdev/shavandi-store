-- +goose Up
ALTER TABLE shavandi.orders DROP CONSTRAINT orders_status_check;
ALTER TABLE shavandi.orders ADD CONSTRAINT orders_status_check CHECK(status IN ('pending_confirmation','cancelled','expired','paid','payment_review'));
CREATE TABLE shavandi.order_payments (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 order_id uuid NOT NULL UNIQUE REFERENCES shavandi.orders(id),
 environment text NOT NULL CHECK(environment IN ('sandbox','production')),
 merchant_fingerprint text NOT NULL CHECK(merchant_fingerprint ~ '^[a-f0-9]{64}$'),
 amount_rial bigint NOT NULL CHECK(amount_rial>0),
 delivery_version bigint NOT NULL CHECK(delivery_version>0),
 authority text UNIQUE CHECK(authority ~ '^[AS][A-Za-z0-9]{35}$'),
 status text NOT NULL CHECK(status IN ('requesting','ready','unknown','failed','succeeded')),
 ref_id bigint CHECK(ref_id>0),
 busy_until timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(environment,merchant_fingerprint,ref_id),
 CHECK((status='succeeded' AND ref_id IS NOT NULL AND authority IS NOT NULL) OR (status<>'succeeded' AND ref_id IS NULL)),
 CHECK(status<>'ready' OR authority IS NOT NULL)
);
-- +goose Down
-- Refuse downgrade while paid orders exist; their accounting records must be retained.
ALTER TABLE shavandi.orders DROP CONSTRAINT orders_status_check;
ALTER TABLE shavandi.orders ADD CONSTRAINT orders_status_check CHECK(status IN ('pending_confirmation','cancelled','expired'));
DROP TABLE shavandi.order_payments;
