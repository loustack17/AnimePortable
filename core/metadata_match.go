// SPDX-License-Identifier: MPL-2.0

package core

import (
	"sort"
	"strings"
)

type MetadataMatchConfidence uint8

const (
	MetadataMatchConfidenceLow MetadataMatchConfidence = iota
	MetadataMatchConfidenceMedium
	MetadataMatchConfidenceHigh
)

type MetadataMatchDecision uint8

const (
	MetadataMatchNoMetadata MetadataMatchDecision = iota
	MetadataMatchNeedsCrossCheck
	MetadataMatchAccepted
)

type MetadataMatchRequest struct {
	Query      MetadataQuery
	Candidates []MetadataCandidate
}

type MetadataMatchResult struct {
	Decision     MetadataMatchDecision
	Confidence   MetadataMatchConfidence
	Candidate    MetadataCandidate
	Score        int
	CrossChecked bool
}

const (
	metadataMediumScore    = 50
	metadataHighScore      = 70
	metadataAmbiguityGap   = 8
	metadataMaxCandidates  = 100
	metadataMaxTitleBytes  = 4 << 10
	metadataMaxTitleRunes  = 1024
	metadataMaxSeasonBytes = 64
	metadataMaxSeasonRunes = 32
)

type metadataScoredCandidate struct {
	candidate MetadataCandidate
	title     string
	native    string
	season    string
	score     int
}

func MatchMetadata(request MetadataMatchRequest) MetadataMatchResult {
	result := noMetadataResult()
	queryTitle, queryNative, querySeason := metadataFields(request.Query.Title, request.Query.NativeTitle, request.Query.Season)
	if !validMetadataRequest(request, queryTitle, queryNative) {
		return result
	}

	scored := scoreMetadataCandidates(request, queryTitle, queryNative, querySeason)
	if len(scored) == 0 {
		return result
	}
	sort.SliceStable(scored, func(left, right int) bool {
		return metadataCandidateBefore(scored[left], scored[right])
	})

	top := scored[0]
	if top.score < metadataMediumScore || metadataHasConflict(top, scored[1:]) {
		return result
	}

	result.Candidate = top.candidate
	result.Score = top.score
	result.Confidence = MetadataMatchConfidenceMedium
	result.CrossChecked = metadataHasCrossCheck(top, scored[1:])
	if top.score >= metadataHighScore || result.CrossChecked {
		result.Decision = MetadataMatchAccepted
		if top.score >= metadataHighScore {
			result.Confidence = MetadataMatchConfidenceHigh
		}
		return result
	}
	result.Decision = MetadataMatchNeedsCrossCheck
	return result
}

func noMetadataResult() MetadataMatchResult {
	return MetadataMatchResult{
		Decision:   MetadataMatchNoMetadata,
		Confidence: MetadataMatchConfidenceLow,
	}
}

func validMetadataRequest(request MetadataMatchRequest, queryTitle, queryNative string) bool {
	return (queryTitle != "" || queryNative != "") && len(request.Candidates) > 0 && len(request.Candidates) <= metadataMaxCandidates
}

func scoreMetadataCandidates(request MetadataMatchRequest, queryTitle, queryNative, querySeason string) []metadataScoredCandidate {
	scored := make([]metadataScoredCandidate, 0, len(request.Candidates))
	for _, candidate := range request.Candidates {
		if strings.TrimSpace(candidate.Ref.Provider) == "" || strings.TrimSpace(candidate.Ref.ID) == "" {
			continue
		}
		title, native, season := metadataFields(candidate.Title, candidate.NativeTitle, candidate.Season)
		titleScore := bestMetadataTitleScore(queryTitle, queryNative, title, native)
		if titleScore == 0 {
			continue
		}
		scored = append(scored, metadataScoredCandidate{
			candidate: candidate,
			title:     title,
			native:    native,
			season:    season,
			score:     titleScore + metadataHintScore(request.Query, candidate, querySeason, season),
		})
	}
	return scored
}

func metadataCandidateBefore(left, right metadataScoredCandidate) bool {
	if left.score != right.score {
		return left.score > right.score
	}
	leftRank := metadataProviderRank(left.candidate.Ref.Provider)
	rightRank := metadataProviderRank(right.candidate.Ref.Provider)
	if leftRank != rightRank {
		return leftRank < rightRank
	}
	if left.candidate.Ref.Provider != right.candidate.Ref.Provider {
		return left.candidate.Ref.Provider < right.candidate.Ref.Provider
	}
	return left.candidate.Ref.ID < right.candidate.Ref.ID
}

func metadataHasCrossCheck(top metadataScoredCandidate, others []metadataScoredCandidate) bool {
	for _, other := range others {
		if other.score >= metadataMediumScore && equivalentMetadataCandidates(top, other) && !sameMetadataProvider(top, other) {
			return true
		}
	}
	return false
}

func metadataHasConflict(top metadataScoredCandidate, others []metadataScoredCandidate) bool {
	for _, other := range others {
		if equivalentMetadataCandidates(top, other) {
			continue
		}
		if other.score >= top.score-metadataAmbiguityGap {
			return true
		}
	}
	return false
}

func sameMetadataProvider(left, right metadataScoredCandidate) bool {
	return strings.EqualFold(strings.TrimSpace(left.candidate.Ref.Provider), strings.TrimSpace(right.candidate.Ref.Provider))
}

func bestMetadataTitleScore(queryTitle, queryNative, candidateTitle, candidateNative string) int {
	best := 0
	for _, query := range []string{queryTitle, queryNative} {
		for _, candidate := range []string{candidateTitle, candidateNative} {
			if score := metadataTextScore(query, candidate); score > best {
				best = score
			}
		}
	}
	return best
}

func metadataTextScore(query, candidate string) int {
	if query == "" || candidate == "" {
		return 0
	}
	if query == candidate {
		return metadataHighScore
	}
	if len([]rune(query)) >= 3 && len([]rune(candidate)) >= 3 && (strings.Contains(query, candidate) || strings.Contains(candidate, query)) {
		return metadataMediumScore + 2
	}
	return 0
}

func metadataHintScore(query MetadataQuery, candidate MetadataCandidate, querySeason, candidateSeason string) int {
	score := 0
	if querySeason != "" && candidateSeason != "" {
		if querySeason == candidateSeason {
			score += 12
		} else {
			score -= 14
		}
	} else if querySeason != "" || candidateSeason != "" {
		score -= 10
	}
	if query.Year > 0 && candidate.Year > 0 {
		if query.Year == candidate.Year {
			score += 12
		} else {
			score -= 14
		}
	}
	if query.EpisodeCount > 0 && candidate.EpisodeCount > 0 {
		if query.EpisodeCount == candidate.EpisodeCount {
			score += 8
		} else {
			score -= 10
		}
	}
	return score
}

func equivalentMetadataCandidates(left, right metadataScoredCandidate) bool {
	titleEqual := left.title != "" && left.title == right.title
	nativeEqual := left.native != "" && left.native == right.native
	if !titleEqual && !nativeEqual {
		return false
	}
	if left.season != "" && right.season != "" && left.season != right.season {
		return false
	}
	if left.candidate.Year > 0 && right.candidate.Year > 0 && left.candidate.Year != right.candidate.Year {
		return false
	}
	if left.candidate.EpisodeCount > 0 && right.candidate.EpisodeCount > 0 && left.candidate.EpisodeCount != right.candidate.EpisodeCount {
		return false
	}
	return true
}

func metadataProviderRank(provider string) int {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anilist":
		return 0
	case "bangumi":
		return 1
	default:
		return 2
	}
}
