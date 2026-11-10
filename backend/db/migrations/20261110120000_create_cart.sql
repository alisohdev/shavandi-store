-- +goose Up
CREATE TABLE shavandi.cart_lines (
 user_id uuid NOT NULL REFERENCES shavandi.users(id) ON DELETE CASCADE,
 variant_id uuid NOT NULL REFERENCES shavandi.variants(id) ON DELETE CASCADE,
 quantity integer NOT NULL CHECK (quantity BETWEEN 1 AND 9),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (user_id, variant_id)
);

-- +goose Down
DROP TABLE shavandi.cart_lines;
