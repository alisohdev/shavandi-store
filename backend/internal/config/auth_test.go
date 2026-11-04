package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func authEnvironment() map[string]string {
	return map[string]string{"AUTH_SECRET": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32))), "AUTH_ALLOWED_ORIGINS": "https://store.example"}
}

func TestAuthenticationDefaultsAreSecure(t *testing.T) {
	cfg, err := loadAuthentication(mapLookup(authEnvironment()))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CookieSecure || len(cfg.Secret) != 32 || len(cfg.AllowedOrigins) != 1 {
		t.Fatal("authentication defaults are not secure")
	}
	values := authEnvironment()
	values["APP_ENV"] = "development"
	values["AUTH_ALLOWED_ORIGINS"] = ""
	values["AUTH_COOKIE_SECURE"] = "false"
	cfg, err = loadAuthentication(mapLookup(values))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CookieSecure || len(cfg.AllowedOrigins) != 2 {
		t.Fatal("explicit development configuration failed")
	}
}

func TestAuthenticationRejectsUnsafeConfigurationWithoutLeakingSecrets(t *testing.T) {
	cases := []map[string]string{
		{"AUTH_SECRET": "private-invalid-secret"},
		{"AUTH_COOKIE_SECURE": "false"},
		{"AUTH_ALLOWED_ORIGINS": "*"},
		{"AUTH_ALLOWED_ORIGINS": "https://*.example"},
		{"AUTH_ALLOWED_ORIGINS": "https://store.example/"},
		{"AUTH_ALLOWED_ORIGINS": "https://store.example?other=1"},
		{"AUTH_ALLOWED_ORIGINS": "http://store.example"},
		{"APP_ENV": "development", "AUTH_ALLOWED_ORIGINS": "http://192.0.2.1:4321"},
		{"APP_ENV": "staging"},
		{"AUTH_COOKIE_SECURE": "maybe"},
	}
	for _, change := range cases {
		values := authEnvironment()
		for k, v := range change {
			values[k] = v
		}
		_, err := loadAuthentication(mapLookup(values))
		if err == nil {
			t.Errorf("accepted unsafe configuration keys %v", change)
		} else if strings.Contains(err.Error(), "private-invalid-secret") {
			t.Fatal("configuration error leaked secret")
		}
	}
}
