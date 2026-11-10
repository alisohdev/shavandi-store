# Inventory phase

Variants now have stock_on_hand (0..1,000,000) and a separate stock_version. Migration 20261110130000 starts every existing variant at zero; new variants also start at zero. Enter actual counts through the authenticated admin screen before expecting cart additions to succeed. Publication is allowed at zero stock, but the product option is unavailable for saving to a cart.

PATCH /api/v1/admin/products/{productID}/variants/{variantID}/stock accepts {version, quantity, reason}. The version is the variant stock version, not the product version. MFA administrator authorization, Origin/CSRF checks and bounded JSON apply. The server takes the actor from the session and confirms the variant belongs to the product. Stock count and an append-only application audit row (actor, previous/new quantity, reason and new version) commit together. A stale version returns 409; reload before retrying. No audit deletion/edit API is provided.

Public variant responses include available_quantity. Cart writes reject quantities greater than current stock. Existing cart lines that exceed a reduced count are excluded from the subtotal until the customer lowers the quantity or removes the line. Available counts on hidden/inactive products or variants are zero in the cart response. Cart quantities remain limited to nine. Stock counts are not reserved by carts, so multiple customers may hold the same item; future order placement must atomically reserve/deduct stock and revalidate prices.

Ordinary catalog writes preserve stock counts and versions. Stock writes do not change the product version. PostgreSQL verification covers zero defaults, concurrent stale-version rejection, audit entries, public stock, cart quantity checks, stock reduction and ordinary product edits preserving inventory. HTTP tests cover MFA, CSRF and rejection of caller-supplied actor identity. Browser tests cover admin count entry and the live cart flow using API fixtures.

Apply the new migration through Goose before running this API build. Automated integration tests use only the dedicated _test database; the working database is unchanged. Orders, reservations, shipping and payment remain inactive.
