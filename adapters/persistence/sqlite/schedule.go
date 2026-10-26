// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"animeportable/core"
	metadatapolicy "animeportable/internal/metadata"
)

type scheduleKey struct {
	ref     core.MetadataRef
	weekday time.Weekday
}

func validScheduleEntry(entry core.ScheduleEntry) bool {
	return validIdentity(entry.Ref.Provider) &&
		validIdentity(entry.Ref.ID) &&
		metadatapolicy.IsCanonicalPlainText(entry.Title, metadatapolicy.TitleLimits()) &&
		metadatapolicy.IsCanonicalPlainText(entry.NativeTitle, metadatapolicy.TitleLimits()) &&
		(entry.Title != "" || entry.NativeTitle != "") &&
		entry.Weekday >= time.Sunday && entry.Weekday <= time.Saturday
}

// ReplaceSchedule swaps the whole stored schedule in one transaction, so a failed refresh
// leaves the previous schedule untouched. AnimeID is not stored; Schedule derives it.
func (store *Store) ReplaceSchedule(ctx context.Context, entries []core.ScheduleEntry) error {
	if len(entries) > core.MaxScheduleEntries {
		return ErrInvalidInput
	}
	seen := make(map[scheduleKey]struct{}, len(entries))
	for _, entry := range entries {
		if !validScheduleEntry(entry) {
			return ErrInvalidInput
		}
		key := scheduleKey{ref: entry.Ref, weekday: entry.Weekday}
		if _, duplicate := seen[key]; duplicate {
			return ErrInvalidInput
		}
		seen[key] = struct{}{}
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(ctx, `DELETE FROM schedule_entries`); err != nil {
			return err
		}
		for position, entry := range entries {
			if _, err := tx.ExecContext(ctx, `INSERT INTO schedule_entries (provider, external_id, title, native_title, weekday, position)
				VALUES (?, ?, ?, ?, ?, ?)`,
				entry.Ref.Provider, entry.Ref.ID, entry.Title, entry.NativeTitle, int(entry.Weekday), position); err != nil {
				return err
			}
		}
		return tx.Commit()
	})
}

// Schedule lists the stored entries Sunday to Saturday, in source order within a day. A
// row that no longer validates fails the read instead of being shown.
func (store *Store) Schedule(ctx context.Context) ([]core.ScheduleEntry, error) {
	return queryAll(ctx, store, func(rows *sql.Rows) (core.ScheduleEntry, error) {
		var entry core.ScheduleEntry
		var weekday int
		var animeID sql.NullString
		if err := rows.Scan(&entry.Ref.Provider, &entry.Ref.ID, &entry.Title, &entry.NativeTitle, &weekday, &animeID); err != nil {
			return entry, err
		}
		entry.Weekday = time.Weekday(weekday)
		entry.AnimeID = core.AnimeID(animeID.String)
		if !validScheduleEntry(entry) {
			return entry, ErrStorage
		}
		return entry, nil
	}, `SELECT s.provider, s.external_id, s.title, s.native_title, s.weekday, m.anime_id
			FROM schedule_entries AS s
			LEFT JOIN metadata AS m ON m.provider = s.provider AND m.external_id = s.external_id
			ORDER BY s.weekday, s.position`)
}
