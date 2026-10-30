package store

import (
	"context"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/zon/abnch/internal/devconfig"
)

// Open connects to Postgres and confirms it is reachable. It does not create or
// change tables; Migrate does that. It returns a clear error when the database
// cannot be reached.
func Open(ctx context.Context, pg devconfig.Postgres) (*Store, error) {
	db, err := open(ctx, pg)
	if err != nil {
		return nil, err
	}
	return New(db), nil
}

// Migrate connects to Postgres and creates or updates the tables the store
// needs. It returns a clear error when the database cannot be reached or its
// tables cannot be created.
func Migrate(ctx context.Context, pg devconfig.Postgres) error {
	db, err := open(ctx, pg)
	if err != nil {
		return err
	}

	s := New(db)
	defer s.Close()
	if err := s.Migrate(); err != nil {
		return fmt.Errorf("failed to create tables in postgres %s:%d: %w", pg.Host, pg.Port, err)
	}
	return nil
}

// open connects to Postgres and confirms it is reachable.
func open(ctx context.Context, pg devconfig.Postgres) (*gorm.DB, error) {
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
	return db, nil
}
