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
	main, _, _ := strings.Cut(base, metadataSubtitleSeparator)
	return main
}

// metadataSubtitleSeparator joins a title's main part and its ～subtitle～ in the normalized
// form, so matching can compare them separately (see metadataNameMatch).
const metadataSubtitleSeparator = "\x1f"

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
	value = strings.ReplaceAll(norm.NFKC.String(value), metadataSubtitleSeparator, " ")
	// Episode markers go first so "Show 2nd Season - 01" does not read "01" as an arc name.
	for _, pattern := range metadataEpisodeSuffixes {
		value = pattern.ReplaceAllString(value, "")
	}
	// A ～subtitle～ may sit before or after the season ("無職轉生 第二季 ～副標～"), so it is
	// taken out first and kept aside.
	subtitle := ""
	if match := metadataSubtitle.FindStringSubmatchIndex(value); match != nil {
		subtitle = value[match[2]:match[3]]
		value = value[:match[0]] + " " + value[match[1]:]
	}
	// A sequel numeral written straight after the title ("無職轉生II", "無職転生Ⅱ") gets a space
	// so the season patterns read it.
	value = metadataAttachedRoman.ReplaceAllString(value, "$1 $2$3")
	season := ""
	// A season followed by an arc name ("Re：從零開始的異世界生活 第三季 襲擊篇") moves the arc into
	// the subtitle. Part and cour markers stay in the title: they name a separate entry.
	if match := metadataMidSeason.FindStringSubmatchIndex(value); match != nil {
		arc := strings.TrimSpace(value[match[1]:])
		if arc != "" && !metadataPartMarker.MatchString(arc) {
			// Group 1 is "第N季/期", group 2 is "Nth season".
			number := ""
			if match[2] >= 0 {
				number = value[match[2]:match[3]]
			} else {
				number = value[match[4]:match[5]]
			}
			if numeric, ok := seasonNumeral(number); ok {
				number = numeric
			}
			season = strings.TrimLeft(number, "0")
			value = value[:match[0]]
			if subtitle == "" {
				subtitle = arc
			}
		}
	}
	for {
		stripped := value
		for _, pattern := range metadataEpisodeSuffixes {
			stripped = pattern.ReplaceAllString(stripped, "")
		}
		if season == "" {
			season = metadataSeasonFromTitle(stripped)
		}
		for _, pattern := range metadataSeasonBeforePartSuffixes {
			stripped = pattern.ReplaceAllString(stripped, "$2")
		}
		for _, pattern := range metadataSeasonSuffixes {
			stripped = pattern.ReplaceAllString(stripped, "")
		}
		if stripped == value {
			break
		}
		value = stripped
	}
	base := normalizeMetadataText(value)
	if subtitle := normalizeMetadataText(subtitle); subtitle != "" {
		base += metadataSubtitleSeparator + subtitle
	}
	return base, season, true
}

func metadataSeasonFromTitle(value string) string {
	for _, pattern := range metadataSeasonBeforePartSuffixes {
		match := pattern.FindStringSubmatch(value)
		if len(match) == 0 {
			continue
		}
		if numeric, ok := seasonNumeral(match[1]); ok {
			return numeric
		}
		return strings.TrimLeft(match[1], "0")
	}
	for _, pattern := range metadataSeasonSuffixes {
		match := pattern.FindStringSubmatch(value)
		if len(match) == 0 {
			continue
		}
		for index := len(match) - 1; index > 0; index-- {
			if match[index] == "" {
				continue
			}
			if numeric, ok := seasonNumeral(match[index]); ok {
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
	value = FoldToSimplified(strings.ToLower(value))
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

func seasonNumeral(value string) (string, bool) {
	if value == "" {
		return "", false
	}
	switch value {
	case "貳":
		return "2", true
	case "參", "叄":
		return "3", true
	case "肆":
		return "4", true
	case "II":
		return "2", true
	case "III":
		return "3", true
	case "IV":
		return "4", true
	case "V":
		return "5", true
	case "VI":
		return "6", true
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
