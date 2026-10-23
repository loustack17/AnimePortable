// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"

	"animeportable/core"
)

// animeColumns are read into animeFields, in this order.
const animeColumns = `id, title, native_title, description, catalog_year, catalog_season, catalog_episode_count`

func animeFields(anime *core.Anime) []any {
	return []any{&anime.ID, &anime.Title, &anime.NativeTitle, &anime.Description, &anime.Catalog.Year, &anime.Catalog.AiringSeason, &anime.Catalog.EpisodeCount}
}

func (store *Store) SaveAnime(ctx context.Context, anime core.Anime) error {
	if !validAnime(anime) {
		return ErrInvalidInput
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `INSERT INTO anime (id, title, native_title, description, catalog_year, catalog_season, catalog_episode_count)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				title = excluded.title,
				native_title = excluded.native_title,
				description = excluded.description,
				catalog_year = excluded.catalog_year,
				catalog_season = excluded.catalog_season,
				catalog_episode_count = excluded.catalog_episode_count`,
			anime.ID, anime.Title, anime.NativeTitle, anime.Description, anime.Catalog.Year, anime.Catalog.AiringSeason, anime.Catalog.EpisodeCount)
		return err
	})
}

func (store *Store) Anime(ctx context.Context, id core.AnimeID) (core.Anime, error) {
	if !validIdentity(string(id)) {
		return core.Anime{}, ErrInvalidInput
	}
	var anime core.Anime
	err := store.withDB(ctx, func(db *sql.DB) error {
		err := db.QueryRowContext(ctx, `SELECT `+animeColumns+` FROM anime WHERE id = ?`, id).Scan(animeFields(&anime)...)
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
		rows, err := db.QueryContext(ctx, `SELECT `+animeColumns+` FROM anime ORDER BY id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var anime core.Anime
			if err := rows.Scan(animeFields(&anime)...); err != nil {
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
