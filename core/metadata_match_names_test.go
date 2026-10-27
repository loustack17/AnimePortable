// SPDX-License-Identifier: MPL-2.0

package core

import "testing"

func bangumiCandidate(id, title, native string, year, episodes int, aliases ...string) MetadataCandidate {
	return MetadataCandidate{Ref: MetadataRef{Provider: "bangumi", ID: id}, Title: title, NativeTitle: native, Year: year, EpisodeCount: episodes, Aliases: aliases}
}

func acceptedID(result MetadataMatchResult) string {
	if result.Decision != MetadataMatchAccepted {
		return ""
	}
	return result.Candidate.Ref.ID
}

func TestMatchMetadataReadsAnUnclosedTildeAsASubtitle(t *testing.T) {
	query := MetadataQuery{Title: "被放逐的作弊賦予魔術師享受隨心所欲的第二人生。～不只是武器、我還能賦予萬物「強化點數」，其他的人沒問題嗎？", Year: 2026}
	right := bangumiCandidate("625799", "被放逐的作弊賦予魔術師享受隨心所欲的第二人生。", "追放されたチート付与魔術師は気ままなセカンドライフを謳歌する。", 2026, 12)
	if got := acceptedID(MatchMetadata(MetadataMatchRequest{Query: query, Candidates: []MetadataCandidate{right}})); got != "625799" {
		t.Fatalf("accepted %q, want 625799", got)
	}
	// Two years apart is a different work: not accepted on the name alone. (One year apart is
	// neutral; Anime1 can list a show a year after Japan.)
	query.Year = 2028
	if got := acceptedID(MatchMetadata(MetadataMatchRequest{Query: query, Candidates: []MetadataCandidate{right}})); got != "" {
		t.Fatalf("other year accepted %q", got)
	}
	if got := normalizeMetadataTitle("作品名 ～副標題"); got != "作品名" {
		t.Fatalf("normalizeMetadataTitle = %q, want the main title only", got)
	}
}

func TestMatchMetadataIgnoresParticlesOnlyWithTheSameSubtitle(t *testing.T) {
	query := MetadataQuery{Title: "世界最強後衛～迷宮國的新人探索者～", Year: 2026, EpisodeCount: 12}
	right := bangumiCandidate("583862", "世界最強的後衛 ～迷宮國的新人探索者～", "世界最強の後衛 ～迷宮国の新人探索者～", 2026, 12)
	if got := acceptedID(MatchMetadata(MetadataMatchRequest{Query: query, Candidates: []MetadataCandidate{right}})); got != "583862" {
		t.Fatalf("accepted %q, want 583862", got)
	}
	for _, other := range []MetadataCandidate{
		bangumiCandidate("1", "世界最強的後衛 ～另一個副標～", "", 2026, 12),
		bangumiCandidate("2", "世界最強的後衛", "", 2026, 12),
	} {
		if got := acceptedID(MatchMetadata(MetadataMatchRequest{Query: query, Candidates: []MetadataCandidate{other}})); got != "" {
			t.Fatalf("%s accepted without the same subtitle", other.Title)
		}
	}
}

func TestMatchMetadataFindsASequelByTheSeriesNameOfItsFirstSeason(t *testing.T) {
	first := bangumiCandidate("420628", "藥屋少女的呢喃", "薬屋のひとりごと", 2023, 24, "药师少女的独语", "Kusuriya no Hitorigoto")
	second := bangumiCandidate("486347", "藥屋少女的呢喃 第二季", "薬屋のひとりごと 第2期", 2025, 24, "药师少女的独语 第二季")
	third := bangumiCandidate("568244", "藥屋少女的呢喃 第三季", "薬屋のひとりごと 第3期", 2026, 12)
	query := MetadataQuery{Title: "藥師少女的獨語 第三季", Year: 2026}
	if got := acceptedID(MatchMetadata(MetadataMatchRequest{Query: query, Candidates: []MetadataCandidate{first, second, third}})); got != "568244" {
		t.Fatalf("accepted %q, want the third season 568244", got)
	}
	// Without the first season's alias there is no series name: a different name never matches.
	if got := acceptedID(MatchMetadata(MetadataMatchRequest{Query: query, Candidates: []MetadataCandidate{third}})); got != "" {
		t.Fatalf("accepted %q without the series alias", got)
	}
	// The series name never picks another season: a fourth season that does not exist yet
	// leaves no match rather than the third.
	query.Title = "藥師少女的獨語 第四季"
	query.Year = 2027
	if got := acceptedID(MatchMetadata(MetadataMatchRequest{Query: query, Candidates: []MetadataCandidate{first, second, third}})); got != "" {
		t.Fatalf("fourth season accepted %q", got)
	}
}

func TestSeasonCandidatesComeInThroughTheSeriesName(t *testing.T) {
	base := bangumiCandidate("296076", "租借女友", "彼女、お借りします", 2020, 12, "出租女友")
	fifth := bangumiCandidate("533027", "租借女友 第五季", "彼女、お借りします 第5期", 2026, 12)
	unrelated := bangumiCandidate("9", "別的作品", "", 2026, 12)
	query := MetadataQuery{Title: "出租女友 第五季", Year: 2026, EpisodeCount: 12}
	queryTitle, queryNative, _ := metadataFields(query.Title, query.NativeTitle)
	series := metadataSeriesNames(queryTitle, queryNative, []MetadataCandidate{base})
	if !metadataNamesMatch(query, fifth, series) || metadataNamesMatch(query, unrelated, series) {
		t.Fatalf("season filter: fifth=%t unrelated=%t, want only the fifth season", metadataNamesMatch(query, fifth, series), metadataNamesMatch(query, unrelated, series))
	}
	if got := acceptedID(MatchMetadata(MetadataMatchRequest{Query: query, Candidates: []MetadataCandidate{base, fifth, unrelated}})); got != "533027" {
		t.Fatalf("accepted %q, want 533027", got)
	}
}

func TestMatchMetadataUnclosedTildeBeforeASeasonFailsClosed(t *testing.T) {
	// The unclosed subtitle takes the season marker with it, so the second season is not told
	// from the first: no match, rather than the wrong season.
	query := MetadataQuery{Title: "作品名～副標題 第二季", Year: 2026}
	for _, candidate := range []MetadataCandidate{
		bangumiCandidate("1", "作品名", "", 2024, 12),
		bangumiCandidate("2", "作品名 第二季", "", 2026, 12),
	} {
		if got := acceptedID(MatchMetadata(MetadataMatchRequest{Query: query, Candidates: []MetadataCandidate{candidate}})); got == "1" {
			t.Fatalf("accepted the first season for a second-season query")
		}
	}
}
