package cache

import (
	"strings"
)

// Key builds a consistent cache key by joining a prefix and parts with colons.
// This ensures cache keys follow a consistent naming convention across the application.
//
// Example:
//
//	key := cache.Key("user", userID)                    // "user:123"
//	key := cache.Key("portfolio", portfolioID, "stats") // "portfolio:abc:stats"
//
// Empty parts are filtered out to prevent double colons.
func Key(prefix string, parts ...string) string {
	// Pre-allocate with capacity for all parts
	filtered := make([]string, 0, len(parts)+1)

	if prefix != "" {
		filtered = append(filtered, prefix)
	}

	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}

	return strings.Join(filtered, ":")
}
