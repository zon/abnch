package main

import (
	"fmt"

	"github.com/zon/abnch/internal/devconfig"
)

// logout removes the stored session for the service URL. Removing a session
// that is not stored succeeds.
func logout(deps deps, serviceURL string) error {
	if err := requireServiceURL(serviceURL); err != nil {
		return err
	}

	if err := devconfig.RemoveSession(deps.homeDir, serviceURL); err != nil {
		return fmt.Errorf("failed to remove the session: %w", err)
	}
	fmt.Fprintf(deps.out, "Signed out of %s\n", serviceURL)
	return nil
}
