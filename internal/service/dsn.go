package service

import (
	"fmt"
	"strings"

	"github.com/zon/abnch/internal/devconfig"
)

// DSN renders the Postgres connection details as the keyword/value string the
// Gorm Postgres driver expects. The keys match the DSN keywords, so the config
// key timeZone maps to the driver's capitalized TimeZone keyword.
func DSN(pg devconfig.Postgres) string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s TimeZone=%s",
		dsnValue(pg.Host),
		pg.Port,
		dsnValue(pg.DBName),
		dsnValue(pg.User),
		dsnValue(pg.Password),
		dsnValue(pg.SSLMode),
		dsnValue(pg.TimeZone),
	)
}

// dsnValue renders a DSN value, quoting it when it holds a character the
// keyword/value format reserves.
func dsnValue(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " '\\") {
		return value
	}
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(value) + "'"
}
