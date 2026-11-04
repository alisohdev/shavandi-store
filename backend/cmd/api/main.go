package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/shavandi/store/backend/internal/auth"
	"github.com/shavandi/store/backend/internal/catalog"
	"github.com/shavandi/store/backend/internal/config"
	"github.com/shavandi/store/backend/internal/dbgen"
	"github.com/shavandi/store/backend/internal/httpapi"
	"github.com/shavandi/store/backend/internal/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	authConfig, err := config.LoadAuthentication()
	if err != nil {
		return fmt.Errorf("load authentication configuration: %w", err)
	}

	pool, err := postgres.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	queries := dbgen.New(pool)
	catalogService := catalog.NewService(postgres.NewCatalogRepository(pool))
	authService, err := auth.NewService(postgres.NewAuthStore(pool), auth.UnavailableSMS{}, authConfig.Secret)
	if err != nil {
		return err
	}
	handler := httpapi.NewHandler(func(ctx context.Context) error {
		_, err := queries.Readiness(ctx)
		return err
	}, catalogService, logger, httpapi.WithAuthentication(authService, httpapi.AuthOptions{AllowedOrigins: authConfig.AllowedOrigins, CookieSecure: authConfig.CookieSecure}))

	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	return httpapi.Serve(ctx, listener, handler, logger)
}
