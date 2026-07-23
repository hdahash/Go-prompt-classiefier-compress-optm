package optimizer

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var normalizeSpaceRe = regexp.MustCompile(`\s+`)

// CacheKey returns a stable cache key for a prompt bound to a given tier.
// The prompt is whitespace-normalized and lowercased before hashing so
// trivial formatting differences (extra spaces, casing) don't cause cache
// misses on otherwise-identical requests.
func CacheKey(tier string, prompt string) string {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	normalized = normalizeSpaceRe.ReplaceAllString(normalized, " ")
	sum := sha256.Sum256([]byte(tier + "|" + normalized))
	return hex.EncodeToString(sum[:])
}
