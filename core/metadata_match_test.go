// SPDX-License-Identifier: MPL-2.0

package core

import (
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
	if result.Decision != MetadataMatchNoMetadata || result.Candidate != (MetadataCandidate{}) {
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
			{Ref: MetadataRef{Provider: "bangumi", ID: "fallback"}, Title: "Great Anime"},
			{Ref: MetadataRef{Provider: "anilist", ID: "primary"}, Title: "Great Anime"},
		},
	})
	if result.Decision != MetadataMatchAccepted || result.Confidence != MetadataMatchConfidenceMedium || !result.CrossChecked {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Candidate.Ref.Provider != "anilist" {
		t.Fatalf("selected provider = %q, want anilist", result.Candidate.Ref.Provider)
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
	if result.Candidate != (MetadataCandidate{}) {
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
	if result.Candidate != (MetadataCandidate{}) {
		t.Fatalf("conflicting candidate leaked: %#v", result.Candidate)
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
		if result.Decision != MetadataMatchNoMetadata || result.Confidence != MetadataMatchConfidenceLow || result.Candidate != (MetadataCandidate{}) {
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
