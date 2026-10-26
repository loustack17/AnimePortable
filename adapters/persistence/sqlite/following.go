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
	return queryAll(ctx, store, func(rows *sql.Rows) (core.FollowedAnime, error) {
		var item core.FollowedAnime
		var followedAt string
		if err := rows.Scan(&item.AnimeID, &followedAt); err != nil {
			return item, err
		}
		var err error
		item.FollowedAt, err = decodeStoredTime(followedAt)
		return item, err
	}, `SELECT anime_id, followed_at FROM following ORDER BY anime_id`)
}
