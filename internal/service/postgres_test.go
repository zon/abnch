package service

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zon/abnch/internal/devconfig"
)

// TestOpenPostgresReportsUnreachableDatabase points the driver at a port that
// is closed, so the connection is refused the way an unreachable database is.
func TestOpenPostgresReportsUnreachableDatabase(t *testing.T) {
	port := freePort(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := OpenPostgres(ctx, devconfig.Postgres{
		Host:     "127.0.0.1",
		Port:     port,
		DBName:   "abnch",
		User:     "app",
		Password: "pw",
		SSLMode:  "disable",
		TimeZone: "UTC",
	})
	if err == nil {
		t.Fatal("OpenPostgres = nil, want an error for an unreachable database")
	}
	if !strings.Contains(err.Error(), strconv.Itoa(port)) {
		t.Errorf("error = %q, want it to name the unreachable host and port", err)
	}
}
