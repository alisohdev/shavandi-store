package config

import (
	"strings"
	"testing"
)

func TestLoadRequiresPostgresURL(t *testing.T) {
	_, err := load(func(string) (string, bool) { return "", false })
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("load error = %v, want DATABASE_URL requirement", err)
	}
}

func TestLoadDefaultsAndParsesPoolBounds(t *testing.T) {
	cfg, err := load(mapLookup(map[string]string{
		"DATABASE_URL": "postgres://store:secret@localhost:5435/shavandi?sslmode=disable",
	}))
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}
	if cfg.HTTPAddr != defaultHTTPAddr {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, defaultHTTPAddr)
	}
	if cfg.Database.MinConns != defaultMinConns || cfg.Database.MaxConns != defaultMaxConns {
		t.Errorf("pool bounds = %d..%d, want %d..%d",
			cfg.Database.MinConns, cfg.Database.MaxConns, defaultMinConns, defaultMaxConns)
	}

	cfg, err = load(mapLookup(map[string]string{
		"DATABASE_URL":       "postgres://localhost/shavandi",
		"HTTP_ADDR":          ":9090",
		"DATABASE_MIN_CONNS": "2",
		"DATABASE_MAX_CONNS": "12",
	}))
	if err != nil {
		t.Fatalf("load with explicit settings returned error: %v", err)
	}
	if cfg.HTTPAddr != ":9090" || cfg.Database.MinConns != 2 || cfg.Database.MaxConns != 12 {
		t.Errorf("loaded config = %#v", cfg)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "non postgres URL",
			env:  map[string]string{"DATABASE_URL": "http://localhost/database"},
			want: "DATABASE_URL",
		},
		{
			name: "malformed URL does not expose its contents",
			env:  map[string]string{"DATABASE_URL": "postgres://user:private@[invalid"},
			want: "DATABASE_URL",
		},
		{
			name: "invalid address",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost/shavandi",
				"HTTP_ADDR":    "localhost",
			},
			want: "HTTP_ADDR",
		},
		{
			name: "zero max connections",
			env: map[string]string{
				"DATABASE_URL":       "postgres://localhost/shavandi",
				"DATABASE_MAX_CONNS": "0",
			},
			want: "DATABASE_MAX_CONNS",
		},
		{
			name: "min exceeds max",
			env: map[string]string{
				"DATABASE_URL":       "postgres://localhost/shavandi",
				"DATABASE_MIN_CONNS": "9",
				"DATABASE_MAX_CONNS": "8",
			},
			want: "DATABASE_MIN_CONNS",
		},
		{
			name: "max is bounded",
			env: map[string]string{
				"DATABASE_URL":       "postgres://localhost/shavandi",
				"DATABASE_MAX_CONNS": "101",
			},
			want: "DATABASE_MAX_CONNS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := load(mapLookup(test.env))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("load error = %v, want message containing %q", err, test.want)
			}
			if strings.Contains(err.Error(), "private") {
				t.Fatalf("load error exposed connection details: %v", err)
			}
		})
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
