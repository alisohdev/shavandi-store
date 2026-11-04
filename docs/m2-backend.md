# M2 backend authentication

M2 adds customer SMS sign-in and a separate administrator password-plus-SMS flow. Sessions are opaque database-backed cookies. The production binary deliberately uses an unavailable SMS sender: without a real provider adapter, OTP requests return `503 sms_unavailable`, and no sign-in succeeds. Test senders exist only in Go test files and cannot be selected by configuration.

The development database is migrated to Goose version `20261104120000`. No real administrator was provisioned: Sajad's phone and password were not supplied. The frontend catalog and its fixture data remain unchanged.

## Design decision

Two structurally distinct candidates were considered before implementation:

| Candidate                                           | Ownership and transitions                                                                                                                                                                                        | Assessment                                                                                                                         |
| --------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| A: Go service with explicit PostgreSQL transactions | The auth service owns policy and crypto; a pgx repository executes sqlc-generated statements inside transactions. PostgreSQL target-row locks serialize resend, verification, and trusted role/password changes. | Chosen. Fits the existing M1a repository structure, keeps policy in ordinary Go, and allows real-database race tests.              |
| B: PostgreSQL auth functions                        | Stored functions own rate limiting, OTP consumption, account creation, and session issuance. Go calls generated wrappers and handles SMS/cookies.                                                                | Viable, but would split authentication policy between Go and PL/pgSQL and require an additional database-function testing surface. |

SMS delivery occurs after reserving a pending challenge. A second transaction activates it only after successful delivery. Delivery failure consumes the pending challenge and records an audit event. A failed database activation leaves the challenge unusable. Sending a message does not itself authenticate a user.

## Configuration

The API and admin CLI read process environment variables; neither parses `.env` automatically. `backend/.env.example` lists the supported settings. The verification scripts load local `.env` assignments into their child process without printing values or changing the file.

| Variable               | Rule                                                                                                                                                                                                                                                  |
| ---------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `DATABASE_URL`         | Existing PostgreSQL connection setting.                                                                                                                                                                                                               |
| `TEST_DATABASE_URL`    | Separate clean database ending in `_test`, required for integration tests.                                                                                                                                                                            |
| `APP_ENV`              | `production` by default; only `production` or `development` accepted.                                                                                                                                                                                 |
| `AUTH_SECRET`          | Required standard base64 encoding of 32–128 cryptographically random bytes. Keep stable across restarts and outside source control. Changing it invalidates existing challenge, CSRF, and session verifiers.                                          |
| `AUTH_ALLOWED_ORIGINS` | Comma-separated exact origins, including scheme and port. Production requires HTTPS. No paths, queries, fragments, credentials, or wildcards. Development defaults to `http://localhost:4321,http://127.0.0.1:4321` and allows HTTP only on loopback. |
| `AUTH_COOKIE_SECURE`   | Defaults to `true`. `false` is rejected unless `APP_ENV=development`.                                                                                                                                                                                 |

Generate a secret in PowerShell without printing it:

```powershell
$authBytes = New-Object byte[] 32
$authRng = [Security.Cryptography.RandomNumberGenerator]::Create()
try { $authRng.GetBytes($authBytes) } finally { $authRng.Dispose() }
$env:AUTH_SECRET = [Convert]::ToBase64String($authBytes)
```

For local HTTP development, also set `APP_ENV=development` and `AUTH_COOKIE_SECURE=false`. Production cookies use the `__Host-` prefix, `Secure`, `HttpOnly`, `SameSite=Strict`, `Path=/`, and no Domain. Development HTTP uses separate `shavandi_dev_session` and `shavandi_dev_csrf` names. Do not use the development settings for a deployed store.

There is no environment switch for fixed OTPs, a fake provider, password-only login, an admin bypass, or authentication disabling. `GET /health/ready` remains a database readiness check; it does not claim SMS provider readiness.

## API contract

Every POST requires `Content-Type: application/json`, an exact allowed `Origin`, and an `X-CSRF-Token` matching the signed HttpOnly CSRF cookie. Clients fetch the token with `GET /api/v1/auth/csrf`, hold it in memory, and use credentialed requests. GET requests may omit Origin; a supplied unapproved Origin is rejected. CORS echoes only an allowed exact origin and allows credentialed GET/POST requests and the explicit Content-Type/CSRF headers.

