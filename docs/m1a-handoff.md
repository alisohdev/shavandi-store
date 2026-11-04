# M1a catalog handoff

M1a adds the first persistent commerce domain while leaving the Astro storefront and its Persian RTL preview editor static. The Go service exposes public catalog reads. Product and category writes exist only behind the internal catalog service and PostgreSQL repository; there are no HTTP mutation routes.

## PostgreSQL schema

The Goose migration `20261103170000_create_catalog.sql` adds these objects under `shavandi`:

| Object                        | Purpose and key constraints                                                                                                                                                                                                                                                      |
| ----------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `categories`                  | UUID identity, Unicode name, lowercase hyphenated unique slug, optional restrictive parent FK, nonnegative display order, active flag, optimistic version, timestamps, and no direct self-parent. The service prevents ancestor cycles.                                          |
| `products`                    | Stable UUID, restrictive primary-category FK, name and description, integer nonnegative toman regular price, optional sale price below regular, `draft`/`published`/`hidden` state, version, and timestamps.                                                                     |
| `product_slugs`               | Globally unique permanent product slug history. A partial unique index allows one current slug per product. A product may restore one of its own old slugs; another product cannot claim it.                                                                                     |
| `variants`                    | Stable UUID and immutable unique SKU, optional custom size/color, optional nonnegative price overrides, active flag, and timestamps. A partial unique index prevents duplicate active `(size, color)` combinations, treating two missing values as the same default combination. |
| `product_images`              | Stable UUID, product FK, local `/images/…` asset reference, nonnegative sort order, and optional alt text. M1a stores references only; it does not upload or delete files.                                                                                                       |
| `effective_active_categories` | Recursive view containing active categories whose full ancestor path is active. Public category and product reads use this view.                                                                                                                                                 |

Product saves and category edits take the same PostgreSQL advisory transaction lock. Product saves recheck that the primary category is in the effective active tree while holding it, so category deactivation cannot race a publication or edit. Product updates compare the expected product version; category updates compare their expected category version. Aggregate writes, permanent slug changes, variant updates, and image metadata replacement commit or roll back together. Existing variant SKUs cannot be changed. A variant slice on `EditProduct` is a set of child changes keyed by variant ID: omitted variants remain unchanged, `active=false` explicitly retires one, and an empty slice is rejected. New optionless products receive a default variant at creation. Image behavior differs: omitted images are preserved, while an explicit image list replaces the product's image metadata. Removed image rows do not delete local files.

New products are drafts. Creation with no supplied variants creates one active default variant with no size/color and a generated SKU. Products with sizes/colors accept arbitrary preset or custom values. The service generates UUIDs where callers omit them. Product identity and each persisted variant identity remain stable during edits.

## Domain and pricing rules

`internal/catalog` owns validation, price resolution, publication transitions, and the narrow public `Reader` interface. `internal/postgres` owns SQL transactions and maps database constraints to catalog errors. `httpapi` receives a `catalog.Reader`, so it cannot call mutation methods.

Creating a product always creates a draft. Editing preserves the current publication state, regardless of a submitted state field. Drafts may be edited and published; published products may be hidden; hidden products may be republished after validation. Published products require a name, slug, primary category, valid prices, an active variant, a nonblank description, and at least one image. A public lookup returns the same 404 for a missing, draft, or hidden product.

Prices are integer toman. Effective prices follow this order:

1. The variant's regular-price override, or else the product regular price.
2. The variant sale-price override, when present.
3. Otherwise, the product sale price only when that variant has no regular-price override.
4. Otherwise, no sale price.

Examples:

| Product                      | Variant                                        | Effective price                |
| ---------------------------- | ---------------------------------------------- | ------------------------------ |
| Regular 100,000; sale 85,000 | No overrides                                   | 100,000 regular / 85,000 sale  |
| Regular 100,000; sale 85,000 | Regular override 120,000                       | 120,000 regular / no sale      |
| Regular 100,000; sale 85,000 | Regular override 120,000; variant sale 105,000 | 120,000 regular / 105,000 sale |
| Regular 100,000; sale 85,000 | Variant sale 90,000 only                       | 100,000 regular / 90,000 sale  |

Every effective sale must be nonnegative and below its effective regular price. No inventory or availability is inferred from variant activity.

## Public API

All routes are read-only. Query work is bounded by a three-second request context. Product pages use keyset pagination ordered by `(created_at DESC, id DESC)`. `limit` defaults to 20 and accepts 1–50. `category` filters by the category slug; `cursor` is an opaque URL-safe continuation token.

### `GET /api/v1/categories`

Returns active categories in a flat list with `parent_id` for tree reconstruction:

```json
{
  "categories": [
    {
      "id": "…",
      "name": "Football Jerseys",
      "slug": "football-jerseys",
      "parent_id": null,
      "display_order": 0
    }
  ]
}
```

### `GET /api/v1/products?category=football-jerseys&limit=20`

Returns only published products whose primary category and complete ancestor path are active:

