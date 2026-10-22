// SPDX-License-Identifier: MPL-2.0

package release

import "testing"

func TestPageURL(t *testing.T) {
	const base = "https://github.com/loustack17/AnimePortable/releases/"
	accepted := map[string]string{
		"release page":     base + "tag/v0.3.0-alpha.1",
		"releases root":    base,
		"tilde and dot":    base + "tag/v1.0.0~rc.1",
		"upper-case https": "HTTPS://github.com/loustack17/AnimePortable/releases/tag/v1",
	}
	for name, raw := range accepted {
		got, ok := PageURL(raw)
		if !ok {
			t.Errorf("%s: %q rejected", name, raw)
			continue
		}
		// What comes back is the re-serialised URL, which parses to itself.
		if again, ok := PageURL(got); !ok || again != got {
			t.Errorf("%s: %q is not stable (%q, %v)", name, got, again, ok)
		}
	}
	if got, _ := PageURL(accepted["upper-case https"]); got != "https://github.com/loustack17/AnimePortable/releases/tag/v1" {
		t.Errorf("scheme not normalised: %q", got)
	}
	rejected := map[string]string{
		"empty":             "",
		"http":              "http://github.com/loustack17/AnimePortable/releases/tag/v1",
		"other host":        "https://evil.example/loustack17/AnimePortable/releases/tag/v1",
		"look-alike host":   "https://github.com.evil.example/loustack17/AnimePortable/releases/",
		"upper-case host":   "https://GitHub.com/loustack17/AnimePortable/releases/tag/v1",
		"sub-domain":        "https://www.github.com/loustack17/AnimePortable/releases/",
		"other repository":  "https://github.com/someone/else/releases/tag/v1",
		"other path":        "https://github.com/loustack17/AnimePortable/issues/1",
		"prefix only":       "https://github.com/loustack17/AnimePortable/releases",
		"double quote":      base + `tag/v1"`,
		"escaped quote":     base + "tag/v1%22",
		"escaped dot":       base + "tag/%2e%2e/x",
		"escaped letter":    base + "tag/%41",
		"dot dot":           base + "tag/../../x",
		"dot dot start":     "https://github.com/loustack17/AnimePortable/releases/../x",
		"query":             base + "tag/v1?x=1",
		"empty query":       base + "tag/v1?",
		"fragment":          base + "tag/v1#x",
		"user information":  "https://user@github.com/loustack17/AnimePortable/releases/",
		"port":              "https://github.com:443/loustack17/AnimePortable/releases/",
		"backslash":         base + `tag\v1`,
		"space":             base + "tag/v1 --x",
		"control character": base + "tag/v1\n",
		"semicolon":         base + "tag/v1;calc",
		"opaque":            "https:github.com/loustack17/AnimePortable/releases/",
		"file":              "file:///C:/Windows/System32/calc.exe",
	}
	for name, raw := range rejected {
		if got, ok := PageURL(raw); ok {
			t.Errorf("%s: %q accepted as %q", name, raw, got)
		}
	}
}
