package config

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultHTTPAddr = "127.0.0.1:8080"
	defaultMinConns = int32(0)
	defaultMaxConns = int32(8)
	maxPoolConns    = int32(100)
)

type Database struct {
	URL      string
	MinConns int32
	MaxConns int32
}

type Config struct {
	HTTPAddr string
	Database Database
}

func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (Config, error) {
	databaseURL, ok := lookup("DATABASE_URL")
	if !ok || strings.TrimSpace(databaseURL) == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	parsedURL, err := url.Parse(databaseURL)
	if err != nil || (parsedURL.Scheme != "postgres" && parsedURL.Scheme != "postgresql") {
		return Config{}, errors.New("DATABASE_URL must be a valid PostgreSQL URL")
	}
	if _, err := pgxpool.ParseConfig(databaseURL); err != nil {
		return Config{}, errors.New("DATABASE_URL must be a valid PostgreSQL URL")
	}

	httpAddr := defaultHTTPAddr
	if value, ok := lookup("HTTP_ADDR"); ok && strings.TrimSpace(value) != "" {
		httpAddr = strings.TrimSpace(value)
	}
	if err := validateHTTPAddr(httpAddr); err != nil {
		return Config{}, err
	}

	minConns, err := poolCount(lookup, "DATABASE_MIN_CONNS", defaultMinConns, 0, maxPoolConns)
	if err != nil {
		return Config{}, err
	}
	maxConns, err := poolCount(lookup, "DATABASE_MAX_CONNS", defaultMaxConns, 1, maxPoolConns)
	if err != nil {
		return Config{}, err
	}
	if minConns > maxConns {
		return Config{}, errors.New("DATABASE_MIN_CONNS cannot exceed DATABASE_MAX_CONNS")
	}

	return Config{
		HTTPAddr: httpAddr,
		Database: Database{
			URL:      databaseURL,
			MinConns: minConns,
			MaxConns: maxConns,
		},
	}, nil
}

func poolCount(
	lookup func(string) (string, bool),
	name string,
	defaultValue int32,
	minimum int32,
	maximum int32,
) (int32, error) {
	value, ok := lookup(name)
	if !ok || strings.TrimSpace(value) == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32)
	if err != nil || parsed < int64(minimum) || parsed > int64(maximum) {
		return 0, errors.New(name + " must be an integer within its supported range")
	}
	return int32(parsed), nil
}

func validateHTTPAddr(address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return errors.New("HTTP_ADDR must be a host:port address")
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("HTTP_ADDR must use a port from 1 through 65535")
	}
	return nil
}
