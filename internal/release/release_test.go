// SPDX-License-Identifier: MPL-2.0

package release

import "testing"

func TestVersionIsParseable(t *testing.T) {
	if _, ok := Parse(Version); !ok {
		t.Fatalf("Version %q does not parse", Version)
	}
}

func TestCompare(t *testing.T) {
	ordered := []string{"v0.1.0-alpha.1", "v0.1.0-alpha.2", "v0.1.0-alpha.10", "v0.1.0-beta", "v0.1.0", "v0.2.0-alpha", "0.2.0-beta", "v0.2.1-alpha", "v0.3.0-alpha", "v0.10.0-alpha", "v1.0.0"}
	for index, left := range ordered {
		for other, right := range ordered {
			l, lok := Parse(left)
			r, rok := Parse(right)
			if !lok || !rok {
				t.Fatalf("parse %q %q", left, right)
			}
			want := compareInt(index, other)
			if got := l.Compare(r); got != want {
				t.Errorf("Compare(%q, %q) = %d, want %d", left, right, got, want)
			}
		}
	}
	a, _ := Parse("v0.2.0-alpha")
	b, _ := Parse("0.2.0-alpha")
	if a.Compare(b) != 0 {
		t.Error("the v prefix changes the order")
	}
}

func TestParseRejects(t *testing.T) {
	for _, value := range []string{"", "v", "latest", "v0.2", "v0.2.0.1", "v0.2.x", "v0.2.0-", "v0.2.0-alpha..1", "v0.2.0-al pha", "v0.2.-1", "v9999999999.0.0", "vv0.2.0"} {
		if _, ok := Parse(value); ok {
			t.Errorf("Parse(%q) accepted", value)
		}
	}
}

func TestParsedStringDropsTheTagPrefixAndKeepsTheStage(t *testing.T) {
	for raw, want := range map[string]string{"v0.3.0-alpha": "0.3.0-alpha", "1.2.3": "1.2.3", " v0.1.0-alpha.2 ": "0.1.0-alpha.2", "v10.0.0-rc-1": "10.0.0-rc-1"} {
		parsed, ok := Parse(raw)
		if !ok || parsed.String() != want {
			t.Errorf("Parse(%q).String() = %q, %v; want %q", raw, parsed.String(), ok, want)
		}
	}
}
