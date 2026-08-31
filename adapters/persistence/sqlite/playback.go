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
