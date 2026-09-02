// SPDX-License-Identifier: MPL-2.0

package core

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

func NormalizeMetadataTitle(value string) string {
	base, _, ok := metadataTitleParts(value)
	if !ok {
		return ""
	}
	return base
}

func metadataFields(title, nativeTitle, season string) (string, string, string) {
	titleBase, titleSeason, titleOK := metadataTitleParts(title)
	nativeBase, nativeSeason, nativeOK := metadataTitleParts(nativeTitle)
	seasonHint, seasonOK := normalizeMetadataSeason(season)
	if !titleOK || !nativeOK || !seasonOK {
		return "", "", ""
	}
	if seasonHint == "" {
		seasonHint = titleSeason
	}
	if seasonHint == "" {
		seasonHint = nativeSeason
	}
	return titleBase, nativeBase, seasonHint
}

func metadataTitleParts(value string) (string, string, bool) {
	if value == "" {
		return "", "", true
	}
	if len(value) > metadataMaxTitleBytes || !utf8.ValidString(value) || utf8.RuneCountInString(value) > metadataMaxTitleRunes {
		return "", "", false
	}
	value = norm.NFKC.String(value)
	season := ""
	for {
		stripped := value
		for _, pattern := range metadataEpisodeSuffixes {
			stripped = pattern.ReplaceAllString(stripped, "")
		}
		if season == "" {
			season = metadataSeasonFromTitle(stripped)
		}
		for _, pattern := range metadataSeasonSuffixes {
			stripped = pattern.ReplaceAllString(stripped, "")
		}
		if stripped == value {
			break
		}
		value = stripped
	}
	return normalizeMetadataText(value), season, true
}

func metadataSeasonFromTitle(value string) string {
	for _, pattern := range metadataSeasonSuffixes {
		match := pattern.FindStringSubmatch(value)
		if len(match) == 0 {
			continue
		}
		for index := len(match) - 1; index > 0; index-- {
			if match[index] == "" {
				continue
			}
			if numeric, ok := chineseNumeral(match[index]); ok {
				return numeric
			}
			return strings.TrimLeft(match[index], "0")
		}
	}
	return ""
}

func normalizeMetadataSeason(value string) (string, bool) {
	if value == "" {
		return "", true
	}
	if len(value) > metadataMaxSeasonBytes || !utf8.ValidString(value) || utf8.RuneCountInString(value) > metadataMaxSeasonRunes {
		return "", false
	}
	value = norm.NFKC.String(strings.ToLower(value))
	if season := metadataSeasonFromTitle(value); season != "" {
		return season, true
	}
	return normalizeMetadataText(value), true
}

func normalizeMetadataText(value string) string {
	value = metadataTraditionalSimplified.Replace(norm.NFKC.String(strings.ToLower(value)))
	var normalized strings.Builder
	space := false
	for _, character := range value {
		switch {
		case unicode.IsSpace(character), unicode.IsPunct(character), unicode.IsSymbol(character):
			space = normalized.Len() > 0
		default:
			if space {
				normalized.WriteByte(' ')
			}
			normalized.WriteRune(character)
			space = false
		}
	}
	return strings.TrimSpace(normalized.String())
}

func chineseNumeral(value string) (string, bool) {
	if value == "" {
		return "", false
	}
	if len([]rune(value)) == 1 {
		for numeric, character := range []rune("一二三四五六七八九") {
			if value == string(character) {
				return string(rune('1' + numeric)), true
			}
		}
		if value == "十" {
			return "10", true
		}
	}
	if value == "十一" {
		return "11", true
	}
	if value == "十二" {
		return "12", true
	}
	return "", false
}
