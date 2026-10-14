package store

// ValidStub reports whether a stub is non-empty and URL-safe. A URL-safe stub
// holds only unreserved URL characters, so it can stand in a path segment
// without escaping.
func ValidStub(stub string) bool {
	if stub == "" {
		return false
	}
	for _, r := range stub {
		if !unreservedRune(r) {
			return false
		}
	}
	return true
}

// unreservedRune reports whether r may appear unescaped in a URL. It is the
// unreserved set of RFC 3986.
func unreservedRune(r rune) bool {
	switch {
	case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
		return true
	case r == '-', r == '.', r == '_', r == '~':
		return true
	}
	return false
}
