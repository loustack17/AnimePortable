// SPDX-License-Identifier: MPL-2.0

package core

import "regexp"

var (
	metadataSeasonSuffixes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)[\s\p{P}]+(season|s)\s*0*([0-9]+)[\s\p{P}]*$`),
		regexp.MustCompile(`(?i)[\s\p{P}]+0*([0-9]+)(?:st|nd|rd|th)\s*season[\s\p{P}]*$`),
		regexp.MustCompile(`[\s\p{P}]+第\s*0*([0-9]+)\s*[季期][\s\p{P}]*$`),
		regexp.MustCompile(`[\s\p{P}]+第([一二三四五六七八九十]+)[季期][\s\p{P}]*$`),
		// Taiwan sequel titles: "炎炎消防隊 貳之章", "冰劍的魔術師將要統一世界 II".
		regexp.MustCompile(`[\s\p{P}]+([一二三四五六七八九十貳參叄肆]+)之章[\s\p{P}]*$`),
		// Whitespace before the numeral, so "ARC-V" is not read as season 5.
		regexp.MustCompile(`\s+(II|III|IV|V|VI)[\s\p{P}]*$`),
	}
	// NFKC has already turned a full-width ～ into ~.
	metadataSubtitle                 = regexp.MustCompile(`\s*[~〜]([^~〜]+)[~〜]\s*`)
	metadataAttachedRoman            = regexp.MustCompile(`(\p{Han})(II|III|IV|V|VI)([\s\p{P}]|$)`)
	metadataMidSeason                = regexp.MustCompile(`(?i)[\s\p{P}]+(?:第\s*([0-9]+|[一二三四五六七八九十]+)\s*[季期]|0*([0-9]+)(?:st|nd|rd|th)\s*season)[\s\p{P}]+`)
	metadataPartMarker               = regexp.MustCompile(`(?i)part|cour|部分|クール|[前後]半|[前後][篇編]`)
	metadataSeasonBeforePartSuffixes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)[\s\p{P}]+(?:season|s)\s*0*([0-9]+)([\s\p{P}]+(?:part|cour)\b)`),
		regexp.MustCompile(`(?i)[\s\p{P}]+0*([0-9]+)(?:st|nd|rd|th)\s*season([\s\p{P}]+(?:part|cour)\b)`),
		regexp.MustCompile(`(?i)[\s\p{P}]+第\s*0*([0-9]+)\s*[季期]([\s\p{P}]+(?:part|cour)\b)`),
		regexp.MustCompile(`(?i)[\s\p{P}]+第([一二三四五六七八九十]+)[季期]([\s\p{P}]+(?:part|cour)\b)`),
	}
	metadataEpisodeSuffixes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)[\s\p{P}]+(episode|ep|#)\s*0*[0-9]+[\s\p{P}]*$`),
		regexp.MustCompile(`[\s\p{P}]+第\s*0*[0-9]+\s*[話集][\s\p{P}]*$`),
		regexp.MustCompile(`[\s\p{P}]+0*[0-9]+\s*[話集][\s\p{P}]*$`),
		regexp.MustCompile(`[\s]*[-–—_:：]\s*0*[0-9]{1,4}[\s\p{P}]*$`),
	}
)
