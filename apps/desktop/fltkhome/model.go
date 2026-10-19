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

var navigationOrder = [...]int{0, 4, 1, 2, 3, 5}

func navigationPosition(section int) int {
	for position, index := range navigationOrder {
		if index == section {
			return position
		}
	}
	return 0
}

type homeRow struct {
	History       backend.History
	Title         string
	EpisodeNumber string
}

func episodeNumber(episodes []backend.Episode, episodeID string, err error) string {
	if err != nil || strings.TrimSpace(episodeID) == "" {
		return "未知"
	}
	for _, episode := range episodes {
		if episode.ID == episodeID {
			if number := strings.TrimSpace(episode.Number); number != "" {
				return number
			}
			return "未知"
		}
	}
	return "未知"
}

func homeCaption(row homeRow) string {
	number := strings.TrimSpace(row.EpisodeNumber)
	if number == "" {
		number = "未知"
	}
	return fmt.Sprintf("集數：%s · 上次播放位置 %s", number, formatPosition(row.History.Position))
}

func selectHomeRows(history []backend.History, library []backend.Anime, limit int) []homeRow {
	if limit < 1 {
		return nil
	}
	titles := animeTitles(library)
	// Pick the newest resumable episode per series, so opening another episode
	// without watching it does not hide the series from Continue Watching.
	ordered := newestPerAnime(history, resumable)
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	rows := make([]homeRow, 0, len(ordered))
	for _, item := range ordered {
		rows = append(rows, homeRow{History: item, Title: titles(item.AnimeID)})
	}
	return rows
}

// resumable reports whether an entry has a saved position to continue from.
func resumable(item backend.History) bool {
	return !item.Completed && item.Position > 0 && (item.Duration <= 0 || item.Position < item.Duration)
}

// newestPerAnime keeps the most recently played entry per anime among those include
// accepts, newest first.
func newestPerAnime(history []backend.History, include func(backend.History) bool) []backend.History {
	newest := make(map[string]backend.History, len(history))
	for _, item := range history {
		if !include(item) {
			continue
		}
		previous, exists := newest[item.AnimeID]
		if !exists || newerHistory(item, previous) {
			newest[item.AnimeID] = item
		}
	}
	ordered := make([]backend.History, 0, len(newest))
	for _, item := range newest {
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(i, j int) bool { return newerHistory(ordered[i], ordered[j]) })
	return ordered
}

// animeTitles looks up display titles by anime ID; unknown or blank titles read 未知作品.
func animeTitles(library []backend.Anime) func(string) string {
	titles := make(map[string]string, len(library))
	for _, item := range library {
		titles[item.ID] = strings.TrimSpace(item.Title)
	}
	return func(id string) string {
		if title := titles[id]; title != "" {
			return title
		}
		return "未知作品"
	}
}

type historyRow struct {
	History backend.History
	Title   string
	Status  string
}

// historyRows lists the 歷史紀錄 page: each anime's most recently played episode, newest
// first. The status reads like "第 5 集 · 1:30 / 24:00 · 上次觀看：今天".
func historyRows(history []backend.History, library []backend.Anime, now time.Time) []historyRow {
	titles := animeTitles(library)
	ordered := newestPerAnime(history, func(backend.History) bool { return true })
	rows := make([]historyRow, 0, len(ordered))
	for _, item := range ordered {
		status := episodeLabel(item.EpisodeNumber) + " · " + historyProgress(item)
		if played, err := time.Parse(time.RFC3339Nano, item.LastPlayed); err == nil {
			status += " · 上次觀看：" + daysAgo(now, played)
		}
		rows = append(rows, historyRow{History: item, Title: titles(item.AnimeID), Status: status})
	}
	return rows
}

func historyProgress(item backend.History) string {
	switch {
	case item.Completed || item.Duration > 0 && item.Position >= item.Duration:
		return "已看完"
	case item.Position <= 0:
		return "尚未開始"
	case item.Duration <= 0:
		return formatPosition(item.Position)
	}
	return formatPosition(item.Position) + " / " + formatPosition(item.Duration)
}

