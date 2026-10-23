-- SPDX-License-Identifier: MPL-2.0

-- What the source catalog says about each anime: the year and season it began airing and
-- its episode count, used as metadata match hints (Loop 37d). Existing rows start unknown
-- and are filled the next time the anime is read from the catalog.
ALTER TABLE anime ADD COLUMN catalog_year INTEGER NOT NULL DEFAULT 0 CHECK (catalog_year >= 0 AND catalog_year <= 9999);
ALTER TABLE anime ADD COLUMN catalog_season TEXT NOT NULL DEFAULT '' CHECK (catalog_season IN ('', 'winter', 'spring', 'summer', 'fall'));
ALTER TABLE anime ADD COLUMN catalog_episode_count INTEGER NOT NULL DEFAULT 0 CHECK (catalog_episode_count >= 0 AND catalog_episode_count <= 100000);
