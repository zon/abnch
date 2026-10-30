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
	parts := parseVersion(t, Version)

	if parts[0] == 0 && parts[1] < 2 {
		t.Errorf("Version = %q, want a minor bump past 0.1.0", Version)
	}
}

// TestVersionAdvancedForChartDelivery checks the version carries the patch bump
// that follows the Deployment release, so a later change cannot silently revert
// it below the release the chart and image tag name.
func TestVersionAdvancedForChartDelivery(t *testing.T) {
	got := parseVersion(t, Version)
	want := [3]int{0, 2, 1}

	for i := range want {
		if got[i] == want[i] {
			continue
		}
		if got[i] < want[i] {
			t.Errorf("Version = %q, want at least 0.2.1", Version)
		}
		return
	}
}

// TestVersionAdvancedForHTTPS checks the version carries the HTTPS milestone's
// major bump, so a later change cannot silently revert it below the release
// that serves HTTPS by default.
func TestVersionAdvancedForHTTPS(t *testing.T) {
	got := parseVersion(t, Version)
	want := [3]int{1, 0, 0}

	for i := range want {
		if got[i] == want[i] {
			continue
		}
		if got[i] < want[i] {
			t.Errorf("Version = %q, want at least 1.0.0", Version)
		}
		return
	}
}

// TestVersionAdvancedForRepoConfig checks the version carries the Repo Config
// milestone's minor bump, so a later change cannot silently revert it below the
// release the chart and image tag name.
func TestVersionAdvancedForRepoConfig(t *testing.T) {
	got := parseVersion(t, Version)
	want := [3]int{1, 2, 0}

	for i := range want {
		if got[i] == want[i] {
			continue
		}
		if got[i] < want[i] {
			t.Errorf("Version = %q, want at least 1.2.0", Version)
		}
		return
	}
}

// TestVersionAdvancedForResultProvenance checks the version carries the Result
// Provenance milestone's major bump, so a later change cannot silently revert it
// below the release that changes the service API and removes the run and score
// commands.
func TestVersionAdvancedForResultProvenance(t *testing.T) {
	got := parseVersion(t, Version)
	want := [3]int{2, 0, 0}

	for i := range want {
		if got[i] == want[i] {
			continue
		}
		if got[i] < want[i] {
			t.Errorf("Version = %q, want at least 2.0.0", Version)
		}
		return
	}
}

func parseVersion(t *testing.T, value string) [3]int {
	t.Helper()

	fields := strings.Split(value, ".")
	if len(fields) != 3 {
		t.Fatalf("Version = %q, want major.minor.patch", value)
	}

	var parts [3]int
	for i, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil {
			t.Fatalf("Version = %q, want numeric fields", value)
		}
		parts[i] = n
	}

	return parts
}
