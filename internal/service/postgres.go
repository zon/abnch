package service

import (
	"context"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/zon/abnch/internal/devconfig"
)

// Database is a live Postgres connection the service holds while it runs.
type Database interface {
	Close() error
}

// Opener opens the database described by a service config.
type Opener func(ctx context.Context, pg devconfig.Postgres) (Database, error)

// OpenPostgres connects to Postgres and confirms it is reachable. It returns a
// clear error when the database cannot be reached.
func OpenPostgres(ctx context.Context, pg devconfig.Postgres) (Database, error) {
	db, err := gorm.Open(postgres.Open(DSN(pg)), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres %s:%d: %w", pg.Host, pg.Port, err)
	}

	conn, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to use postgres %s:%d: %w", pg.Host, pg.Port, err)
	}

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("postgres %s:%d is unreachable: %w", pg.Host, pg.Port, err)
	}

	return conn, nil
}
