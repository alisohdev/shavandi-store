# M2 authentication handoff

M2 adds customer SMS authentication and a separate administrator password plus SMS flow. The public catalog stays read-only. Storefront products, cart, checkout, and the admin editor retain their local fixtures and preview behavior.

The API owns authentication. PostgreSQL stores users, OTP challenges, request limits, session verifiers, and audit events. The browser holds the session in an HttpOnly cookie and sends same-origin requests. It does not store authentication secrets in localStorage.

See [the backend authentication reference](m2-backend.md) for endpoint contracts, provisioning, configuration, security boundaries, and verification details.

## Local development

Run the Go API on `127.0.0.1:8080` and Astro on its usual local port. Astro proxies `/api` to the API while preserving the request Origin. Configure the API's allowed origin to match the Astro origin exactly. The API reads process environment variables; it does not load `.env` automatically.

The static production build requires a same-origin reverse proxy for `/api/v1`. Serve production over HTTPS and configure production cookie settings. The static admin page is a UI gate; every privileged API operation must use the server's administrator middleware.

## SMS dependency

No real SMS provider has been selected or integrated. The shipped API cannot complete SMS login until a real sender is implemented and configured. Missing delivery fails safely. Automated tests inject a deterministic sender; the production entry point exposes no test sender, fixed code, or debug-code endpoint.

## Verification command

From the repository root, run:

```powershell
.\backend\scripts\verify-m2.ps1 -BunExe 'C:\path\to\supported\bun.exe'
```

The command loads the ignored backend environment into its process, generates sqlc queries, runs Go tests and real PostgreSQL integration tests, runs vet and build, validates Astro and Bun, and runs the browser suite. It uses no Docker. Browser UI tests use controlled API responses; PostgreSQL integration tests exercise the real authentication service and HTTP boundary.

If Playwright Chromium is unavailable and Chrome is installed, add `-PlaywrightChannel chrome`. This session used installed Chrome after the Chromium download returned HTTP 403.

## Verified results

On 2026-11-04, the complete `verify-m2.ps1` command exited successfully:

- sqlc generation, `go test ./...`, `go test -tags=integration ./... -count=1`, `go vet ./...`, and `go build ./...` passed.
- Both configured PostgreSQL databases were accessible. Goose applied the development migration at version `20261104120000`.
- Astro checked 38 files with zero errors, warnings, or hints. Lint passed 19 files; formatting and Astro formatting passed.
- Bun passed 23 tests with 397 assertions. The static build produced 31 pages.
- Chrome passed all nine browser tests, including mobile RTL at 360px and desktop layout. These tests use mocked API responses.
- Real PostgreSQL tests covered first and returning login, incorrect/expired/reused OTPs, concurrent successful and failed verification, request limits, resend controls, rotation, fixation, expiration, logout, account isolation, admin MFA and provisioning races, CSRF/Origin, and unavailable SMS.
- Existing public catalog integration tests and fixture regression tests passed. No catalog mutations or commerce endpoints were exposed.

The race detector could not run because this Windows environment has CGO disabled and no gcc. Real concurrent PostgreSQL tests passed. No actual business administrator was provisioned: Sajad's phone and password must be supplied locally through `adminctl`.
