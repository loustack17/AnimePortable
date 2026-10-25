// SPDX-License-Identifier: MPL-2.0

package bangumi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	metadatainternal "animeportable/adapters/metadata/internal"
	"animeportable/core"
	metadatapolicy "animeportable/internal/metadata"
)

const calendarURL = apiOrigin + "/calendar"

// Schedule lists the current weekly airing schedule from Bangumi's calendar. Days and the
// items inside a day keep the source order. Items that cannot be mapped are skipped, never
// fatal; only a malformed envelope or an over-long listing rejects the whole response.
func (client *Client) Schedule(ctx context.Context) ([]core.ScheduleEntry, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	response, err := client.doRequest(ctx, http.MethodGet, calendarURL, nil, false)
	if err != nil {
		return nil, err
	}
	wrapped, ok := wrapCalendar(response.Body)
	if !ok {
		return nil, errMetadataMalformed
	}
	var calendar calendarResponse
	if err := responseRules.Decode(wrapped, &calendar); err != nil {
		return nil, err
	}
	type scheduleKey struct {
		id      string
		weekday time.Weekday
	}
	seen := make(map[scheduleKey]struct{})
	entries := []core.ScheduleEntry{}
	for _, day := range calendar.Days {
		for _, raw := range day.Items {
			entry, ok := mapScheduleItem(raw)
			if !ok {
				continue
			}
			key := scheduleKey{id: entry.Ref.ID, weekday: entry.Weekday}
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			if len(entries) >= core.MaxScheduleEntries {
				return nil, errMetadataMalformed
			}
			entries = append(entries, entry)
		}
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return entries, nil
}

// wrapCalendar puts the calendar's top-level array in an object, because responseRules.Decode
// (duplicate-key and nesting checks included) only accepts objects. It reports false for a
// body that is not an array.
func wrapCalendar(body []byte) ([]byte, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, false
	}
	return append(append([]byte(`{"days":`), trimmed...), '}'), true
}

func mapScheduleItem(raw json.RawMessage) (core.ScheduleEntry, bool) {
	var item calendarItem
	if json.Unmarshal(raw, &item) != nil {
		return core.ScheduleEntry{}, false
	}
	id, _, ok := metadatainternal.ParseJSONID(item.ID)
	if !ok || item.Type == nil || *item.Type != 2 {
		return core.ScheduleEntry{}, false
	}
	if item.AirWeekday == nil || *item.AirWeekday < 1 || *item.AirWeekday > 7 {
		return core.ScheduleEntry{}, false
	}
	title, nativeTitle, ok := mapTitles(item.NameCN, item.Name)
	if !ok {
		return core.ScheduleEntry{}, false
	}
	// Without a Chinese name the title is Bangumi's original name, which for a Chinese work
	// is Simplified Chinese; it is converted like a summary, and Japanese stays as is.
	if (item.NameCN == nil || strings.TrimSpace(*item.NameCN) == "") && !isJapanese(title) {
		if title, ok = toTraditional(title); !ok {
			return core.ScheduleEntry{}, false
		}
	}
	// The store accepts only canonical text; one odd title must not block the whole schedule.
	limits := metadatapolicy.TitleLimits()
	if !metadatapolicy.IsCanonicalPlainText(title, limits) || !metadatapolicy.IsCanonicalPlainText(nativeTitle, limits) {
		return core.ScheduleEntry{}, false
	}
	return core.ScheduleEntry{
		Ref:         core.MetadataRef{Provider: providerID, ID: id},
		Title:       title,
		NativeTitle: nativeTitle,
		// Bangumi counts 1 = Monday ... 7 = Sunday; time.Weekday counts Sunday as 0.
		Weekday: time.Weekday(*item.AirWeekday % 7),
	}, true
}

type calendarResponse struct {
	Days []calendarDay `json:"days"`
}

type calendarDay struct {
	// Items stay raw so one malformed item is skipped instead of rejecting the schedule.
	Items []json.RawMessage `json:"items"`
}

type calendarItem struct {
	ID         json.RawMessage `json:"id"`
	Type       *int            `json:"type"`
	Name       *string         `json:"name"`
	NameCN     *string         `json:"name_cn"`
	AirWeekday *int            `json:"air_weekday"`
}

var _ core.ScheduleProvider = (*Client)(nil)