// historyPlayRequest continues at the saved position, or plays a finished or unstarted
// episode from the beginning.
func historyPlayRequest(item backend.History) backend.PlayRequest {
	if resumable(item) {
		return playRequest(item)
	}
	return backend.PlayRequest{AnimeID: item.AnimeID, EpisodeID: item.EpisodeID}
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

type followingRow struct {
	Anime  backend.Anime
	Status string
}

// followingRows lists followed anime by title. The status says only how far the user has
// watched; it does not hint at unwatched or new episodes (owner decision 2026-10-19).
func followingRows(following []backend.Following, library []backend.Anime, history []backend.History) []followingRow {
	known := make(map[string]backend.Anime, len(library))
	for _, item := range library {
		known[item.ID] = item
	}
	rows := make([]followingRow, 0, len(following))
	for _, item := range following {
		anime, ok := known[item.AnimeID]
		if !ok {
			anime = backend.Anime{ID: item.AnimeID}
		}
		anime.Title = strings.TrimSpace(anime.Title)
		if anime.Title == "" {
			anime.Title = "未知作品"
		}
		rows = append(rows, followingRow{Anime: anime, Status: followingStatus(item, history)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Anime.Title != rows[j].Anime.Title {
			return rows[i].Anime.Title < rows[j].Anime.Title
		}
		return rows[i].Anime.ID < rows[j].Anime.ID
	})
	return rows
}

// recentUpdateRows lists Home's 近期更新: followed anime the backend marks as recently
// updated, newest sighting first, at most limit rows. The status says which episode and
// how long ago it was first seen, for example "第 12 集 · 2 天前".
func recentUpdateRows(following []backend.Following, library []backend.Anime, limit int, now time.Time) []followingRow {
	updated := make([]backend.Following, 0, len(following))
	seen := make(map[string]time.Time, len(following))
	numbers := make(map[string]string, len(following))
	for _, item := range following {
		if !item.RecentlyUpdated {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, item.LatestSeenAt)
		if err != nil {
			continue
		}
		updated = append(updated, item)
		seen[item.AnimeID] = at
		numbers[item.AnimeID] = item.LatestAvailableNumber
	}
	rows := followingRows(updated, library, nil)
	for index := range rows {
		id := rows[index].Anime.ID
		rows[index].Status = episodeLabel(numbers[id]) + " · " + daysAgo(now, seen[id])
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return seen[rows[i].Anime.ID].After(seen[rows[j].Anime.ID])
	})
	return rows[:min(len(rows), limit)]
}

// recentUpdateKey identifies a set of 近期更新 rows by anime and episode state, ignoring
// titles, so Home can tell whether a reload would change them.
func recentUpdateKey(rows []followingRow) string {
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, row.Anime.ID+"\x00"+row.Status)
	}
	return strings.Join(parts, "\x01")
}

// daysAgo counts local calendar days, so something seen late yesterday reads 昨天.
func daysAgo(now, at time.Time) string {
	day := func(value time.Time) time.Time {
		year, month, date := value.Local().Date()
		return time.Date(year, month, date, 0, 0, 0, 0, time.UTC)
	}
	switch days := int(day(now).Sub(day(at)).Hours() / 24); {
	case days <= 0:
		return "今天"
	case days == 1:
		return "昨天"
	default:
		return fmt.Sprintf("%d 天前", days)
	}
}

func recentUpdateEmptyText(following []backend.Following) string {
	if len(following) == 0 {
		return "尚未追蹤任何作品。在作品頁按星號即可追蹤。"
	}
	return "最近 7 天，追蹤的作品沒有新集數。"
}

// followingStatus says which episode the user has watched up to and the saved position in
// it, for example "看到：第 10 集 · 7:52 / 24:00".
func followingStatus(item backend.Following, history []backend.History) string {
	if !item.HasWatched {
		return "尚未觀看"
	}
	status := "看到：" + episodeLabel(item.LatestWatchedNumber)
	for _, entry := range history {
		// An episode only opened has no position worth showing.
		if entry.AnimeID == item.AnimeID && entry.EpisodeID == item.LatestWatched && (entry.Completed || entry.Position > 0) {
			return status + " · " + historyProgress(entry)
		}
	}
	return status
}

func episodeLabel(number string) string {
	number = strings.TrimSpace(number)
	if number == "" {
		return "集數未知"
	}
	return "第 " + number + " 集"
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