```json
{
  "products": [
    {
      "id": "…",
      "slug": "blue-shirt",
      "name": "Blue Shirt",
      "description": "A football shirt.",
      "category_id": "…",
      "regular_price_toman": 100000,
      "sale_price_toman": 85000,
      "created_at": "2026-11-03T12:00:00Z",
      "updated_at": "2026-11-03T12:00:00Z",
      "variants": [
        {
          "id": "…",
          "size": "XL Tall",
          "color": "Persian Blue",
          "price": { "regular_toman": 100000, "sale_toman": 85000 }
        }
      ],
      "images": [
        {
          "id": "…",
          "asset_ref": "/images/products/blue-shirt.webp",
          "sort_order": 0,
          "alt_text": "Blue shirt"
        }
      ]
    }
  ],
  "next_cursor": "…"
}
```

`next_cursor` is omitted on the last page. The API does not return internal SKUs, draft/hidden state, stock, or order data.

### `GET /api/v1/products/blue-shirt`

Returns the same product object inside `{ "product": … }`. Unknown, draft, and hidden slugs return 404.

Errors use one envelope with a request ID:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "The catalog request is invalid",
    "request_id": "…"
  }
}
```

## Static frontend mapping

The existing Astro fixture remains the rendering source; no page or editor now calls the API. The generic catalog fields map as follows:

| Astro fixture                    | M1a API/domain                                                                                                      |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `Product.id`                     | Stable product UUID `id`                                                                                            |
| `Product.slug`                   | Current product slug `slug`                                                                                         |
| `Product.name`, `description`    | Same-named product fields                                                                                           |
| Persian `Product.category` label | A category record, commonly its Persian `name`; use its UUID as `primary_category_id` and stable slug for filtering |
| `Product.priceToman`             | Product `regular_price_toman`; optional product sale price is separate                                              |
| A colorway's size/color options  | Flatten to product variants with custom `size` and `color` strings                                                  |
| Variant SKU                      | Internal variant `sku`; intentionally absent from public JSON                                                       |
| `ProductImage.src`, `alt`        | `asset_ref`, `alt_text`; sort order is explicit                                                                     |

Several fixture fields are presentation or preview policy and do not map to M1a: colorway swatches and notes, image dimensions/fit/position/srcset, material, fit, care, product tags, provenance, and demo `availability`. A colorway groups multiple fixture sizes and their images; the M1a product image list is product-level and the variant record is a size/color combination. The API does not model inventory, so demo `available`/`sold-out` values must not be translated into variant activity. Editorial collections (`slug`, `productIds`, copy, and collection image) are not categories and stay in frontend data. This mapping documents a future adapter; it is not a drop-in replacement for the richer fixture.

## Verification and remaining scope

The dedicated `TEST_DATABASE_URL` integration suite uses the permanent local PostgreSQL service, requires a clean database ending in `_test`, applies Goose migrations, and rolls back only the schema/version table it created. It covers product/category creation and edits, default/custom variants, explicit variant retirement, pricing, slug reservations, duplicate combinations, failed transaction rollback, stale concurrent updates, publication visibility, active category subtrees, and pagination. It also sends HTTP requests through the production handler and PostgreSQL-backed reader for category, detail, draft/hidden visibility, and paginated product routes. No Docker dependency is used.

Checks completed:

- `sqlc generate` with sqlc 1.31.1: passed.
- `go test ./...`: passed.
- `go test -tags=integration ./... -count=1` against the real `shavandi_test` database: passed; migration and catalog tests cleaned up their test schema.
- `go vet ./...`: passed.
- `go build ./...`: passed.
- `bun run validate` with Bun 1.4.4-canary.1, revision `271b65ec6`: passed; all 16 frontend tests passed and Astro built 30 pages.
- Goose on the development database: applied, status clean, version `20261103170000`.

Files changed:

- Schema and generated database access: `backend/db/migrations/20261103170000_create_catalog.sql`, `backend/db/queries/catalog.sql`, `backend/internal/dbgen/catalog.sql.go`, `backend/internal/dbgen/models.go`.
- Domain and persistence: `backend/internal/catalog/catalog.go`, `backend/internal/catalog/catalog_test.go`, `backend/internal/postgres/catalog_repository.go`, `backend/internal/postgres/catalog_reader.go`, `backend/internal/postgres/catalog_integration_test.go`, `backend/internal/postgres/migration_integration_test.go`.
- API and wiring: `backend/internal/httpapi/router.go`, `backend/internal/httpapi/router_test.go`, `backend/internal/httpapi/catalog_test.go`, `backend/cmd/api/main.go`.
- Verification and docs: `backend/scripts/verify-m1a.ps1`, `backend/README.md`, `README.md`, `docs/architecture.md`, and this handoff.

M1a does not add authentication, public mutation routes, frontend API integration, image upload, stock, reservations, checkout, or orders. The dev database receives the catalog schema but no seeded products; the existing frontend fixtures remain unchanged.
