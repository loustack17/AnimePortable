// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The metadata evaluation set (Loop 37d): Anime1 titles with the provider subject each should
// match, or none, and what Bangumi and AniList searches returned for them, recorded once from
// the live services, with Bangumi's season lists. Replaying the recordings through
// RefreshMetadata measures the matcher offline: hits, wrong matches (ADR-009: never
// acceptable) and misses.
type metadataEvalSet struct {
	Recorded string               `json:"recorded"`
	Searches []metadataEvalSearch `json:"searches"`
	Browses  []metadataEvalBrowse `json:"browses"`
	Cases    []metadataEvalCase   `json:"cases"`
}

type metadataEvalBrowse struct {
	Provider   string              `json:"provider"`
	Year       int                 `json:"year"`
	Season     string              `json:"season"`
	Failed     bool                `json:"failed,omitempty"`
	Candidates []MetadataCandidate `json:"candidates"`
}

type metadataEvalSearch struct {
	Provider   string              `json:"provider"`
	Keyword    string              `json:"keyword"`
	Failed     bool                `json:"failed,omitempty"`
	Candidates []MetadataCandidate `json:"candidates"`
}

type metadataEvalCase struct {
	Title       string      `json:"title"`
	NativeTitle string      `json:"native_title,omitempty"`
	Catalog     CatalogInfo `json:"catalog"`
	// Expect lists the acceptable subjects as provider:id; empty means no match is correct.
	Expect []string `json:"expect"`
	// MustHit marks a case the matcher already gets right, so a change that loses it fails.
	MustHit bool   `json:"must_hit,omitempty"`
	Note    string `json:"note,omitempty"`
}

type evalStore struct {
	fakeStore
	anime Anime
}

func (store *evalStore) Anime(context.Context, AnimeID) (Anime, error) { return store.anime, nil }

func (store *evalStore) SaveMetadata(context.Context, AnimeID, AnimeMetadata) error { return nil }

type evalProvider struct {
	name     string
	searches map[string]metadataEvalSearch
	missing  *[]string
	accepted *string
}

// evalBrowsingProvider is Bangumi, which also lists a season.
type evalBrowsingProvider struct {
	evalProvider
	browses map[string]metadataEvalBrowse
}

func evalBrowseKey(provider string, year int, season string) string {
	return provider + "\x00" + strconv.Itoa(year) + "\x00" + season
}

func (provider evalBrowsingProvider) BrowseSeason(_ context.Context, year int, season string) ([]MetadataCandidate, error) {
	recorded, ok := provider.browses[evalBrowseKey(provider.name, year, season)]
	if !ok {
		*provider.missing = append(*provider.missing, provider.name+" season: "+strconv.Itoa(year)+" "+season)
		return nil, errors.New("season not recorded")
	}
	if recorded.Failed {
		return nil, errors.New("recorded failure")
	}
	return recorded.Candidates, nil
}

func (provider evalProvider) Search(_ context.Context, query MetadataQuery) ([]MetadataCandidate, error) {
	// The adapters search by the title, or the native title when there is none.
	keyword := strings.TrimSpace(query.Title)
	if keyword == "" {
		keyword = strings.TrimSpace(query.NativeTitle)
	}
	recorded, ok := provider.searches[provider.name+"\x00"+keyword]
	if !ok {
		*provider.missing = append(*provider.missing, provider.name+": "+keyword)
		return nil, errors.New("search not recorded")
	}
	if recorded.Failed {
		return nil, errors.New("recorded failure")
	}
	return recorded.Candidates, nil
}

func (provider evalProvider) Get(_ context.Context, ref MetadataRef) (AnimeMetadata, error) {
	*provider.accepted = ref.Provider + ":" + ref.ID
	return AnimeMetadata{Ref: ref}, nil
}

func TestMetadataEvaluationSetHasNoWrongMatch(t *testing.T) {
	data, err := os.ReadFile("testdata/metadata_eval.json")
	if err != nil {
		t.Fatal(err)
	}
	var set metadataEvalSet
	if err := json.Unmarshal(data, &set); err != nil {
		t.Fatal(err)
	}
	searches := make(map[string]metadataEvalSearch, len(set.Searches))
	for _, search := range set.Searches {
		searches[search.Provider+"\x00"+search.Keyword] = search
	}
	browses := make(map[string]metadataEvalBrowse, len(set.Browses))
	for _, browse := range set.Browses {
		browses[evalBrowseKey(browse.Provider, browse.Year, browse.Season)] = browse
	}
	var hits, wrong, misses, correctNone int
	for _, test := range set.Cases {
		var accepted string
		var missing []string
		providers := []MetadataProvider{
			evalProvider{name: "anilist", searches: searches, missing: &missing, accepted: &accepted},
			evalBrowsingProvider{evalProvider: evalProvider{name: "bangumi", searches: searches, missing: &missing, accepted: &accepted}, browses: browses},
		}
		store := &evalStore{anime: Anime{ID: "anime", Title: test.Title, NativeTitle: test.NativeTitle, Catalog: test.Catalog}}
		if _, err := NewApp(nil, nil, store).RefreshMetadata(context.Background(), "anime", providers); err != nil && !errors.Is(err, ErrMetadataUnavailable) {
			t.Fatalf("%s: %v", test.Title, err)
		}
		if len(missing) > 0 {
			t.Errorf("%s: lookups not in the recording (record them again): %v", test.Title, missing)
			continue
		}
		switch {
		case accepted != "" && slices.Contains(test.Expect, accepted):
			hits++
		case accepted != "":
			wrong++
			t.Errorf("WRONG MATCH %s: accepted %s, want %v", test.Title, accepted, test.Expect)
		case len(test.Expect) > 0:
			misses++
			if test.MustHit {
				t.Errorf("%s: no longer matched, want %v", test.Title, test.Expect)
			} else {
				t.Logf("miss: %s (want %v)", test.Title, test.Expect)
			}
		default:
			correctNone++
		}
	}
	t.Logf("metadata evaluation (%d cases, recorded %s): hits %d, wrong %d, misses %d, correctly none %d",
		len(set.Cases), set.Recorded, hits, wrong, misses, correctNone)
}
