// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"

	"animeportable/core"
)

// maxEpisodeNumberBytes matches the CHECK on episode_mappings.number.
const maxEpisodeNumberBytes = 128

func validEpisodeMapping(mapping core.EpisodeMapping) bool {
	return validIdentity(string(mapping.AnimeID)) &&
		validIdentity(string(mapping.EpisodeID)) &&
		validSourceRef(mapping.Ref.Anime) &&
		validIdentity(mapping.Ref.ID) &&
		len(mapping.Number) <= maxEpisodeNumberBytes
}

func (store *Store) SaveEpisodeMapping(ctx context.Context, mapping core.EpisodeMapping) error {
	if !validEpisodeMapping(mapping) {
		return ErrInvalidInput
	}
	firstSeen, err := encodeStoredTime(mapping.FirstSeenAt)
	if err != nil {
		return err
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
			anime_id, episode_id, provider, provider_anime_id, provider_episode_id, number, first_seen_at
		) SELECT ?, ?, ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1 FROM episode_mappings
			WHERE provider = ? AND provider_anime_id = ? AND provider_episode_id = ?
		)
		ON CONFLICT(anime_id, episode_id, provider, provider_anime_id, provider_episode_id) DO NOTHING`,
			mapping.AnimeID, mapping.EpisodeID, mapping.Ref.Anime.Provider, mapping.Ref.Anime.ID, mapping.Ref.ID, mapping.Number, firstSeen,
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
		// first_seen_at keeps the first save's time: a later save means the episode was
		// seen again, not that it is new.
		if mapping.Number == "" {
			// A caller without the number must not erase one learned earlier.
			return nil
		}
		_, err = db.ExecContext(ctx, `UPDATE episode_mappings SET number = ?
			WHERE provider = ? AND provider_anime_id = ? AND provider_episode_id = ?`,
			mapping.Number, mapping.Ref.Anime.Provider, mapping.Ref.Anime.ID, mapping.Ref.ID)
		return err
	})
}

func (store *Store) EpisodeMappings(ctx context.Context, animeID core.AnimeID) ([]core.EpisodeMapping, error) {
	if !validIdentity(string(animeID)) {
		return []core.EpisodeMapping{}, ErrInvalidInput
	}
	return queryAll(ctx, store, func(rows *sql.Rows) (core.EpisodeMapping, error) {
		mapping := core.EpisodeMapping{AnimeID: animeID}
		var firstSeen string
		if err := rows.Scan(&mapping.EpisodeID, &mapping.Ref.Anime.Provider, &mapping.Ref.Anime.ID, &mapping.Ref.ID, &mapping.Number, &firstSeen); err != nil {
			return mapping, err
		}
		var err error
		mapping.FirstSeenAt, err = decodeStoredTime(firstSeen)
		return mapping, err
	}, `SELECT episode_id, provider, provider_anime_id, provider_episode_id, number, first_seen_at
			FROM episode_mappings
			WHERE anime_id = ?
			ORDER BY episode_id, provider, provider_anime_id, provider_episode_id`, animeID)
}
