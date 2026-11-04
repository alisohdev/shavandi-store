package config

import (
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Authentication struct {
	Secret         []byte
	AllowedOrigins []string
	CookieSecure   bool
}

func LoadAuthentication() (Authentication, error) { return loadAuthentication(os.LookupEnv) }

func loadAuthentication(lookup func(string) (string, bool)) (Authentication, error) {
	secretText, _ := lookup("AUTH_SECRET")
	secret, err := base64.StdEncoding.DecodeString(secretText)
	if err != nil || len(secret) < 32 || len(secret) > 128 {
		return Authentication{}, errors.New("AUTH_SECRET must be base64 encoding of 32 through 128 random bytes")
	}
	environment, _ := lookup("APP_ENV")
	if environment == "" {
		environment = "production"
	}
	if environment != "production" && environment != "development" {
		return Authentication{}, errors.New("APP_ENV must be production or development")
	}
	secure := true
	if raw, ok := lookup("AUTH_COOKIE_SECURE"); ok && raw != "" {
		secure, err = strconv.ParseBool(raw)
		if err != nil {
			return Authentication{}, errors.New("AUTH_COOKIE_SECURE must be true or false")
		}
	}
	if !secure && environment != "development" {
		return Authentication{}, errors.New("insecure authentication cookies are restricted to APP_ENV=development")
	}
	rawOrigins, _ := lookup("AUTH_ALLOWED_ORIGINS")
	if rawOrigins == "" && environment == "development" {
		rawOrigins = "http://localhost:4321,http://127.0.0.1:4321"
	}
	if rawOrigins == "" {
		return Authentication{}, errors.New("AUTH_ALLOWED_ORIGINS is required")
	}
	var origins []string
	for _, value := range strings.Split(rawOrigins, ",") {
		origin := strings.TrimSpace(value)
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" || strings.Contains(origin, "*") || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return Authentication{}, errors.New("AUTH_ALLOWED_ORIGINS must contain exact HTTP(S) origins without paths or wildcards")
		}
		loopback := parsed.Hostname() == "localhost"
		if address := net.ParseIP(parsed.Hostname()); address != nil {
			loopback = address.IsLoopback()
		}
		if parsed.Scheme != "https" && (environment != "development" || !loopback) {
			return Authentication{}, errors.New("authentication origins must use HTTPS except loopback development origins")
		}
		origins = append(origins, origin)
	}
	return Authentication{Secret: secret, AllowedOrigins: origins, CookieSecure: secure}, nil
}
