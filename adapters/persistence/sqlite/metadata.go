package sqlite

import (
	"context"
	"database/sql"

	"animeportable/core"
)

func animeExists(ctx context.Context, db *sql.DB, id core.AnimeID) (bool, error) {
	var found int
	err := db.QueryRowContext(ctx, `SELECT 1 FROM anime WHERE id = ?`, id).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func requireAnime(ctx context.Context, db *sql.DB, id core.AnimeID) error {
	exists, err := animeExists(ctx, db, id)
	if err != nil {
		return err
	}
	if !exists {
		return ErrInvalidInput
	}
	return nil
}

func (store *Store) SaveSourceRef(ctx context.Context, animeID core.AnimeID, ref core.SourceRef) error {
	if !validIdentity(string(animeID)) || !validSourceRef(ref) {
		return ErrInvalidInput
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		if err := requireAnime(ctx, db, animeID); err != nil {
			return err
		}
		result, err := db.ExecContext(ctx, `INSERT INTO source_refs (anime_id, provider, external_id)
			SELECT ?, ?, ?
			WHERE NOT EXISTS (
				SELECT 1 FROM source_refs
				WHERE provider = ? AND external_id = ? AND anime_id <> ?
			)
			ON CONFLICT(anime_id, provider, external_id) DO NOTHING`,
			animeID, ref.Provider, ref.ID, ref.Provider, ref.ID, animeID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected > 0 {
			return nil
		}
		var owner core.AnimeID
		err = db.QueryRowContext(ctx, `SELECT anime_id FROM source_refs WHERE provider = ? AND external_id = ?`, ref.Provider, ref.ID).Scan(&owner)
		if err != nil {
			return err
		}
		if owner != animeID {
			return ErrIdentityConflict
		}
		return nil
	})
}

func (store *Store) SourceRefs(ctx context.Context, animeID core.AnimeID) ([]core.SourceRef, error) {
	if !validIdentity(string(animeID)) {
		return []core.SourceRef{}, ErrInvalidInput
	}
	items := make([]core.SourceRef, 0)
	err := store.withDB(ctx, func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, `SELECT provider, external_id FROM source_refs WHERE anime_id = ? ORDER BY provider, external_id`, animeID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ref core.SourceRef
			if err := rows.Scan(&ref.Provider, &ref.ID); err != nil {
				return err
			}
			items = append(items, ref)
		}
		return rows.Err()
	})
	if err != nil {
		return []core.SourceRef{}, err
	}
	return items, nil
}

func (store *Store) SaveMetadata(ctx context.Context, animeID core.AnimeID, metadata core.AnimeMetadata) error {
	if !validIdentity(string(animeID)) || !validMetadata(metadata) {
		return ErrInvalidInput
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		if err := requireAnime(ctx, db, animeID); err != nil {
			return err
		}
		result, err := db.ExecContext(ctx, `INSERT INTO metadata (
			anime_id, provider, external_id, title, native_title, description, cover_url, season, year, studio, episode_count
		) SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1 FROM metadata
			WHERE provider = ? AND external_id = ? AND anime_id <> ?
		)
		ON CONFLICT(anime_id) DO UPDATE SET
			provider = excluded.provider,
			external_id = excluded.external_id,
			title = excluded.title,
			native_title = excluded.native_title,
			description = excluded.description,
			cover_url = excluded.cover_url,
			season = excluded.season,
			year = excluded.year,
			studio = excluded.studio,
			episode_count = excluded.episode_count
		WHERE NOT EXISTS (
			SELECT 1 FROM metadata AS existing
			WHERE existing.provider = excluded.provider
				AND existing.external_id = excluded.external_id
				AND existing.anime_id <> excluded.anime_id
		)`,
			animeID, metadata.Ref.Provider, metadata.Ref.ID, metadata.Title, metadata.NativeTitle,
			metadata.Description, metadata.CoverURL, metadata.Season, metadata.Year, metadata.Studio, metadata.EpisodeCount,
			metadata.Ref.Provider, metadata.Ref.ID, animeID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrIdentityConflict
		}
		return nil
	})
}

func (store *Store) Metadata(ctx context.Context, animeID core.AnimeID) (core.AnimeMetadata, error) {
	if !validIdentity(string(animeID)) {
		return core.AnimeMetadata{}, ErrInvalidInput
	}
	var metadata core.AnimeMetadata
	err := store.withDB(ctx, func(db *sql.DB) error {
		err := db.QueryRowContext(ctx, `SELECT provider, external_id, title, native_title, description, cover_url, season, year, studio, episode_count
			FROM metadata WHERE anime_id = ?`, animeID).Scan(
			&metadata.Ref.Provider, &metadata.Ref.ID, &metadata.Title, &metadata.NativeTitle,
			&metadata.Description, &metadata.CoverURL, &metadata.Season, &metadata.Year,
			&metadata.Studio, &metadata.EpisodeCount,
		)
		if err == sql.ErrNoRows {
			return core.ErrNotFound
		}
		return err
	})
	if err != nil {
		return core.AnimeMetadata{}, err
	}
	return metadata, nil
}
