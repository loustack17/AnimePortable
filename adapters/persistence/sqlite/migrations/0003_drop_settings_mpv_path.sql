-- SPDX-License-Identifier: MPL-2.0

-- Removes the legacy external-MPV path. SQLite cannot drop a column that a CHECK
-- constraint references, so the table is rebuilt. No foreign key, index, view or
-- trigger references settings, so the rebuild is safe inside the migration
-- transaction with foreign_keys enabled.
CREATE TABLE settings_new (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    appearance INTEGER NOT NULL CHECK (appearance IN (0, 1, 2, 3)),
    autoplay_next INTEGER NOT NULL CHECK (autoplay_next IN (0, 1, 2)),
    resume_playback INTEGER NOT NULL CHECK (resume_playback IN (0, 1, 2)),
    language INTEGER NOT NULL CHECK (language IN (0, 1, 2))
);

INSERT INTO settings_new (id, appearance, autoplay_next, resume_playback, language)
SELECT id, appearance, autoplay_next, resume_playback, language FROM settings;

DROP TABLE settings;

ALTER TABLE settings_new RENAME TO settings;
