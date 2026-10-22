-- SPDX-License-Identifier: MPL-2.0

-- Adds the playback speed (percent), skip-intro length (seconds), hardware decoding toggle
-- (0 unspecified, 1 disabled, 2 enabled), proxy mode (0 unspecified, 1 none, 2 system,
-- 3 manual) and manual proxy address. Existing rows keep every other value and read 0 or
-- empty, which the core resolves to its defaults.
ALTER TABLE settings ADD COLUMN playback_speed INTEGER NOT NULL DEFAULT 0 CHECK (playback_speed IN (0, 75, 100, 125, 150, 200));
ALTER TABLE settings ADD COLUMN skip_intro_seconds INTEGER NOT NULL DEFAULT 0 CHECK (skip_intro_seconds = 0 OR (skip_intro_seconds BETWEEN 30 AND 120 AND skip_intro_seconds % 5 = 0));
ALTER TABLE settings ADD COLUMN hardware_decoding INTEGER NOT NULL DEFAULT 0 CHECK (hardware_decoding IN (0, 1, 2));
ALTER TABLE settings ADD COLUMN proxy_mode INTEGER NOT NULL DEFAULT 0 CHECK (proxy_mode IN (0, 1, 2, 3));
ALTER TABLE settings ADD COLUMN proxy_address TEXT NOT NULL DEFAULT '' CHECK (length(CAST(proxy_address AS BLOB)) <= 255);
