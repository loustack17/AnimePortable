CREATE TABLE anime (
    id TEXT PRIMARY KEY CHECK (length(id) > 0 AND length(id) <= 1024),
    title TEXT NOT NULL CHECK (length(title) <= 65536),
    native_title TEXT NOT NULL CHECK (length(native_title) <= 65536),
    description TEXT NOT NULL CHECK (length(description) <= 1048576)
);

CREATE TABLE source_refs (
    anime_id TEXT NOT NULL,
    provider TEXT NOT NULL CHECK (length(provider) > 0 AND length(provider) <= 1024),
    external_id TEXT NOT NULL CHECK (length(external_id) > 0 AND length(external_id) <= 1024),
    PRIMARY KEY (anime_id, provider, external_id),
    UNIQUE (provider, external_id),
    FOREIGN KEY (anime_id) REFERENCES anime(id) ON DELETE CASCADE
);

CREATE TABLE metadata (
    anime_id TEXT PRIMARY KEY,
    provider TEXT NOT NULL CHECK (length(provider) > 0 AND length(provider) <= 1024),
    external_id TEXT NOT NULL CHECK (length(external_id) > 0 AND length(external_id) <= 1024),
    title TEXT NOT NULL CHECK (length(title) <= 65536),
    native_title TEXT NOT NULL CHECK (length(native_title) <= 65536),
    description TEXT NOT NULL CHECK (length(description) <= 1048576),
    cover_url TEXT NOT NULL CHECK (length(cover_url) <= 8192),
    season TEXT NOT NULL CHECK (length(season) <= 128),
    year INTEGER NOT NULL CHECK (year >= 0 AND year <= 9999),
    studio TEXT NOT NULL CHECK (length(studio) <= 1024),
    episode_count INTEGER NOT NULL CHECK (episode_count >= 0),
    UNIQUE (provider, external_id),
    FOREIGN KEY (anime_id) REFERENCES anime(id) ON DELETE CASCADE
);

CREATE TABLE following (
    anime_id TEXT PRIMARY KEY,
    FOREIGN KEY (anime_id) REFERENCES anime(id) ON DELETE CASCADE
);

CREATE TABLE playback_progress (
    anime_id TEXT NOT NULL,
    episode_id TEXT NOT NULL CHECK (length(episode_id) > 0 AND length(episode_id) <= 1024),
    position_ns INTEGER NOT NULL CHECK (position_ns >= 0),
    duration_ns INTEGER NOT NULL CHECK (duration_ns >= 0),
    completed INTEGER NOT NULL CHECK (completed IN (0, 1)),
    updated_at TEXT NOT NULL CHECK (updated_at = '' OR length(updated_at) = 30),
    PRIMARY KEY (anime_id, episode_id),
    FOREIGN KEY (anime_id) REFERENCES anime(id) ON DELETE CASCADE
);

CREATE TABLE playback_history (
    anime_id TEXT NOT NULL,
    episode_id TEXT NOT NULL CHECK (length(episode_id) > 0 AND length(episode_id) <= 1024),
    position_ns INTEGER NOT NULL CHECK (position_ns >= 0),
    duration_ns INTEGER NOT NULL CHECK (duration_ns >= 0),
    completed INTEGER NOT NULL CHECK (completed IN (0, 1)),
    updated_at TEXT NOT NULL CHECK (updated_at = '' OR length(updated_at) = 30),
    last_played_at TEXT NOT NULL CHECK (last_played_at = '' OR length(last_played_at) = 30),
    PRIMARY KEY (anime_id, episode_id),
    FOREIGN KEY (anime_id) REFERENCES anime(id) ON DELETE CASCADE
);

CREATE TABLE settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    appearance INTEGER NOT NULL CHECK (appearance IN (0, 1, 2, 3)),
    mpv_path TEXT NOT NULL CHECK (length(mpv_path) <= 8192),
    autoplay_next INTEGER NOT NULL CHECK (autoplay_next IN (0, 1, 2)),
    resume_playback INTEGER NOT NULL CHECK (resume_playback IN (0, 1, 2)),
    language INTEGER NOT NULL CHECK (language IN (0, 1, 2))
);

CREATE INDEX idx_history_last_played ON playback_history (last_played_at DESC, anime_id, episode_id);
