// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"

	"animeportable/core"
)

func (store *Store) SaveAnime(ctx context.Context, anime core.Anime) error {
	if !validAnime(anime) {
		return ErrInvalidInput
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `INSERT INTO anime (id, title, native_title, description)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				title = excluded.title,
				native_title = excluded.native_title,
				description = excluded.description`,
			anime.ID, anime.Title, anime.NativeTitle, anime.Description)
		return err
	})
}

func (store *Store) Anime(ctx context.Context, id core.AnimeID) (core.Anime, error) {
	if !validIdentity(string(id)) {
		return core.Anime{}, ErrInvalidInput
	}
	var anime core.Anime
	err := store.withDB(ctx, func(db *sql.DB) error {
		err := db.QueryRowContext(ctx, `SELECT id, title, native_title, description FROM anime WHERE id = ?`, id).
			Scan(&anime.ID, &anime.Title, &anime.NativeTitle, &anime.Description)
		if err == sql.ErrNoRows {
			return core.ErrNotFound
		}
		return err
	})
	if err != nil {
		return core.Anime{}, err
	}
	return anime, nil
}

func (store *Store) ListAnime(ctx context.Context) ([]core.Anime, error) {
	items := make([]core.Anime, 0)
	err := store.withDB(ctx, func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, `SELECT id, title, native_title, description FROM anime ORDER BY id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var anime core.Anime
			if err := rows.Scan(&anime.ID, &anime.Title, &anime.NativeTitle, &anime.Description); err != nil {
				return err
			}
			items = append(items, anime)
		}
		return rows.Err()
	})
	if err != nil {
		return []core.Anime{}, err
	}
	return items, nil
}
