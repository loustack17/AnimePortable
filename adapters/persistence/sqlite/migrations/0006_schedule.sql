-- SPDX-License-Identifier: MPL-2.0

-- The weekly schedule is Bangumi's airing calendar, cached so the schedule page opens
-- without a network fetch. The whole table is replaced on each refresh. Entries may name
-- shows the library does not have, so there is no foreign key to anime; a local anime is
-- matched through metadata when the schedule is read.
CREATE TABLE schedule_entries (
    provider TEXT NOT NULL CHECK (length(provider) BETWEEN 1 AND 1024),
    external_id TEXT NOT NULL CHECK (length(external_id) BETWEEN 1 AND 1024),
    title TEXT NOT NULL CHECK (length(title) <= 65536),
    native_title TEXT NOT NULL CHECK (length(native_title) <= 65536),
    weekday INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY (provider, external_id, weekday)
);
