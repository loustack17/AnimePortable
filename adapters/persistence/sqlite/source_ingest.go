// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"

	"animeportable/core"
)

func (store *Store) IngestSourceAnime(ctx context.Context, candidate core.Anime, ref core.SourceRef) (core.Anime, error) {
	if !validAnime(candidate) || !validSourceRef(ref) {
		return core.Anime{}, ErrInvalidInput
	}
	var result core.Anime
	err := store.withDB(ctx, func(db *sql.DB) error {
		conn, err := db.Conn(ctx)
		if err != nil {
			return err
		}
		defer conn.Close()
		if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			return err
		}
		committed := false
		defer func() {
			if !committed {
				rollbackCheckpoint(conn)
			}
		}()
		err = conn.QueryRowContext(ctx, `SELECT a.id, a.title, a.native_title, a.description
			FROM anime a JOIN source_refs r ON r.anime_id = a.id
			WHERE r.provider = ? AND r.external_id = ?`, ref.Provider, ref.ID).
			Scan(&result.ID, &result.Title, &result.NativeTitle, &result.Description)
		switch err {
		case nil:
			result.Title, result.NativeTitle = candidate.Title, candidate.NativeTitle
			_, err = conn.ExecContext(ctx, `UPDATE anime SET title = ?, native_title = ? WHERE id = ?`, result.Title, result.NativeTitle, result.ID)
		case sql.ErrNoRows:
			var existing int
			err = conn.QueryRowContext(ctx, `SELECT 1 FROM anime WHERE id = ?`, candidate.ID).Scan(&existing)
			if err == nil {
				return ErrIdentityConflict
			}
			if err != sql.ErrNoRows {
				return err
			}
			result = candidate
			if _, err = conn.ExecContext(ctx, `INSERT INTO anime (id, title, native_title, description) VALUES (?, ?, ?, ?)`, result.ID, result.Title, result.NativeTitle, result.Description); err != nil {
				return err
			}
			_, err = conn.ExecContext(ctx, `INSERT INTO source_refs (anime_id, provider, external_id) VALUES (?, ?, ?)`, result.ID, ref.Provider, ref.ID)
		}
		if err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return err
		}
		committed = true
		return nil
	})
	if err != nil {
		return core.Anime{}, err
	}
	return result, nil
}
