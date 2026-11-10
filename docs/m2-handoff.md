# M2 authentication audit and handoff

Audit date: 2026-11-04. Audited HEAD: `a113148`. This report uses the original M2 requirements in the conversation, current source, Git history, and fresh checks. No implementation was restarted or changed. No commits, pushes, provisioning, development migrations, Docker, or M1b work were performed during this audit.

## 1. Implementation summary and actual state

Customer SMS OTP, server-managed sessions, singleton administrator provisioning, admin password plus SMS MFA, HTTP security guards, and Persian RTL login screens are implemented. The production entry point intentionally constructs only `UnavailableSMS`; new login cannot complete until a real sender is implemented.

At entry there were no tracked unstaged changes or staged files. Only `.ignore` and `decisions.tsv` were untracked; both were preserved. `backend/.env` is ignored and untracked. SQLC regeneration produced no implementation diff. This audit changes only this report and ignored test/build artifacts.

| Existing commit | Milestone                                              |
| --------------- | ------------------------------------------------------ |
| `cb1cb6e`       | Existing M0 foundation and M1a catalog checkpoint.     |
| `40872fd`       | Customer login/admin entry frontend and browser tests. |
| `2992023`       | Authentication backend, schema, CLI and tests.         |
| `44ff5c8`       | Initial documentation.                                 |
| `a113148`       | Existing demo-border design-detector exception.        |

The M2 range `cb1cb6e..a113148` contains 42 changed files, 4,648 insertions and 20 deletions, already committed. The detector exception is ancillary tooling authorized by the later hook-triage request. No other unrelated tracked change was identified.

## 2. Files and modules changed

| Area           | Files and responsibilities                                                                                                                                 |
| -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| SQL            | `backend/db/migrations/20261104120000_create_auth.sql`, `db/queries/auth.sql`, generated `internal/dbgen/auth.sql.go` and `models.go`.                     |
| Domain         | `backend/internal/auth/{auth,service,password,phone}.go`, `auth_test.go`: crypto, normalization, policy, SMS/store boundaries.                             |
| Persistence    | `backend/internal/postgres/auth_store.go`, `auth_integration_test.go`, migration test version constant.                                                    |
| HTTP           | `backend/internal/httpapi/auth.go`, `auth_test.go`, router option wiring.                                                                                  |
| Startup/config | `backend/internal/config/auth.go`, `auth_test.go`, `cmd/api/main.go`, `.env.example`, Go module files.                                                     |
| Provisioning   | `backend/cmd/adminctl/main.go`, `main_test.go`.                                                                                                            |
| Frontend       | `src/components/{AuthAccess,AuthForm}.tsx`, `src/lib/auth.ts`, `auth.test.ts`, login/admin pages, header, `auth.css`, small header rules in `preview.css`. |
| Verification   | `backend/scripts/verify-m2.ps1`, Playwright config, `tests/browser/auth.pw.ts`, Bun manifest/lockfile.                                                     |
| Other          | Astro development proxy and toolbar configuration, backend README, M2 docs, `.impeccable/config.json`.                                                     |

Source diff confirms `src/data` fixtures and `AdminEditor.tsx` are unchanged. The existing editor mounts after the admin session gate. Products, cart, checkout and orders have no new backend connections.

## 3. Database migrations

Goose migration `20261104120000_create_auth.sql` adds six tables:

- `users`: UUID, unique normalized phone, customer/admin role, active status, timestamps, password constraints and singleton-admin partial index.
- `auth_otp_targets`: phone/purpose resend state and shared serialization row.
- `auth_otp_challenges`: keyed challenge/code verifiers, purpose, admin reference, delivery, attempts, expiration and consumption.
- `auth_sessions`: keyed bearer verifier, user, assurance method, timestamps and revocation.
- `auth_rate_limits`: keyed buckets, fixed windows and durable counters.
- `auth_audit_events`: event, optional user reference, keyed subject/IP fingerprints and timestamp.

Fresh integration tests applied all migrations, checked repeat application/version tracking, exercised authentication/catalog behavior, and cleaned their test-owned schema/version table. The helper enforces a separate clean `_test` database and takes a suite advisory lock.

Read-only development Goose `status` and `version` passed in this audit. All three migrations are applied; version is `20261104120000`. This audit did not run development `up` or `down`.

## 4. Authentication API contracts

All POSTs require a JSON object bounded to 4 KiB, exact allowed Origin, and signed CSRF cookie plus matching `X-CSRF-Token`. Unknown fields/trailing JSON are rejected. Fetch `/csrf` first and keep its token in memory.

