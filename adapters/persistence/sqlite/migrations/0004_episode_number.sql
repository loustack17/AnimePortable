-- SPDX-License-Identifier: MPL-2.0

-- Stores the source's episode number with each mapping, so Following can show the
-- latest episode from local data when the source is unreachable. Older rows keep an
-- empty number until their episode list is loaded again.
ALTER TABLE episode_mappings ADD COLUMN number TEXT NOT NULL DEFAULT '' CHECK (length(number) <= 128);
