package sqlite

import (
	"context"
	"database/sql"

	"animeportable/core"
)

func validEpisodeID(id core.EpisodeID) bool {
	return validIdentity(string(id))
}

func validPlaybackProgress(progress core.PlaybackProgress) bool {
	return validIdentity(string(progress.AnimeID)) &&
		validEpisodeID(progress.EpisodeID) &&
		progress.Position >= 0 &&
		progress.Duration >= 0
}

type playbackCheckpointRow struct {
	position     int64
	duration     int64
	completed    bool
	updatedAt    string
	lastPlayedAt string
}

type checkpointQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadProgressCheckpoint(ctx context.Context, queryer checkpointQueryer, animeID core.AnimeID, episodeID core.EpisodeID) (playbackCheckpointRow, bool, error) {
	var row playbackCheckpointRow
	var completed int
	err := queryer.QueryRowContext(ctx, `SELECT position_ns, duration_ns, completed, updated_at
		FROM playback_progress WHERE anime_id = ? AND episode_id = ?`, animeID, episodeID).Scan(
		&row.position, &row.duration, &completed, &row.updatedAt,
	)
	if err == sql.ErrNoRows {
		return playbackCheckpointRow{}, false, nil
	}
	if err != nil {
		return playbackCheckpointRow{}, false, err
	}
	if row.position < 0 || row.duration < 0 || completed < 0 || completed > 1 {
		return playbackCheckpointRow{}, false, ErrStorage
	}
	if _, err := decodeStoredTime(row.updatedAt); err != nil {
		return playbackCheckpointRow{}, false, err
	}
	row.completed = completed != 0
	return row, true, nil
}

func loadHistoryCheckpoint(ctx context.Context, queryer checkpointQueryer, animeID core.AnimeID, episodeID core.EpisodeID) (playbackCheckpointRow, bool, error) {
	var row playbackCheckpointRow
	var completed int
	err := queryer.QueryRowContext(ctx, `SELECT position_ns, duration_ns, completed, updated_at, last_played_at
		FROM playback_history WHERE anime_id = ? AND episode_id = ?`, animeID, episodeID).Scan(
		&row.position, &row.duration, &completed, &row.updatedAt, &row.lastPlayedAt,
	)
	if err == sql.ErrNoRows {
		return playbackCheckpointRow{}, false, nil
	}
	if err != nil {
		return playbackCheckpointRow{}, false, err
	}
	if row.position < 0 || row.duration < 0 || completed < 0 || completed > 1 {
		return playbackCheckpointRow{}, false, ErrStorage
	}
	if _, err := decodeStoredTime(row.updatedAt); err != nil {
		return playbackCheckpointRow{}, false, err
	}
	if _, err := decodeStoredTime(row.lastPlayedAt); err != nil {
		return playbackCheckpointRow{}, false, err
	}
	row.completed = completed != 0
	return row, true, nil
}

func mergeCheckpointProgress(current playbackCheckpointRow, hasCurrent bool, incoming playbackCheckpointRow) playbackCheckpointRow {
	if !hasCurrent {
		return incoming
	}
	completed := current.completed || incoming.completed
	if incoming.updatedAt > current.updatedAt {
		incoming.completed = completed
		return incoming
	}
	if incoming.updatedAt < current.updatedAt {
		current.completed = completed
		return current
	}
	if incoming.position > current.position {
		current.position = incoming.position
	}
	if incoming.duration > current.duration {
		current.duration = incoming.duration
	}
	current.completed = completed
	return current
}

func maxStoredTime(current, incoming string) string {
	if incoming > current {
		return incoming
	}
	return current
}

func rollbackCheckpoint(conn *sql.Conn) {
	rollbackContext, cancel := context.WithTimeout(context.Background(), migrationRollbackTimeout)
	defer cancel()
	_, _ = conn.ExecContext(rollbackContext, "ROLLBACK")
}

