// SPDX-License-Identifier: MPL-2.0

package metadata

import (
	stdhtml "html"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const (
	MaxTitleTextBytes       = 4 << 10
	MaxTitleTextRunes       = 1024
	MaxDescriptionTextBytes = 64 << 10
	MaxDescriptionTextRunes = 16384
	MaxSeasonTextBytes      = 128
	MaxSeasonTextRunes      = 128
	MaxStudioTextBytes      = 1 << 10
	MaxStudioTextRunes      = 1024
	MaxCoverURLBytes        = 8 << 10
)

type PlainTextLimits struct {
	MaxInputBytes  int
	MaxOutputBytes int
	MaxOutputRunes int
}

func TitleLimits() PlainTextLimits {
	return PlainTextLimits{MaxInputBytes: MaxTitleTextBytes, MaxOutputBytes: MaxTitleTextBytes, MaxOutputRunes: MaxTitleTextRunes}
}

func DescriptionLimits() PlainTextLimits {
	return PlainTextLimits{MaxInputBytes: MaxDescriptionTextBytes, MaxOutputBytes: MaxDescriptionTextBytes, MaxOutputRunes: MaxDescriptionTextRunes}
}

func SeasonLimits() PlainTextLimits {
	return PlainTextLimits{MaxInputBytes: MaxSeasonTextBytes, MaxOutputBytes: MaxSeasonTextBytes, MaxOutputRunes: MaxSeasonTextRunes}
}

func StudioLimits() PlainTextLimits {
	return PlainTextLimits{MaxInputBytes: MaxStudioTextBytes, MaxOutputBytes: MaxStudioTextBytes, MaxOutputRunes: MaxStudioTextRunes}
}

func NormalizePlainText(value string, limits PlainTextLimits) (string, bool) {
	if limits.MaxInputBytes <= 0 || limits.MaxOutputBytes <= 0 || limits.MaxOutputRunes <= 0 || len(value) > limits.MaxInputBytes || !utf8.ValidString(value) {
		return "", false
	}
	plain := stripHTML(stdhtml.UnescapeString(value))
	plain = stripHTML(plain)
	plain = strings.NewReplacer("<", " ", ">", " ").Replace(plain)
	plain = strings.TrimSpace(stripMarkdown(plain))
	if len(plain) > limits.MaxOutputBytes || utf8.RuneCountInString(plain) > limits.MaxOutputRunes || containsDisallowedControl(plain) {
		return "", false
	}
	return strings.Join(strings.Fields(plain), " "), true
}

func IsCanonicalPlainText(value string, limits PlainTextLimits) bool {
	normalized, ok := NormalizePlainText(value, limits)
	return ok && normalized == value
}

func IsSafeCoverURL(value string) bool {
	if len(value) > MaxCoverURLBytes || !utf8.ValidString(value) || containsAnyControl(value) {
		return false
	}
	if value == "" {
		return true
	}
	target, err := url.Parse(value)
	if err != nil || target == nil || !strings.EqualFold(target.Scheme, "https") || target.Opaque != "" || target.User != nil || target.Fragment != "" || target.Host == "" || target.Hostname() == "" || target.Port() != "" || hasEmptyPort(target.Host) {
		return false
	}
	return target.IsAbs()
}

var markdownLinkPattern = regexp.MustCompile(`\[([^\[\]\r\n]*)\]\([^()\r\n]*\)`)

func stripHTML(value string) string {
	var result strings.Builder
	tokenizer := html.NewTokenizer(strings.NewReader(value))
	skippedElement := ""
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return result.String()
		case html.StartTagToken:
			name, _ := tokenizer.TagName()
			if skippedElement == "" && (string(name) == "script" || string(name) == "style") {
				skippedElement = string(name)
			}
			result.WriteByte(' ')
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == skippedElement {
				skippedElement = ""
			}
			result.WriteByte(' ')
		case html.TextToken:
			if skippedElement == "" {
				result.Write(tokenizer.Text())
			}
		}
	}
}

func stripMarkdown(value string) string {
	value = markdownLinkPattern.ReplaceAllString(value, "$1")
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		for len(trimmed) > 0 && trimmed[0] == '#' {
			trimmed = trimmed[1:]
		}
		if len(trimmed) > 0 && (trimmed[0] == '>' || trimmed[0] == '-' || trimmed[0] == '+' || trimmed[0] == '*') {
			trimmed = trimmed[1:]
		}
		lines[index] = trimmed
	}
	value = strings.Join(lines, " ")
	var result strings.Builder
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '*', '_', '~', '`':
		case '\\':
			if index+1 < len(value) && strings.ContainsRune("\\`*_{}[]()#+-.!~", rune(value[index+1])) {
				continue
			}
			result.WriteByte(value[index])
		default:
			result.WriteByte(value[index])
		}
	}
	return result.String()
}

func containsDisallowedControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return true
		}
	}
	return false
}

func containsAnyControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func hasEmptyPort(host string) bool {
	if strings.HasPrefix(host, "[") {
		return strings.HasSuffix(host, "]:")
	}
	return strings.HasSuffix(host, ":")
}
