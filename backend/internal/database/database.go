package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/myngrok/backend/migrations"
)

type Pool struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Pool, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL must not be empty")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	config.MaxConnLifetime = time.Hour
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	result := &Pool{pool: pool}
	if err := result.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return result, nil
}

func (p *Pool) Close() { p.pool.Close() }

func (p *Pool) Raw() *pgxpool.Pool { return p.pool }

func (p *Pool) Ping(ctx context.Context) error {
	if err := p.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}

func (p *Pool) Migrate(ctx context.Context) error {
	if _, err := p.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version TEXT PRIMARY KEY,
        applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    )`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	for _, migration := range migrations.All {
		var exists bool
		if err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, migration.Version).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %s: %w", migration.Version, err)
		}
		if exists {
			continue
		}
		tx, err := p.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", migration.Version, err)
		}
		if _, err = tx.Exec(ctx, migration.UpSQL); err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, migration.Version)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", migration.Version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", migration.Version, err)
		}
	}
	return nil
}
