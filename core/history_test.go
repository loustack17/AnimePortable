// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestAppListHistoryAddsStoredEpisodeNumbers(t *testing.T) {
	history := []HistoryEntry{played("a", "ep-2", 3), played("b", "unmapped", 2), played("a", "ep-1", 1)}
	store := &fakeStore{
		episodeMappings: map[AnimeID][]EpisodeMapping{"a": numberedMappings("a", map[EpisodeID]string{"ep-1": "1", "ep-2": "2"})},
		history:         history,
	}
	// A nil source proves history never needs the network.
	app := NewApp(nil, &fakePlayer{}, store)

	entries, err := app.ListHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []WatchedEpisode{
		{HistoryEntry: history[0], EpisodeNumber: "2"},
		{HistoryEntry: history[1]},
		{HistoryEntry: history[2], EpisodeNumber: "1"},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
}

func TestAppListHistoryPropagatesErrors(t *testing.T) {
	storeErr := errors.New("store failed")
	tests := map[string]*fakeStore{
		"history":  {historyErr: storeErr},
		"mappings": {history: []HistoryEntry{played("a", "ep-1", 1)}, episodeMappingsErr: storeErr},
	}
	for name, store := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewApp(nil, &fakePlayer{}, store).ListHistory(context.Background()); !errors.Is(err, storeErr) {
				t.Fatalf("error = %v, want %v", err, storeErr)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewApp(nil, &fakePlayer{}, &fakeStore{}).ListHistory(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
}
