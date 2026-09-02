// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"

	"animeportable/core"
)

func (store *Store) SetFollowing(ctx context.Context, animeID core.AnimeID, following bool) error {
	if !validIdentity(string(animeID)) {
		return ErrInvalidInput
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		if err := requireAnime(ctx, db, animeID); err != nil {
			return err
		}
		var err error
		if following {
			_, err = db.ExecContext(ctx, `INSERT INTO following (anime_id) VALUES (?) ON CONFLICT(anime_id) DO NOTHING`, animeID)
		} else {
			_, err = db.ExecContext(ctx, `DELETE FROM following WHERE anime_id = ?`, animeID)
		}
		return err
	})
}

func (store *Store) Following(ctx context.Context) ([]core.AnimeID, error) {
	items := make([]core.AnimeID, 0)
	err := store.withDB(ctx, func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, `SELECT anime_id FROM following ORDER BY anime_id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id core.AnimeID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			items = append(items, id)
		}
		return rows.Err()
	})
	if err != nil {
		return []core.AnimeID{}, err
	}
	return items, nil
}
