-- name: ListActiveCategories :many
SELECT id, name, slug, parent_id, display_order, active, version, created_at, updated_at
FROM shavandi.effective_active_categories
ORDER BY parent_id NULLS FIRST, display_order, id;

-- name: ListPublishedProducts :many
SELECT
    product.id,
    current_slug.slug,
    product.name,
    product.description,
    product.primary_category_id,
    product.regular_price_toman,
    product.sale_price_toman,
    product.created_at,
    product.updated_at
FROM shavandi.products AS product
JOIN shavandi.product_slugs AS current_slug
  ON current_slug.product_id = product.id AND current_slug.is_current
JOIN shavandi.effective_active_categories AS category
  ON category.id = product.primary_category_id
WHERE product.publication_state = 'published'
  AND (sqlc.narg(category_slug)::text IS NULL OR category.slug = sqlc.narg(category_slug)::text)
  AND (
      sqlc.narg(after_created_at)::timestamptz IS NULL
      OR (product.created_at, product.id) < (
          sqlc.narg(after_created_at)::timestamptz,
          sqlc.narg(after_id)::uuid
      )
  )
ORDER BY product.created_at DESC, product.id DESC
LIMIT sqlc.arg(page_limit)::integer;

-- name: GetPublishedProductBySlug :one
SELECT
    product.id,
    current_slug.slug,
    product.name,
    product.description,
    product.primary_category_id,
    product.regular_price_toman,
    product.sale_price_toman,
    product.created_at,
    product.updated_at
FROM shavandi.products AS product
JOIN shavandi.product_slugs AS current_slug
  ON current_slug.product_id = product.id AND current_slug.is_current
JOIN shavandi.effective_active_categories AS category
  ON category.id = product.primary_category_id
WHERE product.publication_state = 'published'
  AND current_slug.slug = sqlc.arg(slug)::text;

-- name: ListPublicVariants :many
SELECT id, product_id, size, color, regular_price_override_toman, sale_price_override_toman
FROM shavandi.variants
WHERE product_id = ANY(sqlc.arg(product_ids)::uuid[])
  AND active
ORDER BY product_id, size NULLS FIRST, color NULLS FIRST, id;

-- name: ListPublicImages :many
SELECT id, product_id, asset_ref, sort_order, alt_text
FROM shavandi.product_images
WHERE product_id = ANY(sqlc.arg(product_ids)::uuid[])
ORDER BY product_id, sort_order, id;
