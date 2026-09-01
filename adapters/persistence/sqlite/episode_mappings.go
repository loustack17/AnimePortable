package sqlite

import (
	"context"
	"database/sql"

	"animeportable/core"
)

func validEpisodeMapping(mapping core.EpisodeMapping) bool {
	return validIdentity(string(mapping.AnimeID)) &&
		validIdentity(string(mapping.EpisodeID)) &&
		validSourceRef(mapping.Ref.Anime) &&
		validIdentity(mapping.Ref.ID)
}

func (store *Store) SaveEpisodeMapping(ctx context.Context, mapping core.EpisodeMapping) error {
	if !validEpisodeMapping(mapping) {
		return ErrInvalidInput
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		if err := requireAnime(ctx, db, mapping.AnimeID); err != nil {
			return err
		}
		var found int
		err := db.QueryRowContext(ctx, `SELECT 1 FROM source_refs
			WHERE anime_id = ? AND provider = ? AND external_id = ?`,
			mapping.AnimeID, mapping.Ref.Anime.Provider, mapping.Ref.Anime.ID).Scan(&found)
		if err == sql.ErrNoRows {
			return ErrInvalidInput
		}
		if err != nil {
			return err
		}

		result, err := db.ExecContext(ctx, `INSERT INTO episode_mappings (
			anime_id, episode_id, provider, provider_anime_id, provider_episode_id
		) SELECT ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1 FROM episode_mappings
			WHERE provider = ? AND provider_anime_id = ? AND provider_episode_id = ?
		)
		ON CONFLICT(anime_id, episode_id, provider, provider_anime_id, provider_episode_id) DO NOTHING`,
			mapping.AnimeID, mapping.EpisodeID, mapping.Ref.Anime.Provider, mapping.Ref.Anime.ID, mapping.Ref.ID,
			mapping.Ref.Anime.Provider, mapping.Ref.Anime.ID, mapping.Ref.ID)
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

		var ownerAnime core.AnimeID
		var ownerEpisode core.EpisodeID
		err = db.QueryRowContext(ctx, `SELECT anime_id, episode_id FROM episode_mappings
			WHERE provider = ? AND provider_anime_id = ? AND provider_episode_id = ?`,
			mapping.Ref.Anime.Provider, mapping.Ref.Anime.ID, mapping.Ref.ID).Scan(&ownerAnime, &ownerEpisode)
		if err == sql.ErrNoRows {
			return core.ErrNotFound
		}
		if err != nil {
			return err
		}
		if ownerAnime != mapping.AnimeID || ownerEpisode != mapping.EpisodeID {
			return ErrIdentityConflict
		}
		return nil
	})
}

func (store *Store) EpisodeMappings(ctx context.Context, animeID core.AnimeID) ([]core.EpisodeMapping, error) {
	items := make([]core.EpisodeMapping, 0)
	if !validIdentity(string(animeID)) {
		return items, ErrInvalidInput
	}
	err := store.withDB(ctx, func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, `SELECT episode_id, provider, provider_anime_id, provider_episode_id
			FROM episode_mappings
			WHERE anime_id = ?
			ORDER BY episode_id, provider, provider_anime_id, provider_episode_id`, animeID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var mapping core.EpisodeMapping
			mapping.AnimeID = animeID
			mapping.Ref.Anime = core.SourceRef{}
			if err := rows.Scan(&mapping.EpisodeID, &mapping.Ref.Anime.Provider, &mapping.Ref.Anime.ID, &mapping.Ref.ID); err != nil {
				return err
			}
			items = append(items, mapping)
		}
		return rows.Err()
	})
	if err != nil {
		return []core.EpisodeMapping{}, err
	}
	return items, nil
}
