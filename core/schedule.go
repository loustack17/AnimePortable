// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// MaxScheduleEntries bounds one weekly schedule; a provider listing more is malformed.
const MaxScheduleEntries = 512

// ScheduleEntry is one show in the weekly airing schedule, airing on Weekday. AnimeID is
// the local anime it matches, empty when the app does not know it.
type ScheduleEntry struct {
	Ref         MetadataRef
	Title       string
	NativeTitle string
	Weekday     time.Weekday
	AnimeID     AnimeID
}

// ScheduleProvider lists the current weekly airing schedule from an anime database.
type ScheduleProvider interface {
	Schedule(ctx context.Context) ([]ScheduleEntry, error)
}

// MatchSchedule fills AnimeID for entries the store could not match by metadata, when the
// entry's title or native title names exactly one anime in library.
func MatchSchedule(entries []ScheduleEntry, library []Anime) []ScheduleEntry {
	const ambiguous = AnimeID("\x00")
	byTitle := make(map[string]AnimeID, len(library))
	add := func(title string, id AnimeID) {
		key := scheduleTitleKey(title)
		if key == "" {
			return
		}
		if existing, ok := byTitle[key]; ok && existing != id {
			byTitle[key] = ambiguous
			return
		}
		byTitle[key] = id
	}
	for _, anime := range library {
		add(anime.Title, anime.ID)
		add(anime.NativeTitle, anime.ID)
	}
	matched := make([]ScheduleEntry, len(entries))
	for index, entry := range entries {
		if entry.AnimeID == "" {
			for _, title := range []string{entry.Title, entry.NativeTitle} {
				if id, ok := byTitle[scheduleTitleKey(title)]; ok && id != ambiguous {
					entry.AnimeID = id
					break
				}
			}
		}
		matched[index] = entry
	}
	return matched
}

// scheduleTitleKey compares titles the way metadata matching does (normalized, ignoring
// spacing and a standalone "x" for "×"), but keeps the season: Anime1 lists each season as
// its own anime, so "轉生就是劍 第二季" must not match "轉生就是劍".
func scheduleTitleKey(title string) string {
	base, season, ok := metadataTitleParts(title)
	if !ok || base == "" {
		return ""
	}
	return metadataCompareKey(base) + "\x00" + season
}

// ScheduleSearchQuery is what to search for a show the app does not know: the part of the
// title before the first space (full-width too) when that part is Chinese or Japanese, which drops season
// and arc suffixes such as "第二季" or "(2026)"; otherwise the whole title.
func ScheduleSearchQuery(title string) string {
	title = strings.TrimSpace(title)
	end := strings.IndexFunc(title, unicode.IsSpace)
	if end < 0 {
		return title
	}
	head := title[:end]
	if utf8.RuneCountInString(head) < 2 || !strings.ContainsFunc(head, isHanOrKana) {
		return title
	}
	return head
}

func isHanOrKana(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana)
}
