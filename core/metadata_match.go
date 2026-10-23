// SPDX-License-Identifier: MPL-2.0

package core

import (
	"regexp"
	"slices"
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
	series := metadataSeriesNames(queryTitle, queryNative, request.Candidates)
	for _, candidate := range request.Candidates {
		if strings.TrimSpace(candidate.Ref.Provider) == "" || strings.TrimSpace(candidate.Ref.ID) == "" {
			continue
		}
		title, native, season := metadataFields(candidate.Title, candidate.NativeTitle, candidate.Season)
		aliases := metadataAliases(candidate.Aliases)
		titleScore := bestMetadataTitleScore(queryTitle, queryNative, []string{title, native}, aliases)
		if titleScore < metadataWeakExactScore && metadataSeriesMatch(series, title, native) {
			titleScore = metadataWeakExactScore
		}
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

// metadataNamesMatch reports whether any name of the candidate scores against the query, as
// scoreMetadataCandidates would score it with series (from metadataSeriesNames).
func metadataNamesMatch(query MetadataQuery, candidate MetadataCandidate, series []string) bool {
	queryTitle, queryNative, _ := metadataFields(query.Title, query.NativeTitle, query.Season)
	title, native, _ := metadataFields(candidate.Title, candidate.NativeTitle, candidate.Season)
	return bestMetadataTitleScore(queryTitle, queryNative, []string{title, native}, metadataAliases(candidate.Aliases)) > 0 ||
		metadataSeriesMatch(series, title, native)
}

// metadataSeriesNames finds the provider's own name for the query's series. Taiwan names are
// often only an alias of a series' first entry (Bangumi's 藥屋少女的呢喃 lists 药师少女的独语),
// while its sequels carry none ("藥屋少女的呢喃 第三季"). A candidate with an alias equal to the
// query's title, season markers aside, gives its own title as the series name; a sequel named
// so then matches weakly, and the season and year hints decide. queryTitle and queryNative are
// normalized as by metadataFields.
func metadataSeriesNames(queryTitle, queryNative string, candidates []MetadataCandidate) []string {
	var names []string
	for _, candidate := range candidates {
		title, native, _ := metadataFields(candidate.Title, candidate.NativeTitle, candidate.Season)
		for _, alias := range metadataAliases(candidate.Aliases) {
			if metadataTextScore(queryTitle, alias) != metadataHighScore && metadataTextScore(queryNative, alias) != metadataHighScore {
				continue
			}
			for _, name := range []string{title, native} {
				if name != "" && name != queryTitle && name != queryNative && !slices.Contains(names, name) {
					names = append(names, name)
				}
			}
			break
		}
	}
	return names
}

// metadataSeriesMatch reports a candidate named exactly as one of the series names.
func metadataSeriesMatch(series []string, title, native string) bool {
	for _, name := range series {
		if metadataTextScore(name, title) == metadataHighScore || metadataTextScore(name, native) == metadataHighScore {
			return true
		}
	}
	return false
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

// metadataParticles are written in one name and left out of another ("世界最強後衛" and
// "世界最強的後衛"), or written as their Japanese counterpart (之 and の).
var metadataParticles = strings.NewReplacer("的", "", "之", "", "の", "")

// metadataParticleMatch is a weak match for two names whose main titles differ only in
// metadataParticles and whose subtitles are the same and present: the subtitle carries the
// evidence the shortened main title lacks.
func metadataParticleMatch(left, right string) bool {
	leftMain, leftSubtitle, _ := strings.Cut(left, metadataSubtitleSeparator)
	rightMain, rightSubtitle, _ := strings.Cut(right, metadataSubtitleSeparator)
	return leftSubtitle != "" && metadataCompareKey(leftSubtitle) == metadataCompareKey(rightSubtitle) &&
		metadataParticles.Replace(metadataCompareKey(leftMain)) == metadataParticles.Replace(metadataCompareKey(rightMain))
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
	} else if equal || metadataParticleMatch(query, candidate) {
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
		switch difference := query.Year - candidate.Year; {
		case difference == 0:
			score += 12
		case difference == 1 || difference == -1:
			// Anime1 gives the year a show reached it, which can be the year after Japan's
			// (魔法少女奈葉 Detonation: 2018 there, 2019 on Bangumi); neither for nor against.
		default:
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
