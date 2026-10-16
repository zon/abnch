package version

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestVersionIsSemantic checks the version names a semantic version, so a
// consumer can parse and compare it.
func TestVersionIsSemantic(t *testing.T) {
	semantic := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	if !semantic.MatchString(Version) {
		t.Errorf("Version = %q, want a semantic version such as 1.2.3", Version)
	}
}

// TestVersionAdvancedForDeployment checks the version carries the Deployment
// milestone's minor bump, so a later change cannot silently revert it below the
// release the chart and image tag name.
func TestVersionAdvancedForDeployment(t *testing.T) {
	fields := strings.Split(Version, ".")
	if len(fields) != 3 {
		t.Fatalf("Version = %q, want major.minor.patch", Version)
	}
	major, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatalf("Version = %q, want a numeric major", Version)
	}
	minor, err := strconv.Atoi(fields[1])
	if err != nil {
		t.Fatalf("Version = %q, want a numeric minor", Version)
	}

	if major == 0 && minor < 2 {
		t.Errorf("Version = %q, want a minor bump past 0.1.0", Version)
	}
}
