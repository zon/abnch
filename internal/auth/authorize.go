package auth

import (
	"net/http"

	"github.com/zon/abnch/internal/httpx"
)

// Role names a role the service grants an authenticated caller.
const (
	RoleReader  = "Reader"
	RoleService = "Service"
	RoleAgent   = "Agent"
	RoleGrader  = "Grader"
	RoleAdmin   = "Admin"
)

// Access is what a request needs from the caller's roles.
type Access int

const (
	// Read reads a resource.
	Read Access = iota
	// Write writes a resource.
	Write
)

// readRoles are the roles that may read a resource. Agent and Grader reach no
// service resource; their access is to the model providers a run is configured
// with.
var readRoles = []string{RoleReader, RoleService, RoleAdmin}

// writeRoles are the roles that may write a resource.
var writeRoles = []string{RoleAdmin}

// Authorize returns a middleware that allows a request only when the caller's
// roles permit the access the request needs, and answers 403 without calling
// next otherwise. It reads the caller the authentication middleware stored, so
// it runs after that middleware.
func Authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller, ok := CallerFrom(r.Context())
		if !ok || !permits(caller.Roles, accessFor(r.Method)) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// accessFor returns the access an HTTP method needs. A method that reads with
// GET or HEAD reads; every other method writes.
func accessFor(method string) Access {
	if method == http.MethodGet || method == http.MethodHead {
		return Read
	}
	return Write
}

// permits reports whether any of the caller's roles permits access.
func permits(roles []string, access Access) bool {
	allowed := readRoles
	if access == Write {
		allowed = writeRoles
	}
	for _, role := range roles {
		if contains(allowed, role) {
			return true
		}
	}
	return false
}

// contains reports whether values includes value.
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
