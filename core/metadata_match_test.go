// SPDX-License-Identifier: MPL-2.0

package core

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeMetadataTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "script and punctuation", input: "藥師少女的獨語！", want: "药师少女的独语"},
		{name: "season and episode suffixes", input: "[葬送的芙莉蓮] Season 2 - 01", want: "葬送的芙莉莲"},
		{name: "full width", input: "Ａｎｉｍｅ：第０２話", want: "anime"},
		{name: "japanese season suffix", input: "Show 第2期", want: "show"},
		{name: "part remains part of title", input: "Show Season 4 Part 2", want: "show part 2"},
		{name: "cour remains part of title", input: "Show Season 4 Cour 2", want: "show cour 2"},
		{name: "ordinal season before part", input: "Show 4th Season Part 2", want: "show part 2"},
		{name: "japanese season before cour", input: "Show 第2季 Cour 2", want: "show cour 2"},
		{name: "episode marker", input: "進擊の巨人 #12", want: "进击の巨人"},
		{name: "invalid utf8", input: string([]byte{0xff}), want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeMetadataTitle(test.input); got != test.want {
				t.Fatalf("NormalizeMetadataTitle(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestMatchMetadataAcceptsBestCandidateInsteadOfFirst(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{
		Query: MetadataQuery{Title: "葬送的芙莉莲", Year: 2023},
		Candidates: []MetadataCandidate{
			{Ref: MetadataRef{Provider: "anilist", ID: "wrong"}, Title: "葬送的芙莉蓮 第二季", Year: 2024},
			{Ref: MetadataRef{Provider: "anilist", ID: "right"}, Title: "葬送的芙莉蓮", Year: 2023},
		},
	})
	if result.Decision != MetadataMatchAccepted || result.Confidence != MetadataMatchConfidenceHigh {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Candidate.Ref.ID != "right" {
		t.Fatalf("selected candidate = %#v, want right", result.Candidate.Ref)
	}
}

func TestMatchMetadataUsesNativeTitle(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{
		Query: MetadataQuery{Title: "藥師少女的獨語", NativeTitle: "薬屋のひとりごと"},
		Candidates: []MetadataCandidate{
			{Ref: MetadataRef{Provider: "bangumi", ID: "right"}, Title: "薬屋のひとりごと", NativeTitle: "薬屋のひとりごと"},
		},
	})
	if result.Decision != MetadataMatchAccepted || result.Candidate.Ref.ID != "right" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestMatchMetadataSeasonSuffixesMatch(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{
		Query: MetadataQuery{Title: "Show Season 2"},
		Candidates: []MetadataCandidate{
			{Ref: MetadataRef{Provider: "anilist", ID: "show-2"}, Title: "Show S2"},
		},
	})
	if result.Decision != MetadataMatchAccepted || result.Candidate.Ref.ID != "show-2" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestMatchMetadataSeasonAndEpisodeSuffixesMatch(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{
		Query: MetadataQuery{Title: "Show Season 2 Episode 1"},
		Candidates: []MetadataCandidate{
			{Ref: MetadataRef{Provider: "anilist", ID: "show-2"}, Title: "Show S2", Season: "2"},
		},
	})
	if result.Decision != MetadataMatchAccepted || result.Candidate.Ref.ID != "show-2" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestMatchMetadataKeepsPartIdentity(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{
		Query: MetadataQuery{Title: "Show Season 4 Part 2"},
		Candidates: []MetadataCandidate{
			{Ref: MetadataRef{Provider: "anilist", ID: "show-2"}, Title: "Show Season 2"},
		},
	})
	if result.Decision != MetadataMatchNoMetadata || !reflect.DeepEqual(result.Candidate, MetadataCandidate{}) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestMatchMetadataExtractsSeasonBeforePart(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{
		Query: MetadataQuery{Title: "Show Season 4 Part 2"},
		Candidates: []MetadataCandidate{
			{Ref: MetadataRef{Provider: "anilist", ID: "show-part-2"}, Title: "Show Part 2", Season: "4"},
		},
	})
	if result.Decision != MetadataMatchAccepted || result.Confidence != MetadataMatchConfidenceHigh || result.Candidate.Ref.ID != "show-part-2" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestMatchMetadataRequiresCrossCheckForMediumConfidence(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{
		Query: MetadataQuery{Title: "My Great Anime"},
		Candidates: []MetadataCandidate{
			{Ref: MetadataRef{Provider: "anilist", ID: "partial"}, Title: "Great Anime"},
		},
	})
	if result.Decision != MetadataMatchNeedsCrossCheck || result.Confidence != MetadataMatchConfidenceMedium {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Candidate.Ref.ID != "partial" || result.CrossChecked {
		t.Fatalf("unexpected candidate state: %#v", result)
	}
}

func TestMatchMetadataAcceptsMediumConfidenceAfterProviderCrossCheck(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{
		Query: MetadataQuery{Title: "My Great Anime"},
		Candidates: []MetadataCandidate{
			{Ref: MetadataRef{Provider: "anilist", ID: "cross-check"}, Title: "Great Anime"},
			{Ref: MetadataRef{Provider: "bangumi", ID: "primary"}, Title: "Great Anime"},
		},
	})
	if result.Decision != MetadataMatchAccepted || result.Confidence != MetadataMatchConfidenceMedium || !result.CrossChecked {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Candidate.Ref.Provider != "bangumi" {
		t.Fatalf("selected provider = %q, want bangumi (ADR-009: Chinese synopsis first)", result.Candidate.Ref.Provider)
	}
}

func TestMatchMetadataFailsClosedForLowConfidence(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{
		Query: MetadataQuery{Title: "Unique Anime"},
		Candidates: []MetadataCandidate{
			{Ref: MetadataRef{Provider: "anilist", ID: "wrong"}, Title: "Unrelated Work"},
		},
	})
	if result.Decision != MetadataMatchNoMetadata || result.Confidence != MetadataMatchConfidenceLow {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !reflect.DeepEqual(result.Candidate, MetadataCandidate{}) {
		t.Fatalf("low-confidence candidate leaked: %#v", result.Candidate)
	}
}

func TestMatchMetadataDoesNotPromoteNearTitleMatches(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{
		Query: MetadataQuery{Title: "Extraordinary Anime Show", Year: 2024},
		Candidates: []MetadataCandidate{
			{Ref: MetadataRef{Provider: "anilist", ID: "near"}, Title: "Extraordinary Anime Shiw", Year: 2024},
		},
	})
	if result.Decision != MetadataMatchNoMetadata || result.Confidence != MetadataMatchConfidenceLow {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestMatchMetadataFailsClosedForConflictingCandidates(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{
		Query: MetadataQuery{Title: "Anime", Year: 2024},
		Candidates: []MetadataCandidate{
			{Ref: MetadataRef{Provider: "bangumi", ID: "later"}, Title: "Anime", Year: 2025},
			{Ref: MetadataRef{Provider: "anilist", ID: "earlier"}, Title: "Anime", Year: 2023},
		},
	})
	if result.Decision != MetadataMatchNoMetadata || result.Confidence != MetadataMatchConfidenceLow {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !reflect.DeepEqual(result.Candidate, MetadataCandidate{}) {
		t.Fatalf("conflicting candidate leaked: %#v", result.Candidate)
	}
}

func TestMatchMetadataFailsClosedForDistinctSameProviderIdentities(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{
		Query: MetadataQuery{Title: "Anime"},
		Candidates: []MetadataCandidate{
			{Ref: MetadataRef{Provider: "anilist", ID: "2"}, Title: "Anime"},
			{Ref: MetadataRef{Provider: "anilist", ID: "1"}, Title: "Anime"},
		},
	})
	if result.Decision != MetadataMatchNoMetadata || result.Confidence != MetadataMatchConfidenceLow || !reflect.DeepEqual(result.Candidate, MetadataCandidate{}) {
		t.Fatalf("ambiguous identity accepted: %#v", result)
	}
}

func TestMatchMetadataRejectsInvalidAndEmptyInputs(t *testing.T) {
	tests := []MetadataMatchRequest{
		{},
		{Query: MetadataQuery{Title: "Anime"}},
		{Query: MetadataQuery{Title: "Anime"}, Candidates: []MetadataCandidate{{Title: "Anime"}}},
		{Query: MetadataQuery{Title: strings.Repeat("a", metadataMaxTitleBytes+1)}, Candidates: []MetadataCandidate{{Ref: MetadataRef{Provider: "anilist", ID: "too-long"}, Title: "Anime"}}},
		{Query: MetadataQuery{Title: "Anime"}, Candidates: makeMetadataCandidates(metadataMaxCandidates + 1)},
		{Query: MetadataQuery{Title: "Anime", Season: strings.Repeat("1", metadataMaxSeasonBytes+1)}, Candidates: []MetadataCandidate{{Ref: MetadataRef{Provider: "anilist", ID: "too-long-season"}, Title: "Anime"}}},
		{Query: MetadataQuery{Title: "Anime"}, Candidates: []MetadataCandidate{{Ref: MetadataRef{Provider: "anilist", ID: "invalid-season"}, Title: "Anime", Season: string([]byte{0xff})}}},
	}
	for index, request := range tests {
		result := MatchMetadata(request)
		if result.Decision != MetadataMatchNoMetadata || result.Confidence != MetadataMatchConfidenceLow || !reflect.DeepEqual(result.Candidate, MetadataCandidate{}) {
			t.Fatalf("case %d returned unsafe result: %#v", index, result)
		}
	}
}

func makeMetadataCandidates(count int) []MetadataCandidate {
	candidates := make([]MetadataCandidate, count)
	for index := range candidates {
		candidates[index] = MetadataCandidate{Ref: MetadataRef{Provider: "anilist", ID: string(rune(index + 1))}, Title: "Anime"}
	}
	return candidates
}

func TestMetadataMatchReadsTaiwanSeasonNotationAndAliases(t *testing.T) {
	for _, test := range []struct {
		name      string
		query     MetadataQuery
		candidate MetadataCandidate
		other     MetadataCandidate
	}{
		{"之章 with 貳 and 叄", MetadataQuery{Title: "炎炎消防隊 參之章(第三季)"},
			MetadataCandidate{Ref: MetadataRef{Provider: "bangumi", ID: "3"}, Title: "炎炎消防隊 叄之章"},
			MetadataCandidate{Ref: MetadataRef{Provider: "bangumi", ID: "1"}, Title: "炎炎消防隊"}},
		{"roman sequel", MetadataQuery{Title: "冰劍的魔術師將要統一世界 II(第二季)"},
			MetadataCandidate{Ref: MetadataRef{Provider: "bangumi", ID: "2"}, Title: "冰劍的魔術師將要統一世界 第二季"},
			MetadataCandidate{Ref: MetadataRef{Provider: "bangumi", ID: "1"}, Title: "冰劍的魔術師將要統一世界"}},
		{"Taiwan alias, spacing and standalone x", MetadataQuery{Title: "SPY×FAMILY間諜家家酒 第三季"},
			MetadataCandidate{Ref: MetadataRef{Provider: "bangumi", ID: "3"}, Title: "間諜過家家 第三季", Aliases: []string{"SPY x FAMILY 間諜家家酒 Season 3"}},
			MetadataCandidate{Ref: MetadataRef{Provider: "bangumi", ID: "1"}, Title: "間諜過家家", Aliases: []string{"SPY×FAMILY間諜家家酒"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := MatchMetadata(MetadataMatchRequest{Query: test.query, Candidates: []MetadataCandidate{test.other, test.candidate}})
			if result.Decision != MetadataMatchAccepted || result.Candidate.Ref != test.candidate.Ref {
				t.Fatalf("decision=%d picked %+v, want %+v", result.Decision, result.Candidate.Ref, test.candidate.Ref)
			}
		})
	}
}

func TestMetadataQueryUsesBracketedNameButNotSeasonMarker(t *testing.T) {
	if query := metadataQueryFor(Anime{Title: "RELEASE THE SPYCE (月影特工)"}); query.Title != "RELEASE THE SPYCE" || query.NativeTitle != "月影特工" {
		t.Fatalf("query = %+v", query)
	}
	for _, title := range []string{"炎炎消防隊 貳之章(第二季)", "某作品 (劇場版)", "某作品 (OVA)", "Some Anime (2023)"} {
		if query := metadataQueryFor(Anime{Title: title}); query.Title != title || query.NativeTitle != "" {
			t.Fatalf("%q split into %+v", title, query)
		}
	}
}

func TestMetadataMatchDoesNotOverreadSequelMarkersOrAliases(t *testing.T) {
	gundam := MatchMetadata(MetadataMatchRequest{Query: MetadataQuery{Title: "Mobile Suit Gundam X"}, Candidates: []MetadataCandidate{{Ref: MetadataRef{Provider: "anilist", ID: "1"}, Title: "Mobile Suit Gundam"}}})
	if gundam.Score == metadataHighScore {
		t.Fatalf("Gundam X matched Gundam exactly: %+v", gundam)
	}
	if _, season, _ := metadataTitleParts("Yu-Gi-Oh! ARC-V"); season != "" {
		t.Fatalf("ARC-V read as season %q", season)
	}
	alias := MatchMetadata(MetadataMatchRequest{Query: MetadataQuery{Title: "Great Anime Movie Edition"}, Candidates: []MetadataCandidate{{Ref: MetadataRef{Provider: "bangumi", ID: "1"}, Title: "Other", Aliases: []string{"Great Anime"}}}})
	if alias.Decision != MetadataMatchNoMetadata {
		t.Fatalf("alias substring matched: %+v", alias)
	}
}

func TestMetadataTitlePartsReadSubtitlesMidSeasonsAndAttachedNumerals(t *testing.T) {
	for _, test := range []struct{ input, base, season string }{
		{"無職轉生II～到了異世界就拿出真本事～(第二季)", "无职转生" + metadataSubtitleSeparator + "到了异世界就拿出真本事", "2"},
		{"無職轉生 第二季 ～到了異世界就拿出真本事～", "无职转生" + metadataSubtitleSeparator + "到了异世界就拿出真本事", "2"},
		{"無職転生Ⅱ ～異世界行ったら本気だす～", "无职転生" + metadataSubtitleSeparator + "异世界行ったら本気だす", "2"},
		{"Re：從零開始的異世界生活 第三季 襲擊篇", "re 从零开始的异世界生活" + metadataSubtitleSeparator + "袭击篇", "3"},
		{"Re:ゼロから始める異世界生活 3rd season 襲擊編", "re ゼロから始める异世界生活" + metadataSubtitleSeparator + "袭击编", "3"},
		{"無職轉生 第二季 ～到了異世界就拿出真本事～ 第2部分", "无职转生 第二季 第2部分" + metadataSubtitleSeparator + "到了异世界就拿出真本事", ""},
		{"咒術迴戰", "咒术回战", ""},
	} {
		base, season, ok := metadataTitleParts(test.input)
		if !ok || base != test.base || season != test.season {
			t.Errorf("%q => base %q season %q, want %q season %q", test.input, base, season, test.base, test.season)
		}
	}
}

func TestMetadataMatchTakesFirstArcOfSameSeasonButKeepsDifferentSubtitlesApart(t *testing.T) {
	arcs := MatchMetadata(MetadataMatchRequest{Query: MetadataQuery{Title: "Re:從零開始的異世界生活 第三季"}, Candidates: []MetadataCandidate{
		{Ref: MetadataRef{Provider: "bangumi", ID: "1"}, Title: "Re：從零開始的異世界生活 第三季 襲擊篇"},
		{Ref: MetadataRef{Provider: "bangumi", ID: "2"}, Title: "Re：從零開始的異世界生活 第三季 反擊篇"},
		{Ref: MetadataRef{Provider: "bangumi", ID: "3"}, Title: "Re：從零開始的異世界生活 第四季 喪失篇"},
	}})
	if arcs.Decision != MetadataMatchAccepted || arcs.Candidate.Ref.ID != "1" {
		t.Fatalf("arc match = %+v", arcs)
	}
	other := MatchMetadata(MetadataMatchRequest{Query: MetadataQuery{Title: "某作品 ～甲篇～"}, Candidates: []MetadataCandidate{
		{Ref: MetadataRef{Provider: "bangumi", ID: "1"}, Title: "某作品 ～乙篇～"},
	}})
	if other.Decision == MetadataMatchAccepted {
		t.Fatalf("different subtitle accepted: %+v", other)
	}
}

func TestMetadataMatchReviewEdgeCases(t *testing.T) {
	differentSubtitles := MatchMetadata(MetadataMatchRequest{Query: MetadataQuery{Title: "某作品 第二季 ～甲篇～"}, Candidates: []MetadataCandidate{
		{Ref: MetadataRef{Provider: "bangumi", ID: "1"}, Title: "某作品 第二季 ～乙篇～"},
		{Ref: MetadataRef{Provider: "anilist", ID: "2"}, Title: "某作品 第二季 ～乙篇～"},
	}})
	if differentSubtitles.Decision != MetadataMatchNoMetadata {
		t.Fatalf("different subtitles accepted with a season match and cross-check: %+v", differentSubtitles)
	}
	if base, season, _ := metadataTitleParts("Show 2nd Season - 01"); base != "show" || season != "2" {
		t.Fatalf("episode marker read as arc: base %q season %q", base, season)
	}
	recap := MatchMetadata(MetadataMatchRequest{Query: MetadataQuery{Title: "某作品 第二季"}, Candidates: []MetadataCandidate{
		{Ref: MetadataRef{Provider: "bangumi", ID: "1"}, Title: "某作品 第二季 本篇"},
		{Ref: MetadataRef{Provider: "bangumi", ID: "2"}, Title: "某作品 第二季 總集篇"},
	}})
	if recap.Decision == MetadataMatchAccepted {
		t.Fatalf("recap arc treated as a season arc: %+v", recap)
	}
}

func TestMetadataMatchAcceptsColonOrTildeSubtitle(t *testing.T) {
	result := MatchMetadata(MetadataMatchRequest{Query: MetadataQuery{Title: "無職英雄：技能什麼的毫無用處"}, Candidates: []MetadataCandidate{
		{Ref: MetadataRef{Provider: "bangumi", ID: "1"}, Title: "無職英雄 ～技能什麼的毫無用處～"},
	}})
	if result.Decision != MetadataMatchAccepted || result.Score != metadataHighScore {
		t.Fatalf("colon subtitle = %+v", result)
	}
}
