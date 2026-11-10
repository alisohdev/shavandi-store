-- +goose Up
CREATE TABLE shavandi.shipping_methods (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 100),
 fee_toman bigint NOT NULL CHECK(fee_toman BETWEEN 0 AND 1000000000),
 active boolean NOT NULL DEFAULT true,
 version bigint NOT NULL DEFAULT 1 CHECK(version>0)
);
ALTER TABLE shavandi.order_deliveries
 ADD COLUMN shipping_method_id uuid REFERENCES shavandi.shipping_methods(id),
 ADD COLUMN shipping_version bigint,
 ADD COLUMN shipping_name text,
 ADD COLUMN shipping_fee_toman bigint CHECK(shipping_fee_toman BETWEEN 0 AND 1000000000),
 ADD CONSTRAINT order_delivery_shipping_complete CHECK(
 (shipping_method_id IS NULL AND shipping_version IS NULL AND shipping_name IS NULL AND shipping_fee_toman IS NULL)
 OR (shipping_method_id IS NOT NULL AND shipping_version>0 AND shipping_name IS NOT NULL AND shipping_fee_toman IS NOT NULL));
-- +goose Down
ALTER TABLE shavandi.order_deliveries DROP CONSTRAINT order_delivery_shipping_complete,
 DROP COLUMN shipping_method_id,DROP COLUMN shipping_version,DROP COLUMN shipping_name,DROP COLUMN shipping_fee_toman;
DROP TABLE shavandi.shipping_methods;
