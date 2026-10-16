// Package version reports the semantic version the repo's apps share.
package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var fileVersion string

// Version is the semantic version shared by abnch, abnchd, and abnchctl.
var Version = strings.TrimSpace(fileVersion)
