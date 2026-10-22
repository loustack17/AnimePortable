-- SPDX-License-Identifier: MPL-2.0

-- Adds the auto-play toggle (0 unspecified, 1 disabled, 2 enabled). Existing rows keep
-- every other value and read 0, which the core resolves to its default.
ALTER TABLE settings ADD COLUMN auto_play INTEGER NOT NULL DEFAULT 0 CHECK (auto_play IN (0, 1, 2));
