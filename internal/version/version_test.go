package version

import (
	"regexp"
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
