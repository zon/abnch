package auth

import "github.com/coreos/go-oidc/v3/oidc"

// mapRoles maps each value of a role claim through a role map. A value with no
// mapping grants no role, and a role appears once however many values map to
// it.
func mapRoles(claim any, roleMap map[string]string) []string {
	var roles []string
	seen := map[string]bool{}
	for _, value := range roleValues(claim) {
		role, ok := roleMap[value]
		if !ok || seen[role] {
			continue
		}
		seen[role] = true
		roles = append(roles, role)
	}
	return roles
}

// claimValue returns a token's claim value, or nil when the token carries no
// such claim.
func claimValue(token *oidc.IDToken, name string) any {
	if name == "" {
		return nil
	}
	var claims map[string]any
	if err := token.Claims(&claims); err != nil {
		return nil
	}
	return claims[name]
}

// roleValues reads a role claim as one value or a list of values.
func roleValues(claim any) []string {
	switch v := claim.(type) {
	case string:
		return []string{v}
	case []any:
		var values []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				values = append(values, s)
			}
		}
		return values
	default:
		return nil
	}
}
