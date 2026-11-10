-- +goose Up
CREATE TABLE shavandi.order_deliveries (
 order_id uuid PRIMARY KEY REFERENCES shavandi.orders(id),
 recipient text NOT NULL CHECK(length(btrim(recipient)) BETWEEN 2 AND 100),
 phone text NOT NULL CHECK(phone ~ '^\+989[0-9]{9}$'),
 province text NOT NULL CHECK(length(btrim(province)) BETWEEN 2 AND 100),
 city text NOT NULL CHECK(length(btrim(city)) BETWEEN 2 AND 100),
 address text NOT NULL CHECK(length(btrim(address)) BETWEEN 10 AND 500),
 postal_code text NOT NULL CHECK(postal_code ~ '^[0-9]{10}$'),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 updated_at timestamptz NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE shavandi.order_deliveries;
