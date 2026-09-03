// SPDX-License-Identifier: MPL-2.0

package metadata

import (
	"strings"
	"testing"
)

func TestNormalizePlainText(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "plain canonical", value: "Anime Title", want: "Anime Title"},
		{name: "whitespace", value: "  Anime\n\tTitle  ", want: "Anime Title"},
		{name: "HTML and Markdown", value: "# **A synopsis** <br><p>[with a link](https://example.test)</p>", want: "A synopsis with a link"},
		{name: "encoded markup", value: "Synopsis &amp;lt;b&amp;gt;text&amp;lt;/b&amp;gt;", want: "Synopsis text"},
		{name: "script content", value: "Synopsis <script>alert(1)</script> end", want: "Synopsis end"},
		{name: "unclosed tag", value: "Synopsis <img src=x", want: "Synopsis"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := NormalizePlainText(test.value, DescriptionLimits())
			if !ok || got != test.want {
				t.Fatalf("result = %q, %v, want %q, true", got, ok, test.want)
			}
		})
	}
}

func TestNormalizePlainTextRejectsInvalidAndOversizedValues(t *testing.T) {
	limits := PlainTextLimits{MaxInputBytes: 8, MaxOutputBytes: 8, MaxOutputRunes: 4}
	tests := []struct {
		name  string
		value string
		limit PlainTextLimits
	}{
		{name: "invalid limits", value: "safe", limit: PlainTextLimits{}},
		{name: "invalid UTF-8", value: string([]byte{0xff}), limit: limits},
		{name: "disallowed control", value: "a\x00b", limit: limits},
		{name: "input bytes", value: "123456789", limit: limits},
		{name: "output bytes", value: "12345", limit: PlainTextLimits{MaxInputBytes: 8, MaxOutputBytes: 4, MaxOutputRunes: 8}},
		{name: "output runes", value: "12345", limit: limits},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, ok := NormalizePlainText(test.value, test.limit); ok || got != "" {
				t.Fatalf("result = %q, %v", got, ok)
			}
		})
	}
}

func TestNormalizePlainTextAdversarialOpenDelimiters(t *testing.T) {
	value := strings.Repeat("<", 4096) + strings.Repeat("[", 4096)
	got, ok := NormalizePlainText(value, PlainTextLimits{MaxInputBytes: 16 << 10, MaxOutputBytes: 16 << 10, MaxOutputRunes: 16 << 10})
	if !ok || got == "" {
		t.Fatalf("result = %q, %v", got, ok)
	}
}

func TestIsCanonicalPlainText(t *testing.T) {
	for _, value := range []string{"", "Anime Title", "中文標題"} {
		if !IsCanonicalPlainText(value, TitleLimits()) {
			t.Fatalf("canonical value rejected: %q", value)
		}
	}
	for _, value := range []string{" Anime Title ", "Anime\nTitle", "**Anime**", "<b>Anime</b>", "a\x00b", strings.Repeat("x", TitleLimits().MaxOutputBytes+1)} {
		if IsCanonicalPlainText(value, TitleLimits()) {
			t.Fatalf("noncanonical value accepted: %q", value)
		}
	}
}

func TestFieldLimitsAreIndependentValues(t *testing.T) {
	limits := TitleLimits()
	limits.MaxInputBytes = 1
	if TitleLimits().MaxInputBytes != MaxTitleTextBytes {
		t.Fatal("title limits expose mutable policy")
	}
}

func TestIsSafeCoverURL(t *testing.T) {
	for _, value := range []string{"", "https://images.example/cover.jpg", "HTTPS://images.example/cover.png?size=large"} {
		if !IsSafeCoverURL(value) {
			t.Fatalf("safe cover URL rejected: %q", value)
		}
	}
	for _, value := range []string{
		"http://images.example/cover.jpg",
		"//images.example/cover.jpg",
		"https:opaque",
		"https://user:secret@images.example/cover.jpg",
		"https://images.example:444/cover.jpg",
		"https://images.example:/cover.jpg",
		"https://images.example/cover.jpg#fragment",
		"https://images.example/cover.jpg\nnext",
		"https:///cover.jpg",
		strings.Repeat("x", MaxCoverURLBytes+1),
	} {
		if IsSafeCoverURL(value) {
			t.Fatalf("unsafe cover URL accepted: %q", value)
		}
	}
}
