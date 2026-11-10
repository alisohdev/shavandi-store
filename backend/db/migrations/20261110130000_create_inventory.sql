-- +goose Up
ALTER TABLE shavandi.variants
 ADD COLUMN stock_on_hand integer NOT NULL DEFAULT 0 CHECK (stock_on_hand BETWEEN 0 AND 1000000),
 ADD COLUMN stock_version bigint NOT NULL DEFAULT 1 CHECK (stock_version > 0);
CREATE TABLE shavandi.stock_changes (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 variant_id uuid NOT NULL REFERENCES shavandi.variants(id),
 actor_id uuid NOT NULL REFERENCES shavandi.users(id),
 previous_quantity integer NOT NULL CHECK (previous_quantity BETWEEN 0 AND 1000000),
 new_quantity integer NOT NULL CHECK (new_quantity BETWEEN 0 AND 1000000),
 version bigint NOT NULL CHECK (version > 1),
 reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 1 AND 300),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(variant_id,version)
);
-- +goose Down
DROP TABLE shavandi.stock_changes;
ALTER TABLE shavandi.variants DROP COLUMN stock_on_hand, DROP COLUMN stock_version;
