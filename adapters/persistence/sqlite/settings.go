// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"

	"animeportable/core"
)

const maxMPVPathBytes = 8192

func validSettings(settings core.Settings) bool {
	return settings.Appearance <= core.AppearanceDark &&
		validText(settings.MPVPath, maxMPVPathBytes) &&
		settings.AutoplayNext <= core.ToggleEnabled &&
		settings.ResumePlayback <= core.ToggleEnabled &&
		settings.Language <= core.LanguageEnglish
}

func (store *Store) SaveSettings(ctx context.Context, settings core.Settings) error {
	if !validSettings(settings) {
		return ErrInvalidInput
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `INSERT INTO settings (
			id, appearance, mpv_path, autoplay_next, resume_playback, language
		) VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			appearance = excluded.appearance,
			mpv_path = excluded.mpv_path,
			autoplay_next = excluded.autoplay_next,
			resume_playback = excluded.resume_playback,
			language = excluded.language`,
			settings.Appearance, settings.MPVPath, settings.AutoplayNext, settings.ResumePlayback, settings.Language)
		return err
	})
}

func (store *Store) Settings(ctx context.Context) (core.Settings, error) {
	settings := core.DefaultSettings()
	err := store.withDB(ctx, func(db *sql.DB) error {
		var appearance, autoplayNext, resumePlayback, language int
		err := db.QueryRowContext(ctx, `SELECT appearance, mpv_path, autoplay_next, resume_playback, language
			FROM settings WHERE id = 1`).Scan(
			&appearance, &settings.MPVPath, &autoplayNext, &resumePlayback, &language,
		)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		if appearance < int(core.AppearanceUnspecified) || appearance > int(core.AppearanceDark) ||
			autoplayNext < int(core.ToggleUnspecified) || autoplayNext > int(core.ToggleEnabled) ||
			resumePlayback < int(core.ToggleUnspecified) || resumePlayback > int(core.ToggleEnabled) ||
			language < int(core.LanguageUnspecified) || language > int(core.LanguageEnglish) {
			return ErrStorage
		}
		settings.Appearance = core.Appearance(appearance)
		settings.AutoplayNext = core.Toggle(autoplayNext)
		settings.ResumePlayback = core.Toggle(resumePlayback)
		settings.Language = core.Language(language)
		if !validSettings(settings) {
			return ErrStorage
		}
		return nil
	})
	if err != nil {
		return core.Settings{}, err
	}
	return settings, nil
}
