package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shavandi/store/backend/internal/config"
)

const startupTimeout = 5 * time.Second

func Open(ctx context.Context, database config.Database) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(database.URL)
	if err != nil {
		return nil, errors.New("DATABASE_URL must be a valid PostgreSQL URL")
	}

	poolConfig.MinConns = database.MinConns
	poolConfig.MaxConns = database.MaxConns
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.MaxConnLifetime = time.Hour
	if timeout := poolConfig.ConnConfig.ConnectTimeout; timeout <= 0 || timeout > startupTimeout {
		poolConfig.ConnConfig.ConnectTimeout = startupTimeout
	}

	connectCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return pool, nil
}
