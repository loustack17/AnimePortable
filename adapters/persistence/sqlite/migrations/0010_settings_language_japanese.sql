-- SPDX-License-Identifier: MPL-2.0

-- Allows language 3 (Japanese). SQLite cannot change a CHECK constraint, so the table is
-- rebuilt with the same columns, order and other constraints; every value is kept. No
-- foreign key, index, view or trigger references settings (see 0003).
CREATE TABLE settings_new (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    appearance INTEGER NOT NULL CHECK (appearance IN (0, 1, 2, 3)),
    autoplay_next INTEGER NOT NULL CHECK (autoplay_next IN (0, 1, 2)),
    resume_playback INTEGER NOT NULL CHECK (resume_playback IN (0, 1, 2)),
    language INTEGER NOT NULL CHECK (language IN (0, 1, 2, 3)),
    auto_play INTEGER NOT NULL DEFAULT 0 CHECK (auto_play IN (0, 1, 2)),
    playback_speed INTEGER NOT NULL DEFAULT 0 CHECK (playback_speed IN (0, 75, 100, 125, 150, 200)),
    hardware_decoding INTEGER NOT NULL DEFAULT 0 CHECK (hardware_decoding IN (0, 1, 2)),
    proxy_mode INTEGER NOT NULL DEFAULT 0 CHECK (proxy_mode IN (0, 1, 2, 3)),
    proxy_address TEXT NOT NULL DEFAULT '' CHECK (length(CAST(proxy_address AS BLOB)) <= 255)
);

INSERT INTO settings_new (id, appearance, autoplay_next, resume_playback, language, auto_play,
    playback_speed, hardware_decoding, proxy_mode, proxy_address)
SELECT id, appearance, autoplay_next, resume_playback, language, auto_play,
    playback_speed, hardware_decoding, proxy_mode, proxy_address FROM settings;

DROP TABLE settings;

ALTER TABLE settings_new RENAME TO settings;