| Method and path                          | Request                                  | Success                                                                                                             |
| ---------------------------------------- | ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `GET /api/v1/auth/csrf`                  | None                                     | `200 {"csrf_token":"..."}` and CSRF cookie.                                                                         |
| `POST /api/v1/auth/customer/otp/request` | `{"phone":"09123456789"}`                | `202 {"challenge_id":"...","expires_in":300,"resend_after":60}` after SMS delivery.                                 |
| `POST /api/v1/auth/customer/otp/verify`  | `{"challenge_id":"...","code":"123456"}` | `200` session JSON and a fresh session cookie.                                                                      |
| `POST /api/v1/auth/admin/login`          | `{"phone":"...","password":"..."}`       | Same `202` challenge shape, only after a correct active-admin password and successful SMS delivery.                 |
| `POST /api/v1/auth/admin/otp/verify`     | Same verification shape                  | `200` session JSON and a fresh admin MFA session cookie.                                                            |
| `GET /api/v1/auth/session`               | Session cookie                           | `200` session JSON, or `401 authentication_required`.                                                               |
| `GET /api/v1/auth/admin/session`         | Session cookie                           | Same JSON only after both admin role and `admin_password_sms` assurance; customer sessions receive `403 forbidden`. |
| `POST /api/v1/auth/logout`               | `{}`                                     | `204`, database revocation and cookie removal. Idempotent.                                                          |

Session JSON is `{"user":{"id":"<uuid>","phone":"+989123456789","role":"customer"},"expires_at":"<RFC3339 timestamp>"}`. Admin sessions use role `admin`. Tokens, password hashes, active-state flags, OTPs, and authentication methods are excluded from JSON. Customer sessions expire after 12 hours; admin sessions after one hour. A successful sign-in rotates the cookie and revokes any prior session token presented by that browser.

Phones normalize to Iranian mobile E.164 (`+989xxxxxxxxx`). Local `09...`, national `9...`, international `+98...` and `0098...`, Persian/Arabic digits, spaces, hyphens, and parentheses are supported. Foreign and malformed numbers are rejected. OTPs are six digits; verification also normalizes Persian/Arabic digits.

Errors retain the existing envelope: `{"error":{"code":"...","message":"...","request_id":"..."}}`.

| Status | Codes                                                                                                                                             |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `400`  | `invalid_request` for invalid/bounded input.                                                                                                      |
| `401`  | `invalid_credentials`, `invalid_otp`, `authentication_required`. Wrong, expired, consumed, exhausted, and wrong-purpose OTPs share `invalid_otp`. |
| `403`  | `origin_not_allowed`, `csrf_invalid`, `forbidden`.                                                                                                |
| `415`  | `invalid_request` for an unsupported request content type.                                                                                        |
| `429`  | `rate_limited`, with integer-second `Retry-After`.                                                                                                |
| `503`  | `sms_unavailable` or generic `auth_unavailable`; provider/database error text is not exposed.                                                     |

Resend uses the same request endpoint after the cooldown. Admin resend repeats the password step. A new challenge invalidates the previous unconsumed challenge for the same phone/purpose. Customer issuance is identical for unknown, existing, disabled, and administrator phone identities; customer verification cannot grant a session to an administrator or disabled account.

## Security and persistence

