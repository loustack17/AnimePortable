// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"

	"animeportable/core"
)

func validSettings(settings core.Settings) bool {
	return settings.Appearance <= core.AppearanceDark &&
		settings.AutoplayNext <= core.ToggleEnabled &&
		settings.AutoPlay <= core.ToggleEnabled &&
		settings.ResumePlayback <= core.ToggleEnabled &&
		settings.Language <= core.LanguageJapanese &&
		settings.PlaybackSpeed.Valid() &&
		settings.HardwareDecoding <= core.ToggleEnabled &&
		settings.ProxyMode <= core.ProxyModeManual &&
		core.ValidProxyAddress(settings.ProxyAddress)
}

func (store *Store) SaveSettings(ctx context.Context, settings core.Settings) error {
	if !validSettings(settings) {
		return ErrInvalidInput
	}
	return store.withDB(ctx, func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `INSERT INTO settings (
			id, appearance, autoplay_next, auto_play, resume_playback, language,
			playback_speed, hardware_decoding, proxy_mode, proxy_address
		) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			appearance = excluded.appearance,
			autoplay_next = excluded.autoplay_next,
			auto_play = excluded.auto_play,
			resume_playback = excluded.resume_playback,
			language = excluded.language,
			playback_speed = excluded.playback_speed,
			hardware_decoding = excluded.hardware_decoding,
			proxy_mode = excluded.proxy_mode,
			proxy_address = excluded.proxy_address`,
			settings.Appearance, settings.AutoplayNext, settings.AutoPlay, settings.ResumePlayback, settings.Language,
			settings.PlaybackSpeed, settings.HardwareDecoding, settings.ProxyMode, settings.ProxyAddress)
		return err
	})
}

func (store *Store) Settings(ctx context.Context) (core.Settings, error) {
	var settings core.Settings
	err := store.withDB(ctx, func(db *sql.DB) error {
		var appearance, autoplayNext, autoPlay, resumePlayback, language, playbackSpeed, hardwareDecoding, proxyMode int
		var proxyAddress string
		err := db.QueryRowContext(ctx, `SELECT appearance, autoplay_next, auto_play, resume_playback, language,
				playback_speed, hardware_decoding, proxy_mode, proxy_address
			FROM settings WHERE id = 1`).Scan(
			&appearance, &autoplayNext, &autoPlay, &resumePlayback, &language,
			&playbackSpeed, &hardwareDecoding, &proxyMode, &proxyAddress,
		)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		// The enum-typed fields are narrower than int, so range-check before converting.
		if appearance < int(core.AppearanceUnspecified) || appearance > int(core.AppearanceDark) ||
			autoplayNext < int(core.ToggleUnspecified) || autoplayNext > int(core.ToggleEnabled) ||
			autoPlay < int(core.ToggleUnspecified) || autoPlay > int(core.ToggleEnabled) ||
			resumePlayback < int(core.ToggleUnspecified) || resumePlayback > int(core.ToggleEnabled) ||
			language < int(core.LanguageUnspecified) || language > int(core.LanguageJapanese) ||
			hardwareDecoding < int(core.ToggleUnspecified) || hardwareDecoding > int(core.ToggleEnabled) ||
			proxyMode < int(core.ProxyModeUnspecified) || proxyMode > int(core.ProxyModeManual) {
			return ErrStorage
		}
		settings.Appearance = core.Appearance(appearance)
		settings.AutoplayNext = core.Toggle(autoplayNext)
		settings.AutoPlay = core.Toggle(autoPlay)
		settings.ResumePlayback = core.Toggle(resumePlayback)
		settings.Language = core.Language(language)
		settings.PlaybackSpeed = core.PlaybackSpeed(playbackSpeed)
		settings.HardwareDecoding = core.Toggle(hardwareDecoding)
		settings.ProxyMode = core.ProxyMode(proxyMode)
		settings.ProxyAddress = proxyAddress
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
