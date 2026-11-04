# M0 verification report

Date: 2026-11-03

## Scope and layout

The Astro storefront remains a static application at the repository root. The Go API is a separate module in `backend/`, with the process entry point in `cmd/api` and small internal packages for configuration, PostgreSQL, generated SQL, and HTTP lifecycle. The API currently exposes health endpoints only. No storefront routes or design were changed, and no commerce schema or business logic was added.

Selected versions are Go 1.27.3, PostgreSQL 18.6, chi 5.5.0, pgx/v5 5.12.0, Goose 3.29.0, and sqlc 1.31.1. PostgreSQL 18.6 is a supported stable release; version details and Windows commands are in [the backend README](../backend/README.md).

## Database used

The installed Windows PostgreSQL 18.6 service was running on `127.0.0.1:5435`, but a password-authenticated connection as `postgres` failed with `fe_sendauth: no password supplied`. No database URL or PostgreSQL credentials were present in the environment. The service was left unchanged.

To run real database checks without changing that service, a temporary loopback-only PostgreSQL 18.6 cluster was started from the already-installed PostgreSQL binaries on port `5436`. It contained `shavandi` and the dedicated `shavandi_m0_test` database, using temporary trust authentication. The test cluster was separate from the installed service. The temporary cluster has been stopped. Its data folder, log, and temporary API binaries remain under `%TEMP%` because automatic review blocked cleanup deletion.

## Verification results

| Check                                                                                                                           | Result                                                                                                                                                                                                              |
| ------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Go tests: `go test ./...`                                                                                                       | Passed.                                                                                                                                                                                                             |
| Go static analysis: `go vet ./...`                                                                                              | Passed.                                                                                                                                                                                                             |
| API build: `go build -o <temporary-path> ./cmd/api`                                                                             | Passed.                                                                                                                                                                                                             |
| sqlc: `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`                                                               | Passed. Generated sqlc has no unused `Querier` interface.                                                                                                                                                           |
| Goose migration on clean `shavandi` database: `up`, `status`, `version` using Goose 3.29.0                                      | Passed. Goose reported migration version `20261103150500`.                                                                                                                                                          |
| Real PostgreSQL integration: `go test -tags=integration ./... -count=1` with `TEST_DATABASE_URL` pointing to `shavandi_m0_test` | Passed. The test applied migrations, verified the tracked version and generated readiness query, repeated the migration safely, and cleaned only its test-owned artifacts.                                          |
| Migration down safety                                                                                                           | Passed. `goose down` refused to drop a schema containing a probe table (`RESTRICT`); the table remained. The probe was removed manually, and `goose up` left the recorded version intact.                           |
| Live API health behavior                                                                                                        | Passed. With PostgreSQL available, `/health/live` and `/health/ready` both returned 200. After stopping only the temporary database, liveness remained 200 and readiness returned 503 with the JSON error envelope. |
| Graceful shutdown                                                                                                               | Passed in `TestServeGracefullyDrainsActiveRequest`. The live API listener closed after Ctrl+C, but the terminal wrapper returned status 1 and did not provide a clean process exit-code check.                      |
| Frontend: `bun run validate`                                                                                                    | Passed after the root documentation updates: Astro check had 0 errors, warnings, or hints; lint and formatting passed; 16 tests passed (354 assertions); Astro built 30 pages.                                      |
| Docker workflow search                                                                                                          | No Docker command, Compose requirement, or container workflow was introduced.                                                                                                                                       |

The API connection setup applies a five-second bound to PostgreSQL connection attempts, preserving any shorter positive URL timeout. Readiness queries and HTTP shutdown also use bounded contexts.

## Local Windows commands

From the repository root, start and validate the existing storefront with `bun run dev` and `bun run validate`. In another PowerShell terminal:

```powershell
Set-Location backend
$env:DATABASE_URL = 'postgres://<user>@127.0.0.1:5432/shavandi?sslmode=disable'
go run ./cmd/api
```

Apply and inspect migrations, regenerate query code, and run Go checks from `backend/`:

```powershell
go run github.com/pressly/goose/v3/cmd/goose@v3.29.0 -dir db/migrations postgres "$env:DATABASE_URL" up
go run github.com/pressly/goose/v3/cmd/goose@v3.29.0 -dir db/migrations postgres "$env:DATABASE_URL" status
go run github.com/pressly/goose/v3/cmd/goose@v3.29.0 -dir db/migrations postgres "$env:DATABASE_URL" version
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
go test ./...
go vet ./...
go build -o .\api.exe ./cmd/api
```

The opt-in integration test requires a dedicated real PostgreSQL database with a `_test` suffix, configured as `TEST_DATABASE_URL`. It does not use Docker.

## Remaining local prerequisite

The existing PostgreSQL service needs a valid local password or a suitable password-file/configuration entry before it can be used by future commands. Its configuration was intentionally not changed. Set `DATABASE_URL` and `TEST_DATABASE_URL` in the local PowerShell process (or use an ignored local env file plus a loader of your choice); `.env.example` contains names only and the Go binary does not load `.env` files.
