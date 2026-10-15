// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"animeportable/core"
)

func (store *Store) SetFollowing(ctx context.Context, animeID core.AnimeID, following bool) error {
	if !validIdentity(string(animeID)) {
		return ErrInvalidInput
	}
	followedAt, err := encodeStoredTime(time.Now())
	if err != nil {
		return err
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		if err := requireAnime(ctx, db, animeID); err != nil {
			return err
		}
		var err error
		if following {
			// ON CONFLICT keeps the original time, so following again does not hide updates.
			_, err = db.ExecContext(ctx, `INSERT INTO following (anime_id, followed_at) VALUES (?, ?) ON CONFLICT(anime_id) DO NOTHING`, animeID, followedAt)
		} else {
			_, err = db.ExecContext(ctx, `DELETE FROM following WHERE anime_id = ?`, animeID)
		}
		return err
	})
}

func (store *Store) Following(ctx context.Context) ([]core.FollowedAnime, error) {
	items := make([]core.FollowedAnime, 0)
	err := store.withDB(ctx, func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, `SELECT anime_id, followed_at FROM following ORDER BY anime_id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item core.FollowedAnime
			var followedAt string
			if err := rows.Scan(&item.AnimeID, &followedAt); err != nil {
				return err
			}
			if item.FollowedAt, err = decodeStoredTime(followedAt); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	if err != nil {
		return []core.FollowedAnime{}, err
	}
	return items, nil
}
