package identifier

import (
	"regexp"
	"strconv"
	"strings"
)

// MaxSpaceKeyLength is the longest short key a Space may have.
const MaxSpaceKeyLength = 16

// SpaceKeyMatcher matches a Space's short key: lowercase letters, digits and hyphens,
// starting and ending with a letter or digit.
var SpaceKeyMatcher = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,14}[a-z0-9])?$`)

var nonKeyCharacters = regexp.MustCompile(`[^a-z0-9]+`)

// FormatHandle returns a memo's human-readable handle, such as "haymakr-12".
func FormatHandle(spaceKey string, seq int32) string {
	return spaceKey + "-" + strconv.FormatInt(int64(seq), 10)
}

// ParseHandle splits a handle into its Space key and number. The number follows the last hyphen,
// so keys may contain hyphens themselves. The key is case-folded, so "Haymakr-12" finds "haymakr-12".
func ParseHandle(handle string) (spaceKey string, seq int32, ok bool) {
	handle = strings.ToLower(strings.TrimSpace(handle))
	i := strings.LastIndex(handle, "-")
	if i <= 0 || i == len(handle)-1 {
		return "", 0, false
	}
	n, err := strconv.ParseInt(handle[i+1:], 10, 32)
	if err != nil || n <= 0 || !SpaceKeyMatcher.MatchString(handle[:i]) {
		return "", 0, false
	}
	return handle[:i], int32(n), true
}

// DeriveSpaceKey proposes a short key for a new Space. A short uid is already one; otherwise the
// title is reduced to lowercase words, with the first sixteen characters kept. The result is not
// guaranteed unique, and falls back to "space" when nothing usable remains.
func DeriveSpaceKey(uid, title string) string {
	if key := strings.ToLower(uid); SpaceKeyMatcher.MatchString(key) {
		return key
	}
	slug := strings.Trim(nonKeyCharacters.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if len(slug) > MaxSpaceKeyLength {
		slug = strings.Trim(slug[:MaxSpaceKeyLength], "-")
	}
	if slug == "" {
		return "space"
	}
	return slug
}