| Method/path under `/api/v1/auth` | Request                | Success                                                                                                                   |
| -------------------------------- | ---------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `GET /csrf`                      | None                   | `200 {csrf_token}` and HttpOnly CSRF cookie.                                                                              |
| `POST /customer/otp/request`     | `{phone}`              | Successful delivery: `202 {challenge_id, expires_in:300, resend_after:60}`. Default sender returns `503 sms_unavailable`. |
| `POST /customer/otp/verify`      | `{challenge_id, code}` | `200` session JSON and fresh cookie; first verification creates customer.                                                 |
| `POST /admin/login`              | `{phone, password}`    | Correct privileged password plus delivery produces an admin challenge; no session yet.                                    |
| `POST /admin/otp/verify`         | `{challenge_id, code}` | `200` privileged session after SMS verification.                                                                          |
| `GET /session`                   | Cookie                 | `200` identity or `401 authentication_required`.                                                                          |
| `GET /admin/session`             | Cookie                 | Requires persisted admin role plus password/SMS assurance; customer gets `403`.                                           |
| `POST /logout`                   | `{}`                   | `204`, revokes presented session and clears cookie.                                                                       |

Session JSON contains `user:{id,phone,role}` and RFC3339 `expires_at`. Tokens, hashes, codes and assurance fields are excluded. Errors retain `{error:{code,message,request_id}}`; bad/expired/reused OTPs share `401 invalid_otp`, throttling returns `429` plus `Retry-After`, and Origin/CSRF failures return `403`.

Phones normalize supported `09...`, `9...`, `+98...`, `0098...`, Persian/Arabic digits and separators into `+989xxxxxxxxx`. Resend repeats the request endpoint; admin resend repeats password verification.

Public category/product GET contracts and handlers remain unchanged. Existing catalog HTTP/PostgreSQL tests passed. No catalog mutation routes exist.

## 5. Admin and session security model

Only trusted local `adminctl --phone <Sajad-mobile> --password-stdin` provisions an administrator. No HTTP role-assignment endpoint exists. The singleton index prevents a second admin phone. Reprovisioning the same phone resets its password, revokes sessions and invalidates challenges transactionally. Actual Sajad provisioning was not performed; credentials were not supplied.

Passwords use Argon2id with 64 MiB, three passes, one lane, random 16-byte salt and 32-byte output. Inputs require at least 12 characters and at most 1,024 bytes. Web hashing concurrency is bounded to two per service. Unknown/non-admin credentials use a dummy hash. Authorization checks persisted role and completed MFA, never submitted phone alone.

Challenge/session tokens contain 32 random bytes. PostgreSQL stores domain-separated HMAC-SHA-256 verifiers, including keyed OTP verifiers. Codes use crypto/rand, last five minutes and allow five attempts. Shared target/challenge locks serialize resend, verification and provisioning. Consumption, account creation, issuance, rotation and success audit commit atomically; denial counters also commit.

Cooldown is 60 seconds. OTP requests allow five per phone and 20 per IP per hour; admin password attempts five per phone and 30 per IP per 15 minutes. Verification has challenge/IP budgets and ten attempts per phone per 15 minutes. Forwarding headers are ignored; IP comes from RemoteAddr.

Customer sessions last 12 hours, admin one hour. Authentication rotates tokens and revokes the presented old token. Every identity read checks database expiry/revocation/active status. Production cookies use `__Host-`, Secure, HttpOnly, SameSite=Strict, Path=/ and no Domain. Explicit development mode permits separate HTTP cookie names; production rejects insecure cookies and non-HTTPS/wildcard origins.

`RequireAuthentication`, `RequireAdmin` and `SessionFromContext` are reusable authorization helpers. Current auth routes also apply Origin/CSRF/no-store guards. The static admin gate is presentation only and does not protect downloadable public fixture data.

## 6. Fresh verification commands and actual results

The existing verification command exited zero during this audit:

```powershell
.\backend\scripts\verify-m2.ps1 -BunExe C:/Users/soheili/AppData/Local/Temp/shavandi-bun-canary/bun-windows-x64/bun.exe -PlaywrightChannel chrome
```

It loads ignored environment settings into its child process without printing credentials. Results below are fresh, not copied from the previous handoff.