- Passwords use Argon2id with 64 MiB, three passes, one lane, a random 16-byte salt, and a 32-byte result. Administrator passwords must contain at least 12 characters and at most 1024 bytes. Concurrent web password hashing is bounded to two operations. Unknown/disabled/non-admin credentials perform the same password-hashing work before the generic rejection.
- Challenge IDs and session tokens contain 32 random bytes. PostgreSQL stores domain-separated keyed HMAC-SHA-256 verifiers, never their raw values. Six-digit OTP verifiers also use the application secret so database access alone cannot brute-force the small code space. Password hashes contain their own salts.
- Challenges last at most five minutes and permit five attempts. The attempt update, consumption, user creation, session creation, old-session revocation, and success audit commit atomically. Denials also commit counters and audit events. Expiry is checked again after locking; session queries use current database statement time.
- A phone/purpose target row serializes resend, verify, and provisioning. Concurrent valid verification produces one session. Concurrent incorrect verification cannot exceed five stored attempts. A phone-wide verification budget survives replacement challenges.
- SMS requests permit five attempts per phone and 20 per IP per hour, plus a 60-second resend cooldown. Admin password attempts permit five per phone and 30 per IP per 15 minutes. OTP verification has challenge, IP, and shared phone budgets (10 per phone per 15 minutes). Cooldown and failed-send attempts count toward limits. IP identity comes from `RemoteAddr`; untrusted forwarding headers are ignored.
- Audit rows record event type, optional user UUID, time, and keyed subject/IP fingerprints. They contain no passwords, raw OTPs, raw tokens, phone text, or provider error text. Provider adapters must honor the bounded context and must not log message codes.
- A partial unique index enforces one administrator identity. The database rejects administrator rows without a non-null Argon2id hash. Trusted reprovisioning of the same phone changes the password, revokes sessions, and invalidates pending challenges; it refuses a second administrator phone.
- `RequireAuthentication(service, options)`, `SessionFromContext`, and `RequireAdmin` are reusable HTTP middleware helpers. Admin authorization checks both role and completed MFA assurance. No catalog mutation routes or public admin provisioning routes were added.

The password parameters exceed the [OWASP minimum Argon2id guidance](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html), using Go's [Argon2id implementation](https://pkg.go.dev/golang.org/x/crypto/argon2). Existing dependencies remain pinned; `golang.org/x/crypto v0.58.0` is added for password hashing.

Expired rate buckets, challenges, sessions, and audit data need an operator retention policy before long-term production use. No scheduler or delivery provider was invented as part of M2.

## Administrator provisioning

Run after loading the database/auth environment and applying migrations:

```powershell
Set-Location backend
go run ./cmd/adminctl --phone '<Sajad-mobile>' --password-stdin
```

The command reads one password from standard input; it accepts no password argument. Supply stdin through a hidden prompt or secret manager so a password does not enter shell history. For example, if Python is available:

```powershell
python -c "import getpass, subprocess; subprocess.run(['go','run','./cmd/adminctl','--phone','<Sajad-mobile>','--password-stdin'], input=getpass.getpass('Admin password: ')+'\n', text=True, check=True)"
```

The command outputs only the administrator UUID on success. It does not send an SMS or create an authenticated session. There is no default phone, default password, seeded admin, or recovery bypass.

## Verification performed

All checks used the existing local PostgreSQL service without Docker. `.env` values stayed in child-process environment and were not printed.

- Both `DATABASE_URL` and `TEST_DATABASE_URL`: pgx connection/ping passed before implementation.
- Existing migration/readiness/version integration test: passed before changes.
- `sqlc generate` with sqlc 1.31.1: passed after schema/query changes.
- `go test ./...`: passed.
- `go test -tags=integration ./... -count=1`: passed after final implementation changes.
- `go vet ./...`: passed.
- `go build ./...`: passed.
- `git diff --check -- backend`: passed.
- Development Goose up: passed, latest version `20261104120000`.
- `govulncheck ./...`: could not complete because the vulnerability database returned HTTP 403 for `https://vuln.go.dev/index/modules.json.gz`; no vulnerability assessment result is claimed.

Unit tests cover normalization, salted password verification and bounds, CSRF forgery, config fail-closed behavior, middleware origin/CSRF rules, bounded JSON, secure cookies, CORS preflight, error redaction, role/assurance, and CLI stdin bounds. Real PostgreSQL tests cover full customer/admin lifecycles, verifier storage, distinct-IP simultaneous verification, concurrent failed-attempt bounds, cooldown/replacement/expiry, global limits across resends, logout/rotation/reset/disable revocation, failed SMS, real HTTP requests, singleton provisioning races, password-null constraints, and a deterministic password-change race between credential verification and OTP issuance.

The race detector was unavailable on this host (`CGO_ENABLED=0`, no GCC). Concurrent database behavior was exercised against PostgreSQL directly. Integration tests retain the clean `_test` database guard, dev/test database separation check, advisory suite lock, Goose version assertions, and rollback of only the schema/version table created by the tests.
