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

// Access is what an operation needs from the caller's roles.
type Access int

const (
	// Read reads a resource. The Reader, Service, and Admin roles may read.
	Read Access = iota
	// Write writes a resource the Admin role alone may write, such as a repo,
	// task, test, or datum.
	Write
	// WriteResult writes a result, which the Service role may also write.
	WriteResult
)

// readRoles are the roles that may read a resource. Agent and Grader reach no
// service resource; their access is to the model providers a run is configured
// with.
var readRoles = []string{RoleReader, RoleService, RoleAdmin}

// writeRoles are the roles that may write a resource only Admin writes.
var writeRoles = []string{RoleAdmin}

// resultWriteRoles are the roles that may write a result.
var resultWriteRoles = []string{RoleService, RoleAdmin}

// Authorize returns a middleware that allows a request only when the caller's
// roles permit access, and answers 403 without calling next otherwise. It
// reads the caller the authentication middleware stored, so it runs after that
// middleware.
func Authorize(access Access, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorize(w, r, access, next)
	})
}

// AuthorizeMethod returns a middleware that allows a request only when the
// caller's roles permit the access its method needs: GET and HEAD read, every
// other method writes. It authorizes an operation whose access depends on the
// method alone, such as a route that serves no resource.
func AuthorizeMethod(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorize(w, r, accessFor(r.Method), next)
	})
}

// authorize allows the request when the caller's roles permit access, and
// writes the service error response without calling next otherwise.
func authorize(w http.ResponseWriter, r *http.Request, access Access, next http.Handler) {
	caller, ok := CallerFrom(r.Context())
	if !ok || !permits(caller.Roles, access) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	next.ServeHTTP(w, r)
}

// accessFor returns the access an HTTP method needs. A method that reads with
// GET or HEAD reads; every other method writes.
func accessFor(method string) Access {
	if method == http.MethodGet || method == http.MethodHead {
		return Read
	}
	return Write
}

// accessRoles returns the roles that permit access.
func accessRoles(access Access) []string {
	switch access {
	case Write:
		return writeRoles
	case WriteResult:
		return resultWriteRoles
	default:
		return readRoles
	}
}

// permits reports whether any of the caller's roles permits access.
func permits(roles []string, access Access) bool {
	allowed := accessRoles(access)
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
