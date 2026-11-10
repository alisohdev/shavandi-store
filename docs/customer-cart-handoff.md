# Customer cart phase

Signed-in customers can save a published variant with a quantity from 1 through 9, view their persisted cart, change quantities and remove lines. The header now links to /cart/. Legacy fixture cart storage and the simulated /checkout/ flow are separate and are not imported into the account cart.

API: GET /api/v1/cart returns lines, subtotal_toman and has_unavailable. POST /api/v1/cart/lines/{variantID} accepts only {"quantity": 0..9}; zero removes the line, and other values set an absolute quantity. Repeating a write does not add twice. The account comes only from the authenticated session. Writes require the existing exact Origin and signed CSRF checks. Responses use no-store; user input cannot supply account IDs or prices.

Migration 20261110120000_create_cart.sql creates cart_lines with user/variant foreign keys, a per-account primary key and quantity checks. Per-account writes lock the user row to serialize the 50-line cap. Quotes read current product/variant pricing in one database query. Hidden products, inactive variants or inactive categories leave removable unavailable lines with null prices and exclude them from the subtotal. Values remain integer toman and totals are bounded for exact browser arithmetic. Product names and options remain visible to the owner for removal. Concurrent tabs use last-write-wins absolute quantities; no cart version conflict protocol is introduced.

Apply the existing Goose migration workflow to the development/production database before running this API version. Verification applies migrations only to the isolated _test database; no development migration is performed by the tests. Existing sessions and cart lines survive process restarts. GET recomputes prices; saved carts do not reserve inventory or lock prices.

PostgreSQL tests cover persistence, account isolation, concurrent repeat writes, repricing, visibility, unavailable-item removal, invalid quantity/UUID and the line limit. HTTP tests cover session-derived ownership, authentication, Origin/CSRF and rejecting extra user/price fields. Browser tests cover variant selection, save, quantity change, reload, removal, guest login and unavailable lines. Browser cart responses are mocked; live browser-to-auth-to-PostgreSQL verification still requires a provisioned account.

Shipping, inventory reservations, order creation and payment remain inactive. The next commerce phase needs inventory and order rules before accepting purchases.
