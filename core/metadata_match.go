// SPDX-License-Identifier: MPL-2.0

package core

import (
	"regexp"
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
	metadataMediumScore = 50
	metadataHighScore   = 70
	// metadataWeakExactScore is ten below a full match, beyond metadataAmbiguityGap, so a full
	// title match wins cleanly over an alias or one-sided-subtitle match.
	metadataWeakExactScore = metadataHighScore - 10
	metadataAmbiguityGap   = 8
	metadataMaxCandidates  = 100
	metadataMaxTitleBytes  = 4 << 10
	metadataMaxTitleRunes  = 1024
	metadataMaxSeasonBytes = 64
	metadataMaxSeasonRunes = 32
	metadataMaxAliases     = 16
)

type metadataScoredCandidate struct {
	candidate  MetadataCandidate
	title      string
	native     string
	season     string
	titleScore int
	score      int
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
	// A weak exact name match (alias, or a subtitle on one side only) is accepted when no other
	// candidate is close and no hint counts against it.
	weakExactAccepted := top.titleScore == metadataWeakExactScore && top.score >= metadataWeakExactScore
	if top.score >= metadataHighScore || result.CrossChecked || weakExactAccepted {
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
		aliases := metadataAliases(candidate.Aliases)
		titleScore := bestMetadataTitleScore(queryTitle, queryNative, []string{title, native}, aliases)
		if titleScore == 0 {
			continue
		}
		scored = append(scored, metadataScoredCandidate{
			candidate:  candidate,
			title:      title,
			native:     native,
			season:     season,
			titleScore: titleScore,
			score:      titleScore + metadataHintScore(request.Query, candidate, querySeason, season),
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
	// Numeric IDs (Bangumi) compare by value so the older entry comes first.
	if len(left.candidate.Ref.ID) != len(right.candidate.Ref.ID) && metadataNumericID(left.candidate.Ref.ID) && metadataNumericID(right.candidate.Ref.ID) {
		return len(left.candidate.Ref.ID) < len(right.candidate.Ref.ID)
	}
	return left.candidate.Ref.ID < right.candidate.Ref.ID
}

func metadataNumericID(value string) bool {
	return value != "" && strings.Trim(value, "0123456789") == ""
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
		if equivalentMetadataCandidates(top, other) && (!sameMetadataProvider(top, other) || top.candidate.Ref.ID == other.candidate.Ref.ID) {
			continue
		}
		if sameSeasonArcs(top, other) {
			continue
		}
		if other.score >= top.score-metadataAmbiguityGap {
			return true
		}
	}
	return false
}

// sameSeasonArcs reports two entries of one provider for the same season of the same show that
// differ only in arc name ("第三季 襲擊篇" and "第三季 反擊篇"). Either describes that season, so
// they are not a conflict; the higher-ranked one is used.
func sameSeasonArcs(left, right metadataScoredCandidate) bool {
	leftMain, leftArc, _ := strings.Cut(left.title, metadataSubtitleSeparator)
	rightMain, rightArc, _ := strings.Cut(right.title, metadataSubtitleSeparator)
	return sameMetadataProvider(left, right) && left.season != "" && left.season == right.season &&
		leftMain != "" && leftMain == rightMain && leftArc != "" && rightArc != "" &&
		!metadataSpecialArc.MatchString(leftArc) && !metadataSpecialArc.MatchString(rightArc)
}

// metadataSpecialArc marks recaps and extras, which are not a season's main arcs.
var metadataSpecialArc = regexp.MustCompile(`(?i)ova|oad|special|特别|总集|剧场|迷你|sp`)

func sameMetadataProvider(left, right metadataScoredCandidate) bool {
	return strings.EqualFold(strings.TrimSpace(left.candidate.Ref.Provider), strings.TrimSpace(right.candidate.Ref.Provider))
}

// metadataAliases normalizes alternative names the same way as titles, dropping invalid ones.
func metadataAliases(values []string) []string {
	aliases := make([]string, 0, min(len(values), metadataMaxAliases))
	for _, value := range values[:min(len(values), metadataMaxAliases)] {
		if alias, _, ok := metadataTitleParts(value); ok && alias != "" {
			aliases = append(aliases, alias)
		}
	}
	return aliases
}

// bestMetadataTitleScore scores the candidate's titles normally but its aliases only on a full
// exact match, and then as a weak exact match: aliases are many and short, and a sequel's
// aliases often include the bare series name, so they must rank below a real title match.
func bestMetadataTitleScore(queryTitle, queryNative string, titles, aliases []string) int {
	best := 0
	for _, query := range []string{queryTitle, queryNative} {
		for _, candidate := range titles {
			best = max(best, metadataTextScore(query, candidate))
		}
		for _, alias := range aliases {
			if metadataTextScore(query, alias) == metadataHighScore {
				best = max(best, metadataWeakExactScore)
			}
		}
	}
	return best
}

// metadataCompareKey ignores spacing ("SPY×FAMILY間諜家家酒" and "SPY×FAMILY 間諜家家酒") and a
// standalone "x", which titles such as "SPY x FAMILY" use in place of "×".
func metadataCompareKey(value string) string {
	words := strings.Fields(value)
	kept := make([]string, 0, len(words))
	for index, word := range words {
		// Only an "x" between two words stands for "×"; "Gundam X" keeps its X.
		if word == "x" && index > 0 && index < len(words)-1 {
			continue
		}
		kept = append(kept, word)
	}
	return strings.Join(kept, "")
}

// metadataNameMatch compares normalized names. Equal main titles with equal (or both missing)
// subtitles are a full match. A subtitle on only one side is a weaker match: sources often omit
// it, but a side entry such as "葬送的芙莉蓮 ～●●的魔法～" must rank below the main show. Two
// different subtitles mean two different works.
func metadataNameMatch(left, right string) (equal, full bool) {
	leftMain, leftSubtitle, _ := strings.Cut(left, metadataSubtitleSeparator)
	rightMain, rightSubtitle, _ := strings.Cut(right, metadataSubtitleSeparator)
	// The same subtitle may be written with tildes or a colon ("無職英雄 ～技能什麼的毫無用處～",
	// "無職英雄：技能什麼的毫無用處"), so the whole name counts as a full match too.
	if metadataCompareKey(leftMain+" "+leftSubtitle) == metadataCompareKey(rightMain+" "+rightSubtitle) {
		return true, true
	}
	if metadataCompareKey(leftMain) != metadataCompareKey(rightMain) {
		return false, false
	}
	if metadataCompareKey(leftSubtitle) == metadataCompareKey(rightSubtitle) {
		return true, true
	}
	return leftSubtitle == "" || rightSubtitle == "", false
}

// metadataSubtitlesDiffer reports two names that both carry a subtitle and the subtitles differ:
// different works, even when the main titles contain each other.
func metadataSubtitlesDiffer(left, right string) bool {
	_, leftSubtitle, _ := strings.Cut(left, metadataSubtitleSeparator)
	_, rightSubtitle, _ := strings.Cut(right, metadataSubtitleSeparator)
	return leftSubtitle != "" && rightSubtitle != "" && metadataCompareKey(leftSubtitle) != metadataCompareKey(rightSubtitle)
}

func metadataTextScore(query, candidate string) int {
	if query == "" || candidate == "" {
		return 0
	}
	if equal, full := metadataNameMatch(query, candidate); full {
		return metadataHighScore
	} else if equal {
		return metadataWeakExactScore
	}
	if metadataSubtitlesDiffer(query, candidate) {
		return 0
	}
	// Containment compares main titles only; subtitles are too short and generic for it.
	query, _, _ = strings.Cut(query, metadataSubtitleSeparator)
	candidate, _, _ = strings.Cut(candidate, metadataSubtitleSeparator)
	query, candidate = metadataCompareKey(query), metadataCompareKey(candidate)
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

// metadataProviderRank breaks score ties. Bangumi comes first because its record has the Chinese
// synopsis the app shows; AniList's is English (ADR-009, owner decision 2026-10-14).
func metadataProviderRank(provider string) int {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "bangumi":
		return 0
	case "anilist":
		return 1
	default:
		return 2
	}
}
