# Zarinpal payments

The normal API can initiate and verify Zarinpal v4 payments. Payments remain disabled by default. No real gateway call, merchant setup, production migration or charge was performed while implementing this phase. HTTP provider tests use a transport double; database tests use an injected gateway; browser tests intercept the gateway URL.

## Configuration

Set backend-only PAYMENT_PROVIDER=zarinpal, ZARINPAL_MERCHANT_ID to this store's merchant UUID, ZARINPAL_ENV to sandbox or production, and PAYMENT_PUBLIC_ORIGIN to the exact storefront origin. Sandbox requires APP_ENV=development and its authorities begin with S; production authorities begin with A. HTTPS is required except a loopback sandbox origin. No arbitrary API endpoint or callback/return URL is accepted from a browser. Merchant fingerprints and environment are persisted with attempts; changing merchant or environment cannot verify an old attempt under different credentials. Complete/reconcile outstanding attempts before changing the configuration. Order history shows sandbox payments as test payments even after configuration changes; use a separate nonproduction catalog/database for sandbox testing.

Apply migration 20261110170000 through Goose before starting the new API. Its downgrade refuses to discard paid/payment-review order states. The dedicated database test removes its own payment fixtures before exercising migration rollback; this does not relax production accounting protection. Keep the reverse proxy routing /api/* to Go, including the callback. The callback URL is PAYMENT_PUBLIC_ORIGIN/api/v1/payments/zarinpal/callback, and its return destination is the configured origin plus /orders/.

## Initiation and review

POST /api/v1/orders/{id}/payment accepts delivery_version and expected_total_toman. The session owner, active reservation, persisted address and an explicitly selected shipping method are required. The server calculates merchandise plus the saved shipping fee and compares it with the reviewed amount. It converts integer toman to integer rial once (multiply by ten) and sends currency IRR. Zero-value orders cannot use this gateway path. An opaque order ID is included as gateway metadata for reconciliation; recipient addresses, card data and OTPs are not sent.

One payment attempt per order is durably created before the external request. A per-order uniqueness constraint prevents concurrent initiation from issuing another gateway request. Ready-attempt retries reuse the same authority. No network call occurs while a database transaction lock is held. Delivery details freeze when any attempt exists; physical inventory counts cannot be reduced below existing reservations. While an attempt is requesting/ready/unknown, customer cancellation is blocked; the original reservation expiry still applies.

If request delivery is ambiguous, the attempt is unknown (or remains requesting if the database update also failed). The app does not retry the gateway request. Consult the merchant dashboard using the order metadata and request time/amount before any recovery; there is no automatic unknown-authority reconciliation or replacement-attempt endpoint. A definitively rejected request is failed; cancel that reservation and create a new order after fixing the cause. Cancellation/recreation is explicit, not an automatic charge retry.

## Verification and stock

The public GET callback is intentionally outside session/CSRF middleware so it works on return from the gateway. Status=OK only triggers server-to-server verification of the saved authority and amount; it cannot itself mark an order paid. Unknown authorities are ignored, and Status=NOK does not change accounting state. The callback redirects to the fixed orders page with no success claim. Owners can POST {} to /api/v1/orders/{id}/payment/verify to recheck a known authority if the browser return was lost or verification temporarily failed.

Gateway verification codes 100 and 101 require a positive reference ID. Card PAN/hash/provider messages are neither stored nor exposed. Attempts use a renewable verification lease with a timeout to prevent concurrent external verification. Successful settlement and the reference ID are persisted in one transaction; the reference is unique within merchant/environment. Replays do not deduct stock twice. If the reservation is still pending and valid, settlement deducts both physical and reserved quantities once and marks paid. A verified payment after expiry/cancellation becomes payment_review, with no automatic fulfillment or stock deduction. Operators must investigate/refund these records in the merchant system; no refund API or fulfillment workflow is implemented here.

If gateway verification succeeds but the database commit fails, the authority remains retryable after its lease expires. A later 101 response with its reference can complete the same local transaction. Missing/invalid references, provider errors and unrecognized states never mark an order paid. A database error while recording a payment is not treated as a declined payment.

## Operational limits and verification

There is no automatic gateway reconciliation job, admin payment-review queue or refund action yet. Lost callbacks require the owner's recheck or merchant-panel reconciliation; outstanding ambiguous attempts and payment_review orders need operator handling before production launch. Reservation expiry is not extended when payment starts. This favors preserving stock correctness over automatically fulfilling late payments.

Tests cover reviewed totals, toman/rial conversion, provider contract and 100/101 handling, sandbox/HTTPS configuration gates, session ownership, callback forgery, duplicate initiation/verification, uncertain request handling, frozen delivery, cancellation guards, atomic rollback/recovery, stock deduction and late-payment review. Browser tests cover the allowlisted gateway redirect, frozen details, explicit verification after return, reference display, review-state messaging and rejection of off-gateway redirects. The complete local checks do not prove live merchant connectivity or real Zarinpal sandbox behavior.

## Provider sources

The implementation follows Zarinpal's [connection/verification guide](https://www.zarinpal.com/docs/paymentGateway/connectToGateway) and [sandbox guide](https://www.zarinpal.com/docs/paymentGateway/sandBox). The [inquiry guide](https://www.zarinpal.com/docs/paymentGateway/otherMethods/Inquiry) explicitly distinguishes inquiry from payment verification; inquiry is not used as settlement evidence.
