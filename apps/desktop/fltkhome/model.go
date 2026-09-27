// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"animeportable/apps/desktop/backend"
)

const rowLimit = 6

var sections = [...]string{"首頁", "時間表", "追蹤", "歷史紀錄", "搜尋", "設定"}

type homeRow struct {
	History backend.History
	Title   string
}

func selectHomeRows(history []backend.History, library []backend.Anime, limit int) []homeRow {
	if limit < 1 {
		return nil
	}
	titles := make(map[string]string, len(library))
	for _, item := range library {
		titles[item.ID] = item.Title
	}
	newest := make(map[string]backend.History, len(history))
	for _, item := range history {
		previous, exists := newest[item.AnimeID]
		if !exists || newerHistory(item, previous) {
			newest[item.AnimeID] = item
		}
	}
	ordered := make([]backend.History, 0, len(newest))
	for _, item := range newest {
		if !item.Completed && item.Position > 0 && (item.Duration <= 0 || item.Position < item.Duration) {
			ordered = append(ordered, item)
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return newerHistory(ordered[i], ordered[j]) })
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	rows := make([]homeRow, 0, len(ordered))
	for _, item := range ordered {
		title := titles[item.AnimeID]
		if strings.TrimSpace(title) == "" {
			title = "未知作品"
		} else {
			title = strings.TrimSpace(title)
		}
		rows = append(rows, homeRow{History: item, Title: title})
	}
	return rows
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

func formatPosition(milliseconds int64) string {
	if milliseconds < 0 {
		milliseconds = 0
	}
	seconds := milliseconds / 1000
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func followingTitles(following []backend.Following, library []backend.Anime, limit int) []string {
	if limit < 1 {
		return nil
	}
	titles := make(map[string]string, len(library))
	for _, item := range library {
		titles[item.ID] = strings.TrimSpace(item.Title)
	}
	result := make([]string, 0, min(len(following), limit))
	for _, item := range following[:min(len(following), limit)] {
		name := titles[item.AnimeID]
		if name == "" {
			name = "未知作品"
		}
		result = append(result, name)
	}
	return result
}

func playRequest(item backend.History) backend.PlayRequest {
	return backend.PlayRequest{AnimeID: item.AnimeID, EpisodeID: item.EpisodeID, StartAt: max(item.Position, int64(0))}
}

func startupErrorMessage(err error, copying, imported bool) string {
	if backend.IsPortableImportConflict(err) {
		return "資料夾內已有資料，不會覆蓋。請關閉後重新開啟。"
	}
	if imported {
		return "舊資料已複製到這個資料夾，但目前無法開啟。請關閉後重試；舊資料仍保留。"
	}
	if copying {
		return "無法複製舊資料；舊資料沒有變更。你可以建立新的空白資料，或關閉程式。"
	}
	return "無法在此資料夾開啟資料，請確認解壓資料夾可寫入後重新開啟。"
}

func homeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return "無法載入作品資料，請重試。"
}

func isCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
