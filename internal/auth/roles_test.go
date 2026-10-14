package auth

import (
	"reflect"
	"testing"
)

// TestMapRoles checks a role claim is mapped through a role map, a value with
// no mapping grants no role, and a role appears once.
func TestMapRoles(t *testing.T) {
	roleMap := map[string]string{"admin": "Admin", "reader": "Reader"}

	for _, tc := range []struct {
		name  string
		claim any
		want  []string
	}{
		{name: "mapped value", claim: "admin", want: []string{"Admin"}},
		{name: "unmapped value", claim: "stranger", want: nil},
		{name: "list", claim: []any{"admin", "reader"}, want: []string{"Admin", "Reader"}},
		{name: "list skips unmapped", claim: []any{"reader", "stranger"}, want: []string{"Reader"}},
		{name: "list dedupes", claim: []any{"admin", "admin"}, want: []string{"Admin"}},
		{name: "absent value", claim: nil, want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := mapRoles(tc.claim, roleMap); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("mapRoles(%v) = %v, want %v", tc.claim, got, tc.want)
			}
		})
	}
}
