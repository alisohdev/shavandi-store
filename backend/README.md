# Shavandi API foundation

The Go API lives in this folder as a separate module. The Astro storefront stays at the repository root and remains a static preview. M1a adds the catalog schema, internal domain/write service, and read-only public API. The storefront does not call these endpoints.

## Versions

- [Go 1.27.3](https://go.dev/doc/devel/release), selected by `go.mod`
- [PostgreSQL 18.6](https://www.postgresql.org/support/versioning/)
- [chi 5.5.0](https://github.com/go-chi/chi/releases/tag/v5.5.0)
- [pgx/v5 5.12.0](https://github.com/jackc/pgx/blob/master/CHANGELOG.md)
- [Goose 3.29.0](https://github.com/pressly/goose/releases/tag/v3.29.0)
- [sqlc 1.31.1](https://github.com/sqlc-dev/sqlc/releases/tag/v1.31.1)

PostgreSQL 18 is supported through November 2030. Install and configure PostgreSQL separately. This workflow does not install, start, stop, or reconfigure the system service. It does not use Docker or Docker Compose.

## Set environment variables in PowerShell

The API reads the process environment. `.env.example` lists the supported variables but the binary does not parse `.env` files.

```powershell
Set-Location backend
$env:DATABASE_URL = 'postgres://<user>@127.0.0.1:5432/shavandi?sslmode=disable'
$env:HTTP_ADDR = '127.0.0.1:8080'
$env:DATABASE_MIN_CONNS = '0'
$env:DATABASE_MAX_CONNS = '8'
```

Replace the example URL with your local PostgreSQL connection details. Keep passwords in your local environment or PostgreSQL password file. Do not commit them.

If `go env GOSUMDB` returns `off`, allow Go to verify toolchain downloads in the current PowerShell session:

```powershell
$env:GOSUMDB = 'sum.golang.org'
```

## Start the applications

Use one PowerShell terminal for Astro:

```powershell
# Run from the repository root.
bun run dev
```

The existing frontend checks and build run from the repository root:

```powershell
bun run check
bun run validate
bun run build
```

The repository README documents the Bun version that provides the existing native lint and format commands. Put that Bun directory first on `PATH` so child scripts use the same executable.

Use another terminal for the API:

```powershell
Set-Location backend
go run ./cmd/api
```

Press Ctrl+C to stop the API. The server drains active requests before it closes the PostgreSQL pool.

The API serves `GET /health/live`, `GET /health/ready`, `GET /api/v1/categories`, `GET /api/v1/products`, and `GET /api/v1/products/{slug}`. Catalog responses include active categories, published products, effective variant prices, and image references. Liveness does not contact PostgreSQL. Readiness and catalog reads have bounded contexts. There are no HTTP mutation routes; the catalog service and write repository are internal to the Go application.

## Apply migrations and generate SQL code

Run Goose from the `backend` directory after setting `DATABASE_URL`:

```powershell
go run github.com/pressly/goose/v3/cmd/goose@v3.29.0 -dir db/migrations postgres "$env:DATABASE_URL" up
go run github.com/pressly/goose/v3/cmd/goose@v3.29.0 -dir db/migrations postgres "$env:DATABASE_URL" status
go run github.com/pressly/goose/v3/cmd/goose@v3.29.0 -dir db/migrations postgres "$env:DATABASE_URL" version
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```

The first migration creates the `shavandi` schema. The catalog migration adds categories, products, permanent product slug history, variants, images, and an active-category view. Goose records applied versions in `goose_db_version`. Migrations run only when you invoke Goose. The API does not migrate the database at startup.

## Test and build the API

Run unit tests and Go's static analysis from `backend`:

```powershell
go test ./...
go vet ./...
go build -o .\\api.exe ./cmd/api
```

Run the complete M1a verification from the repository root. It reads backend `.env` assignments into only its child process, generates sqlc code, runs unit and real-PostgreSQL integration tests, vet, build, and Astro validation. It never prints database URLs or writes to `.env`.

```powershell
.\backend\scripts\verify-m1a.ps1 -BunExe 'C:\path\to\bun.exe'
```

`-BunExe` is optional when the documented Bun build is already first on `PATH`.

The PostgreSQL integration tests use a real, dedicated database. Give its database name a `_test` suffix. Tests require no existing `shavandi` schema and no `goose_db_version` table. They apply migrations, exercise catalog writes and public visibility, then remove only the schema and version table they created. Tests refuse a database with either artifact already present and verify it differs from `DATABASE_URL`.

```powershell
$env:TEST_DATABASE_URL = 'postgres://<user>@127.0.0.1:5432/shavandi_test?sslmode=disable'
go test -tags=integration ./...
```

The integration tests do not use Docker. They fail if `TEST_DATABASE_URL` is missing, invalid, or does not point to a clean database whose name ends in `_test`. See [the M1a handoff](../docs/m1a-handoff.md) for schema constraints, service rules, endpoint examples, fixture mapping, and test coverage.