| Check                                                       | Status     | Result                                                                                                                                   |
| ----------------------------------------------------------- | ---------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate` | PASS       | Succeeded; no generated implementation diff.                                                                                             |
| `go test ./...`                                             | PASS       | All packages passed; initial invocation used cache.                                                                                      |
| `go test ./... -count=1`                                    | PASS       | Separate uncached rerun passed all packages.                                                                                             |
| `go test -tags=integration ./... -count=1`                  | PASS       | All packages passed; PostgreSQL package 6.316 seconds, including auth, HTTP, concurrency, provisioning, migration and catalog tests.     |
| `go vet ./...`                                              | PASS       | Succeeded.                                                                                                                               |
| `go build ./...`                                            | PASS       | Succeeded.                                                                                                                               |
| Development Goose `status` and `version`                    | PASS       | Database reachable; version `20261104120000`, all migrations applied. Test DB access established by integration execution.               |
| `bun run validate`                                          | PASS       | Astro: 38 files, zero errors/warnings/hints; lint: 19 files; formatting: 38 files plus Astro; 23 tests, 397 assertions; build: 31 pages. |
| `bun run test:e2e`, Chrome                                  | PASS       | Nine browser cases, 15.3 seconds. RTL at 360px, desktop, resend/errors, session/logout and admin gate. All auth API responses mocked.    |
| `go test -race ./internal/auth`                             | BLOCKED    | Actual command failed: `-race requires cgo`; host `CGO_ENABLED=0`. No host changes made.                                                 |
| Vulnerability scan                                          | NOT TESTED | Not rerun here. Previous report records HTTP 403; historical evidence is not a current scan verdict.                                     |
| Real browser through Go/PostgreSQL                          | NOT TESTED | Browser suite intercepts auth requests; real backend HTTP tests use httptest separately.                                                 |
| Real SMS/deployed HTTPS flow                                | BLOCKED    | No selected provider/adapter or deployment supplied.                                                                                     |

The capturing SMS implementation is confined to integration-tagged Go test code. Fixed example browser codes are confined to its explicit API mock. Production has no fake-sender configuration switch.

## 7. Security review findings

No demonstrated current authentication bypass or privilege escalation was found in the reviewed source/tests. This scoped review is not a penetration-test certification. No implementation fixes were made.

| Finding                                 | Assessment                                                                                                                                                                                                                                     |
| --------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Browser/backend coverage stops at mocks | Verification gap: browser cookie enforcement, proxy Origin behavior and Go/PostgreSQL login are not exercised together.                                                                                                                        |
| Two-customer isolation                  | Query joins identity through the cookie verifier, and tests verify expected customers/admin denial. No explicit two-customer cross-account identity substitution test exists.                                                                  |
| Theft/fixation coverage                 | Rotation and old/revoked-cookie rejection pass. Dedicated forged-cookie/copied-live-cookie/fixation attack scenarios are absent. A stolen still-valid bearer cookie remains usable until revocation/expiry; HttpOnly does not eliminate theft. |
| Future mutation middleware composition  | Exported auth helpers do not apply CSRF/Origin/no-store themselves. Those guards are private to the auth subtree. Future mutation routes need equivalent protection explicitly; no current unprotected mutation was found.                     |
| Logout audit attribution                | Logout writes empty subject/user fields, preventing direct correlation to the revoked identity. Login/admin provisioning events do include user identity. Improve traceability in later authorized work.                                       |
| Retention/proxy deployment              | No pruning policy for auth records. Behind a reverse proxy, RemoteAddr can group all users into one IP budget. Define retention and trusted-proxy behavior before rollout.                                                                     |
| Provisioning inputs                     | Secret length is validated, not randomness; password length is validated, not common-password strength. Operators must choose a random key and strong password.                                                                                |

Singleton/null-hash constraints, consistent customer issuance and password-reset race handling are present and covered by passing tests. No new blanket verification is claimed for historical review statements.

## 8. Requirements, limitations and remaining work

PASS means inspected implementation with relevant passing evidence; FAIL means contradicted behavior/failing required check; BLOCKED means missing prerequisite; NOT TESTED means evidence does not prove that scenario. No executed required verification command failed.

| Original requirement                                                 | Status     | Evidence or limit                                                                                                         |
| -------------------------------------------------------------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------- |
| Review/checkpoint M0/M1a and preserve unrelated changes              | PASS       | History and clean tracked baseline inspected; no audit commits.                                                           |
| Both PostgreSQL databases accessible                                 | PASS       | Development Goose reads and real integration.                                                                             |
| Preserve fixtures/design                                             | PASS       | Fixtures/editor unchanged; responsive auth and existing regressions pass. Whole-store visual pixel comparison NOT TESTED. |
| Versioned users/status/privileges/session/OTP/audit schema, sqlc/pgx | PASS       | Migration/source and real DB tests.                                                                                       |
| No raw OTP/password/session secret in PostgreSQL                     | PASS       | Verifier schema, hashing and storage assertions.                                                                          |
| First customer login and automatic account creation                  | PASS       | Test verifies no account before verification and customer afterwards.                                                     |
| Returning customer login                                             | PASS       | Same phone verifies again during rotation test; explicit stable-ID equality assertion absent.                             |
| Iranian phone normalization                                          | PASS       | Backend/frontend input tests.                                                                                             |
| Cryptographic, short-lived, single-use, atomic OTP                   | PASS       | crypto/rand, locks, expiry/replay/concurrency tests.                                                                      |
| Request/verification limits, cooldown, bounded attempts              | PASS       | Persistent counters and concurrent failure/resend tests.                                                                  |
| No customer-issuance account enumeration                             | PASS       | Same path for unknown/admin/disabled numbers and generic verification denial.                                             |
| No routine secret disclosure in logs/responses/storage               | PASS       | Safe projections, no body logging, redaction tests; exhaustive panic/provider-log fault injection NOT TESTED.             |
| Small SMS interface and deterministic test sender                    | PASS       | Test-only injected capturing implementation.                                                                              |
| Production bypass excluded; absent sender fails safely               | PASS       | UnavailableSMS production wiring and failure-consumption tests.                                                           |
| Real SMS delivery                                                    | BLOCKED    | Provider deliberately deferred by original M2 scope.                                                                      |
| Trusted singleton admin provisioning; no self-promotion              | PASS       | CLI, unique index, no HTTP role assignment.                                                                               |
| Actual Sajad account provisioned                                     | BLOCKED    | Credentials not supplied; no real account changed.                                                                        |
| Separate password plus OTP, Argon2id, admin authorization            | PASS       | MFA/purpose/role and provisioning-race tests.                                                                             |
| Reusable authentication/authorization middleware                     | PASS       | Exported helpers; future CSRF composition limitation above.                                                               |
| HttpOnly/Secure/SameSite and safe local development                  | PASS       | Config and real-handler cookie assertions; deployed HTTPS browser behavior NOT TESTED.                                    |
| Server expiration/revocation, rotation and logout                    | PASS       | SQL/lifecycle/HTTP tests.                                                                                                 |
| CSRF/Origin and no wildcard credentialed CORS                        | PASS       | Current auth routes and rejection/preflight tests.                                                                        |
| No auth in localStorage; no sensitive response caching               | PASS       | Cookie/memory client boundary and no-store headers.                                                                       |
| Request/verify/current identity/logout contracts                     | PASS       | Routes and tests.                                                                                                         |
| Persian RTL login/resend/errors/return/logout, 360px                 | PASS       | Unit and mocked browser tests.                                                                                            |
| Minimal admin entry, preserved editor, no role dashboard             | PASS       | Client gate plus server role/MFA check.                                                                                   |
| Invalid/expired/reused/concurrent OTP tests                          | PASS       | Real PostgreSQL transitions.                                                                                              |
| Full theft/fixation scenario verification                            | NOT TESTED | Rotation/revocation covered; dedicated attack scenarios absent.                                                           |
| Explicit two-customer account isolation verification                 | NOT TESTED | No cross-account identity substitution scenario.                                                                          |
| Live browser to API/database integration                             | NOT TESTED | Mocked browser and real backend HTTP tested separately.                                                                   |
| Existing catalog/frontend compatibility                              | PASS       | Public contract and fixture/preview tests pass.                                                                           |
| Required Go/SQLC/PostgreSQL/Bun checks, no Docker                    | PASS       | Unified command exit zero; optional race detector blocked separately.                                                     |
| No commerce mutations/payments/inventory/orders/M1b                  | PASS       | Routes/schema/diff reviewed; previews remain local.                                                                       |

## 9. Acceptance decision

| M2 acceptance criterion                                                     | Status                                                                   |
| --------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| Customer authentication and admin identity authorization implemented/tested | PASS for code with deterministic SMS; actual Sajad provisioning BLOCKED. |
| No production OTP bypass                                                    | PASS.                                                                    |
| Stored, revocable sessions                                                  | PASS.                                                                    |
| Public catalog unchanged                                                    | PASS.                                                                    |
| Existing storefront/admin designs preserved                                 | PASS within inspected source and responsive/regression coverage.         |
| No commerce mutations                                                       | PASS.                                                                    |
| No real payments/inventory/order processing                                 | PASS.                                                                    |

The core implementation is ready for review, but **unqualified M2 acceptance is not fully evidenced**. Explicit account-isolation and theft/fixation scenarios and live browser/backend verification remain NOT TESTED. Real SMS and business-admin provisioning remain BLOCKED operational dependencies; provider deferral was expressly allowed by the original brief. Production login is not ready for use. Complete the missing verification and explicitly accept or resolve the review limitations before claiming every original requirement passed. Stop at M2.
