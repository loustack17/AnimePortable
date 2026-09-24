// SPDX-License-Identifier: MPL-2.0

package native

import (
	"sort"
	"strings"
	"time"

	"animeportable/apps/desktop/backend"
)

const homeLimit = 6

func latestHistory(items []backend.History) []backend.History {
	newest := make(map[string]backend.History, len(items))
	for _, item := range items {
		previous, ok := newest[item.AnimeID]
		if !ok || newerHistory(item, previous) {
			newest[item.AnimeID] = item
		}
	}
	result := make([]backend.History, 0, len(newest))
	for _, item := range newest {
		if !item.Completed && item.Position > 0 && (item.Duration <= 0 || item.Position < item.Duration) {
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool { return newerHistory(result[i], result[j]) })
	if len(result) > homeLimit {
		result = result[:homeLimit]
	}
	return result
}

func newerHistory(left, right backend.History) bool {
	leftTime, leftErr := time.Parse(time.RFC3339Nano, left.LastPlayed)
	rightTime, rightErr := time.Parse(time.RFC3339Nano, right.LastPlayed)
	switch {
	case leftErr == nil && rightErr == nil:
		if !leftTime.Equal(rightTime) {
			return leftTime.After(rightTime)
		}
	case leftErr == nil:
		return true
	case rightErr == nil:
		return false
	case left.LastPlayed != right.LastPlayed:
		return left.LastPlayed > right.LastPlayed
	}
	if left.EpisodeID != right.EpisodeID {
		return left.EpisodeID > right.EpisodeID
	}
	return left.AnimeID > right.AnimeID
}

func titleFor(library []backend.Anime, animeID string) string {
	for _, item := range library {
		if item.ID == animeID && strings.TrimSpace(item.Title) != "" {
			return strings.TrimSpace(item.Title)
		}
	}
	return "無法取得作品名稱"
}
