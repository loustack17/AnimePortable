// SPDX-License-Identifier: MPL-2.0

package release

import (
	"net/url"
	"strings"
)

const (
	pageHost       = "github.com"
	pagePathPrefix = "/loustack17/AnimePortable/releases/"
)

// PageURL checks a release page link and returns it re-serialised. Only an https page under
// this project's releases passes: host exactly github.com, a path of plain characters, and
// no query, fragment, user information, port or ".." segment. Callers use the returned
// value, never the input, so nothing a browser or shell could read differently is passed on.
func PageURL(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host != pageHost || parsed.User != nil || parsed.Opaque != "" {
		return "", false
	}
	// RawPath is set only when the path holds percent-escapes, which no release link needs.
	if parsed.RawPath != "" || strings.ContainsAny(raw, "?#") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	if !strings.HasPrefix(parsed.Path, pagePathPrefix) || strings.Contains(parsed.Path, "..") || !plainPath(parsed.Path) {
		return "", false
	}
	return parsed.String(), true
}

func plainPath(path string) bool {
	for _, character := range path {
		switch {
		case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z', character >= '0' && character <= '9':
		case character == '.', character == '_', character == '~', character == '/', character == '-':
		default:
			return false
		}
	}
	return true
}
