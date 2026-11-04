package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/shavandi/store/backend/internal/config"
)

func TestOpenRejectsInvalidDatabaseURLWithoutConnecting(t *testing.T) {
	_, err := Open(context.Background(), config.Database{
		URL:      "postgres://[invalid",
		MinConns: 0,
		MaxConns: 8,
	})
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("Open error = %v, want DATABASE_URL validation error", err)
	}
}
