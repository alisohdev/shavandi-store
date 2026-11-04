-- +goose Up
CREATE TABLE shavandi.categories (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (length(btrim(name)) > 0),
    slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    parent_id uuid REFERENCES shavandi.categories(id) ON DELETE RESTRICT,
    display_order integer NOT NULL DEFAULT 0 CHECK (display_order >= 0),
    active boolean NOT NULL DEFAULT true,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE INDEX categories_parent_order
    ON shavandi.categories (parent_id, display_order, id);

CREATE TABLE shavandi.products (
    id uuid PRIMARY KEY,
    primary_category_id uuid NOT NULL REFERENCES shavandi.categories(id) ON DELETE RESTRICT,
    name text NOT NULL CHECK (length(btrim(name)) > 0),
    description text NOT NULL DEFAULT '',
    regular_price_toman bigint NOT NULL CHECK (regular_price_toman >= 0),
    sale_price_toman bigint,
    publication_state text NOT NULL DEFAULT 'draft'
        CHECK (publication_state IN ('draft', 'published', 'hidden')),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        sale_price_toman IS NULL
        OR (sale_price_toman >= 0 AND sale_price_toman < regular_price_toman)
    )
);

CREATE INDEX products_public_ordering
    ON shavandi.products (created_at DESC, id DESC)
    WHERE publication_state = 'published';

CREATE INDEX products_public_category_ordering
    ON shavandi.products (primary_category_id, created_at DESC, id DESC)
    WHERE publication_state = 'published';

CREATE TABLE shavandi.product_slugs (
    slug text PRIMARY KEY CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    product_id uuid NOT NULL REFERENCES shavandi.products(id) ON DELETE RESTRICT,
    is_current boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX product_slugs_one_current_per_product
    ON shavandi.product_slugs (product_id)
    WHERE is_current;

CREATE TABLE shavandi.variants (
    id uuid PRIMARY KEY,
    product_id uuid NOT NULL REFERENCES shavandi.products(id) ON DELETE RESTRICT,
    sku text NOT NULL UNIQUE CHECK (length(btrim(sku)) > 0),
    size text CHECK (size IS NULL OR length(btrim(size)) > 0),
    color text CHECK (color IS NULL OR length(btrim(color)) > 0),
    regular_price_override_toman bigint CHECK (regular_price_override_toman IS NULL OR regular_price_override_toman >= 0),
    sale_price_override_toman bigint CHECK (sale_price_override_toman IS NULL OR sale_price_override_toman >= 0),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX variants_active_combination
    ON shavandi.variants (product_id, size, color) NULLS NOT DISTINCT
    WHERE active;

CREATE INDEX variants_by_product
    ON shavandi.variants (product_id, active, size, color, id);

CREATE TABLE shavandi.product_images (
    id uuid PRIMARY KEY,
    product_id uuid NOT NULL REFERENCES shavandi.products(id) ON DELETE RESTRICT,
    asset_ref text NOT NULL CHECK (asset_ref LIKE '/images/%'),
    sort_order integer NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    alt_text text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX product_images_by_product
    ON shavandi.product_images (product_id, sort_order, id);

CREATE VIEW shavandi.effective_active_categories AS
WITH RECURSIVE active_tree(id) AS (
    SELECT id
    FROM shavandi.categories
    WHERE parent_id IS NULL AND active

    UNION

    SELECT child.id
    FROM shavandi.categories AS child
    JOIN active_tree AS parent ON parent.id = child.parent_id
    WHERE child.active
)
SELECT category.*
FROM shavandi.categories AS category
JOIN active_tree USING (id);

-- +goose Down
DROP VIEW shavandi.effective_active_categories;
DROP TABLE shavandi.product_images;
DROP TABLE shavandi.variants;
DROP TABLE shavandi.product_slugs;
DROP TABLE shavandi.products;
DROP TABLE shavandi.categories;
