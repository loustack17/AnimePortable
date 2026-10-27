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
	"animeportable/apps/desktop/uitext"
)

const rowLimit = 6

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
	History backend.History
	Title   string
}

// homeCaption uses the stored episode number, so Home needs no network to show it.
func homeCaption(row homeRow) string {
	number := strings.TrimSpace(row.History.EpisodeNumber)
	if number == "" {
		number = uitext.T().UnknownNumber
	}
	if finished(row.History) {
		return fmt.Sprintf(uitext.T().RowFinished, number)
	}
	return fmt.Sprintf(uitext.T().RowPosition, number, formatPosition(row.History.Position))
}

func selectHomeRows(history []backend.History, library []backend.Anime, limit int) []homeRow {
	if limit < 1 {
		return nil
	}
	titles := animeTitles(library)
	// Pick the newest started or finished episode per series: opening another episode
	// without watching it does not hide the series, and finishing the newest one shows it
	// as watched instead of an older unfinished episode (owner decision 2026-10-21).
	ordered := newestPerAnime(history, func(item backend.History) bool { return resumable(item) || finished(item) })
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

// finished reports whether an entry was watched to the end.
func finished(item backend.History) bool {
	return item.Completed || item.Duration > 0 && item.Position >= item.Duration
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
		return uitext.T().UnknownTitle
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
		status := EpisodeLabel(item.EpisodeNumber) + " · " + historyProgress(item)
		if played, err := time.Parse(time.RFC3339Nano, item.LastPlayed); err == nil {
			status += fmt.Sprintf(uitext.T().LastWatched, daysAgo(now, played))
		}
		rows = append(rows, historyRow{History: item, Title: titles(item.AnimeID), Status: status})
	}
	return rows
}

func historyProgress(item backend.History) string {
	switch {
	case finished(item):
		return uitext.T().Finished
	case item.Position <= 0:
		return uitext.T().NotStarted
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
			anime.Title = uitext.T().UnknownTitle
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
		rows[index].Status = EpisodeLabel(numbers[id]) + " · " + daysAgo(now, seen[id])
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
		return uitext.T().Today
	case days == 1:
		return uitext.T().Yesterday
	default:
		return fmt.Sprintf(uitext.T().DaysAgo, days)
	}
}

func recentUpdateEmptyText(following []backend.Following) string {
	if len(following) == 0 {
		return uitext.T().NoFollowing
	}
	return uitext.T().NoRecentUpdates
}

// followingStatus says which episode the user has watched up to and the saved position in
// it, for example "看到：第 10 集 · 7:52 / 24:00".
func followingStatus(item backend.Following, history []backend.History) string {
	if !item.HasWatched {
		return uitext.T().NotWatched
	}
	status := fmt.Sprintf(uitext.T().WatchedUpTo, EpisodeLabel(item.LatestWatchedNumber))
	for _, entry := range history {
		// An episode only opened has no position worth showing.
		if entry.AnimeID == item.AnimeID && entry.EpisodeID == item.LatestWatched && (entry.Completed || entry.Position > 0) {
			return status + " · " + historyProgress(entry)
		}
	}
	return status
}

// scheduleDays is how many days 時間表 shows, starting today.
const scheduleDays = 7

// scheduleDay is one column of the 時間表 week, for example Tuesday 10/20, today.
type scheduleDay struct {
	Weekday time.Weekday
	Date    string
	Today   bool
	Shows   []scheduleShow
}

// heading names the day in the current language, for example 週二 and 今天 10/20.
func (day scheduleDay) heading() string {
	text := uitext.T()
	date := day.Date
	if day.Today {
		date = fmt.Sprintf(text.TodayDate, date)
	}
	return text.Weekdays[day.Weekday] + "\n" + date
}

// scheduleShow is one show on a day. Anime.ID is empty when the app does not know the show;
// Query is then what to search for.
type scheduleShow struct {
	Anime backend.Anime
	Query string
}

// scheduleWeek lays the weekly schedule onto the scheduleDays days from today, in source
// order within a day. Every day gets a column, with or without shows.
func scheduleWeek(items []backend.ScheduleItem, now time.Time) []scheduleDay {
	year, month, date := now.Date()
	today := time.Date(year, month, date, 0, 0, 0, 0, now.Location())
	days := make([]scheduleDay, 0, scheduleDays)
	for offset := range scheduleDays {
		day := today.AddDate(0, 0, offset)
		column := scheduleDay{Weekday: day.Weekday(), Date: fmt.Sprintf("%d/%d", day.Month(), day.Day()), Today: offset == 0}
		for _, item := range items {
			if time.Weekday(item.Weekday) != day.Weekday() {
				continue
			}
			show := scheduleShow{Anime: backend.Anime{ID: item.AnimeID, Title: strings.TrimSpace(item.Title)}, Query: item.Query}
			if show.Anime.Title == "" {
				show.Anime.Title = uitext.T().UnknownTitle
			}
			column.Shows = append(column.Shows, show)
		}
		days = append(days, column)
	}
	return days
}

// EpisodeLabel names an episode by its stored number; the player's episode list uses it too.
func EpisodeLabel(number string) string {
	number = strings.TrimSpace(number)
	if number == "" {
		return uitext.T().EpisodeUnknown
	}
	return fmt.Sprintf(uitext.T().EpisodeNumber, number)
}

func playRequest(item backend.History) backend.PlayRequest {
	return backend.PlayRequest{AnimeID: item.AnimeID, EpisodeID: item.EpisodeID, StartAt: max(item.Position, int64(0))}
}

func startupErrorMessage(err error) string {
	if backend.IsDatabaseExists(err) {
		return uitext.T().DataExists
	}
	return uitext.T().DataFolderUnwritable
}

func homeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return uitext.T().LoadAnimeFailed
}

func isCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// pageCount is how many pages of size hold items.
func pageCount(items, size int) int {
	return (items + size - 1) / size
}

// movedPage is page moved by delta and kept within the pages that hold items (items > 0).
func movedPage(page, delta, items, size int) int {
	return max(0, min((items-1)/size, page+delta))
}
