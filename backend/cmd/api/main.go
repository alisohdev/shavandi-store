package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/shavandi/store/backend/internal/auth"
	"github.com/shavandi/store/backend/internal/catalog"
	"github.com/shavandi/store/backend/internal/config"
	"github.com/shavandi/store/backend/internal/dbgen"
	"github.com/shavandi/store/backend/internal/httpapi"
	"github.com/shavandi/store/backend/internal/imagestore"
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
	smsSender, err := newSMSSender(os.Getenv("APP_ENV"), cfg.HTTPAddr, os.Stderr)
	if err != nil {
		return err
	}
	authService, err := auth.NewService(postgres.NewAuthStore(pool), smsSender, authConfig.Secret)
	if err != nil {
		return err
	}
	imageDir := os.Getenv("PRODUCT_IMAGE_DIR")
	if imageDir == "" {
		base, dirErr := os.UserConfigDir()
		if dirErr != nil {
			return fmt.Errorf("resolve local image storage directory: %w", dirErr)
		}
		imageDir = filepath.Join(base, "shavandi-store", "product-images")
	}
	imageStorage, err := imagestore.NewLocal(imageDir)
	if err != nil {
		return fmt.Errorf("initialize product image storage: %w", err)
	}
	handler := httpapi.NewHandler(func(ctx context.Context) error {
		_, err := queries.Readiness(ctx)
		return err
	}, catalogService, logger, httpapi.WithAuthentication(authService, httpapi.AuthOptions{AllowedOrigins: authConfig.AllowedOrigins, CookieSecure: authConfig.CookieSecure}), httpapi.WithAdminCatalog(catalogService), httpapi.WithImageStore(imageStorage), httpapi.WithCart(postgres.NewCartStore(pool)), httpapi.WithInventory(postgres.NewInventoryStore(pool)))

	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	return httpapi.Serve(ctx, listener, handler, logger)
}
