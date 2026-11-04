//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/shavandi/store/backend/internal/config"
	"github.com/shavandi/store/backend/internal/dbgen"
	"github.com/shavandi/store/backend/internal/httpapi"
)

const latestMigrationVersion int64 = 20261104120000

func TestRealPostgresMigrationReadinessAndVersionTracking(t *testing.T) {
	pool, gooseDB := openFreshIntegrationDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	assertLatestMigration(t, ctx, gooseDB, latestMigrationVersion)
	appliedBefore, err := appliedMigrationCount(ctx, gooseDB)
	if err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}

	checker := dbgen.New(pool)
	handler := httpapi.NewHandler(func(ctx context.Context) error {
		_, err := checker.Readiness(ctx)
		return err
	}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("real database readiness returned %d, want 200: %s", response.Code, response.Body.String())
	}

	migrationDir := filepath.Join("..", "..", "db", "migrations")
	if err := goose.UpContext(ctx, gooseDB, migrationDir); err != nil {
		t.Fatalf("repeat Goose up: %v", err)
	}
	assertLatestMigration(t, ctx, gooseDB, latestMigrationVersion)
	appliedAfter, err := appliedMigrationCount(ctx, gooseDB)
	if err != nil {
		t.Fatalf("count migrations after repeat up: %v", err)
	}
	if appliedAfter != appliedBefore {
		t.Fatalf("applied migration count changed from %d to %d after repeat up", appliedBefore, appliedAfter)
	}
}

func openFreshIntegrationDatabase(t *testing.T) (*pgxpool.Pool, *sql.DB) {
	t.Helper()
	databaseURL, ok := os.LookupEnv("TEST_DATABASE_URL")
	if !ok || strings.TrimSpace(databaseURL) == "" {
		t.Fatal("TEST_DATABASE_URL is required for tests tagged integration")
	}

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal("TEST_DATABASE_URL must be a valid PostgreSQL URL")
	}
	if !strings.HasSuffix(strings.ToLower(poolConfig.ConnConfig.Database), "_test") {
		t.Fatal("TEST_DATABASE_URL must name a dedicated database ending in _test")
	}
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		devConfig, err := pgxpool.ParseConfig(databaseURL)
		if err != nil {
			t.Fatal("DATABASE_URL must be valid when TEST_DATABASE_URL is set")
		}
		if sameDatabase(devConfig, poolConfig) {
			t.Fatal("DATABASE_URL and TEST_DATABASE_URL must name different databases")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := Open(ctx, config.Database{
		URL:      databaseURL,
		MinConns: 0,
		MaxConns: 4,
	})
	if err != nil {
		t.Fatalf("connect to test PostgreSQL database: %v", err)
	}

	lockConn, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		t.Fatalf("acquire integration database lock: %v", err)
	}
	if _, err := lockConn.Exec(ctx, "SELECT pg_advisory_lock($1, $2)", int32(20261103), int32(170000)); err != nil {
		lockConn.Release()
		pool.Close()
		t.Fatalf("lock integration database: %v", err)
	}

	var schemaExists, versionTableExists bool
	err = pool.QueryRow(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'shavandi'),
			to_regclass('public.goose_db_version') IS NOT NULL
	`).Scan(&schemaExists, &versionTableExists)
	if err != nil {
		_, _ = lockConn.Exec(context.Background(), "SELECT pg_advisory_unlock($1, $2)", int32(20261103), int32(170000))
		lockConn.Release()
		pool.Close()
		t.Fatalf("inspect test database: %v", err)
	}
	if schemaExists || versionTableExists {
		_, _ = lockConn.Exec(context.Background(), "SELECT pg_advisory_unlock($1, $2)", int32(20261103), int32(170000))
		lockConn.Release()
		pool.Close()
		t.Fatal("TEST_DATABASE_URL must point to a clean database with no shavandi schema or goose_db_version table")
	}

	gooseDB := stdlib.OpenDB(*poolConfig.ConnConfig)
	gooseDB.SetMaxOpenConns(1)
	gooseDB.SetMaxIdleConns(1)
	if err := goose.SetDialect("postgres"); err != nil {
		gooseDB.Close()
		_, _ = lockConn.Exec(context.Background(), "SELECT pg_advisory_unlock($1, $2)", int32(20261103), int32(170000))
		lockConn.Release()
		pool.Close()
		t.Fatalf("set Goose dialect: %v", err)
	}

	migrationDir := filepath.Join("..", "..", "db", "migrations")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()

		var versionTableExists bool
		if err := pool.QueryRow(cleanupCtx, "SELECT to_regclass('public.goose_db_version') IS NOT NULL").Scan(&versionTableExists); err != nil {
			t.Errorf("inspect Goose table during cleanup: %v", err)
		} else if versionTableExists {
			if err := goose.DownToContext(cleanupCtx, gooseDB, migrationDir, 0); err != nil {
				t.Errorf("roll back test-created Goose migrations: %v", err)
				_ = gooseDB.Close()
				_, _ = lockConn.Exec(cleanupCtx, "SELECT pg_advisory_unlock($1, $2)", int32(20261103), int32(170000))
				lockConn.Release()
				pool.Close()
				return
			}
		}

		if err := gooseDB.Close(); err != nil {
			t.Errorf("close Goose database: %v", err)
		}

		var schemaExists bool
		if err := pool.QueryRow(cleanupCtx, "SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'shavandi')").Scan(&schemaExists); err != nil {
			t.Errorf("inspect shavandi schema during cleanup: %v", err)
		} else if schemaExists {
			t.Errorf("test-owned migrations left the shavandi schema behind")
		} else if versionTableExists {
			if _, err := pool.Exec(cleanupCtx, "DROP TABLE public.goose_db_version RESTRICT"); err != nil {
				t.Errorf("remove test-created Goose version table: %v", err)
			}
		}

		if _, err := lockConn.Exec(cleanupCtx, "SELECT pg_advisory_unlock($1, $2)", int32(20261103), int32(170000)); err != nil {
			t.Errorf("release integration database lock: %v", err)
		}
		lockConn.Release()
		pool.Close()
	})

	if err := goose.UpContext(ctx, gooseDB, migrationDir); err != nil {
		t.Fatalf("apply Goose migrations: %v", err)
	}

	return pool, gooseDB
}

func sameDatabase(left, right *pgxpool.Config) bool {
	return strings.EqualFold(left.ConnConfig.Host, right.ConnConfig.Host) &&
		left.ConnConfig.Port == right.ConnConfig.Port &&
		left.ConnConfig.Database == right.ConnConfig.Database
}

func assertLatestMigration(t *testing.T, ctx context.Context, database *sql.DB, want int64) {
	t.Helper()
	var version int64
	err := database.QueryRowContext(ctx, `
		SELECT version_id
		FROM goose_db_version
		WHERE is_applied
		ORDER BY version_id DESC
		LIMIT 1
	`).Scan(&version)
	if err != nil {
		t.Fatalf("read latest migration version: %v", err)
	}
	if version != want {
		t.Fatalf("latest migration version = %d, want %d", version, want)
	}
}

func appliedMigrationCount(ctx context.Context, database *sql.DB) (int64, error) {
	var count int64
	err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM goose_db_version WHERE is_applied").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