func savePlaybackCheckpoint(ctx context.Context, db *sql.DB, entry core.HistoryEntry, updatedAt, lastPlayedAt string) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	began := false
	committed := false
	defer func() {
		if began && !committed {
			rollbackCheckpoint(conn)
		}
		_ = conn.Close()
	}()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	began = true
	if err := requireAnimeWithQueryer(ctx, conn, entry.Progress.AnimeID); err != nil {
		return err
	}
	progressRow, hasProgress, err := loadProgressCheckpoint(ctx, conn, entry.Progress.AnimeID, entry.Progress.EpisodeID)
	if err != nil {
		return err
	}
	historyRow, hasHistory, err := loadHistoryCheckpoint(ctx, conn, entry.Progress.AnimeID, entry.Progress.EpisodeID)
	if err != nil {
		return err
	}
	incoming := playbackCheckpointRow{
		position:     int64(entry.Progress.Position),
		duration:     int64(entry.Progress.Duration),
		completed:    entry.Progress.Completed,
		updatedAt:    updatedAt,
		lastPlayedAt: lastPlayedAt,
	}
	merged := mergeCheckpointProgress(progressRow, hasProgress, historyRow)
	merged = mergeCheckpointProgress(merged, hasProgress || hasHistory, incoming)
	merged.lastPlayedAt = maxStoredTime(historyRow.lastPlayedAt, lastPlayedAt)
	if _, err := conn.ExecContext(ctx, `INSERT INTO playback_progress (
		anime_id, episode_id, position_ns, duration_ns, completed, updated_at
	) VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(anime_id, episode_id) DO UPDATE SET
		position_ns = excluded.position_ns,
		duration_ns = excluded.duration_ns,
		completed = excluded.completed,
		updated_at = excluded.updated_at`,
		entry.Progress.AnimeID, entry.Progress.EpisodeID, merged.position, merged.duration,
		merged.completed, merged.updatedAt); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO playback_history (
		anime_id, episode_id, position_ns, duration_ns, completed, updated_at, last_played_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(anime_id, episode_id) DO UPDATE SET
		position_ns = excluded.position_ns,
		duration_ns = excluded.duration_ns,
		completed = excluded.completed,
		updated_at = excluded.updated_at,
		last_played_at = excluded.last_played_at`,
		entry.Progress.AnimeID, entry.Progress.EpisodeID, merged.position, merged.duration,
		merged.completed, merged.updatedAt, merged.lastPlayedAt); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

func requireAnimeWithQueryer(ctx context.Context, queryer checkpointQueryer, id core.AnimeID) error {
	var found int
	err := queryer.QueryRowContext(ctx, `SELECT 1 FROM anime WHERE id = ?`, id).Scan(&found)
	if err == sql.ErrNoRows {
		return ErrInvalidInput
	}
	return err
}

func (store *Store) SavePlaybackCheckpoint(ctx context.Context, entry core.HistoryEntry) error {
	if !validPlaybackProgress(entry.Progress) {
		return ErrInvalidInput
	}
	updatedAt, err := encodeStoredTime(entry.Progress.UpdatedAt)
	if err != nil {
		return err
	}
	lastPlayedAt, err := encodeStoredTime(entry.LastPlayedAt)
	if err != nil {
		return err
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		return savePlaybackCheckpoint(ctx, db, entry, updatedAt, lastPlayedAt)
	})
}

func (store *Store) AddHistory(ctx context.Context, entry core.HistoryEntry) error {
	if !validPlaybackProgress(entry.Progress) {
		return ErrInvalidInput
	}
	updatedAt, err := encodeStoredTime(entry.Progress.UpdatedAt)
	if err != nil {
		return err
	}
	lastPlayedAt, err := encodeStoredTime(entry.LastPlayedAt)
	if err != nil {
		return err
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		if err := requireAnime(ctx, db, entry.Progress.AnimeID); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, `INSERT INTO playback_history (
			anime_id, episode_id, position_ns, duration_ns, completed, updated_at, last_played_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(anime_id, episode_id) DO UPDATE SET
			position_ns = excluded.position_ns,
			duration_ns = excluded.duration_ns,
			completed = excluded.completed,
			updated_at = excluded.updated_at,
			last_played_at = excluded.last_played_at`,
			entry.Progress.AnimeID, entry.Progress.EpisodeID, entry.Progress.Position, entry.Progress.Duration,
			entry.Progress.Completed, updatedAt, lastPlayedAt)
		return err
	})
}

func (store *Store) History(ctx context.Context) ([]core.HistoryEntry, error) {
	items := make([]core.HistoryEntry, 0)
	err := store.withDB(ctx, func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, `SELECT
			anime_id, episode_id, position_ns, duration_ns, completed, updated_at, last_played_at
			FROM playback_history
			ORDER BY last_played_at DESC, anime_id, episode_id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var entry core.HistoryEntry
			var updatedAt, lastPlayedAt string
			if err := rows.Scan(
				&entry.Progress.AnimeID, &entry.Progress.EpisodeID, &entry.Progress.Position,
				&entry.Progress.Duration, &entry.Progress.Completed, &updatedAt, &lastPlayedAt,
			); err != nil {
				return err
			}
			entry.Progress.UpdatedAt, err = decodeStoredTime(updatedAt)
			if err != nil {
				return err
			}
			entry.LastPlayedAt, err = decodeStoredTime(lastPlayedAt)
			if err != nil {
				return err
			}
			items = append(items, entry)
		}
		return rows.Err()
	})
	if err != nil {
		return []core.HistoryEntry{}, err
	}
	return items, nil
}

