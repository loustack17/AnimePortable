-- SPDX-License-Identifier: MPL-2.0

-- Anime1 gives no episode dates, so Home's 近期更新 uses the time the app first saw an
-- episode, counted only after the anime was followed. Older mappings keep an empty
-- first-seen time and never count as recent. Anime followed before this migration get
-- the migration time, so episodes they already had are not reported as updates.
ALTER TABLE episode_mappings ADD COLUMN first_seen_at TEXT NOT NULL DEFAULT '' CHECK (first_seen_at = '' OR length(first_seen_at) = 30);
ALTER TABLE following ADD COLUMN followed_at TEXT NOT NULL DEFAULT '' CHECK (followed_at = '' OR length(followed_at) = 30);
UPDATE following SET followed_at = strftime('%Y-%m-%dT%H:%M:%f', 'now') || '000000Z';