func (store *Store) RemoveHistory(ctx context.Context, animeID core.AnimeID) error {
	if !validIdentity(string(animeID)) {
		return ErrInvalidInput
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		if err := requireAnime(ctx, db, animeID); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, `DELETE FROM playback_history WHERE anime_id = ?`, animeID)
		return err
	})
}

func (store *Store) SaveProgress(ctx context.Context, progress core.PlaybackProgress) error {
	if !validPlaybackProgress(progress) {
		return ErrInvalidInput
	}
	updatedAt, err := encodeStoredTime(progress.UpdatedAt)
	if err != nil {
		return err
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		if err := requireAnime(ctx, db, progress.AnimeID); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, `INSERT INTO playback_progress (
			anime_id, episode_id, position_ns, duration_ns, completed, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(anime_id, episode_id) DO UPDATE SET
			position_ns = excluded.position_ns,
			duration_ns = excluded.duration_ns,
			completed = excluded.completed,
			updated_at = excluded.updated_at`,
			progress.AnimeID, progress.EpisodeID, progress.Position, progress.Duration,
			progress.Completed, updatedAt)
		return err
	})
}

func (store *Store) Progress(ctx context.Context, animeID core.AnimeID, episodeID core.EpisodeID) (core.PlaybackProgress, error) {
	if !validIdentity(string(animeID)) || !validEpisodeID(episodeID) {
		return core.PlaybackProgress{}, ErrInvalidInput
	}
	var progress core.PlaybackProgress
	err := store.withDB(ctx, func(db *sql.DB) error {
		var updatedAt string
		err := db.QueryRowContext(ctx, `SELECT position_ns, duration_ns, completed, updated_at
			FROM playback_progress WHERE anime_id = ? AND episode_id = ?`, animeID, episodeID).Scan(
			&progress.Position, &progress.Duration, &progress.Completed, &updatedAt,
		)
		if err == sql.ErrNoRows {
			return core.ErrNotFound
		}
		if err != nil {
			return err
		}
		progress.AnimeID = animeID
		progress.EpisodeID = episodeID
		progress.UpdatedAt, err = decodeStoredTime(updatedAt)
		return err
	})
	if err != nil {
		return core.PlaybackProgress{}, err
	}
	return progress, nil
}
