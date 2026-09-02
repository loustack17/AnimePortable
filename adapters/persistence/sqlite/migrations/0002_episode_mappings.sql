-- SPDX-License-Identifier: MPL-2.0

CREATE TABLE episode_mappings (
    anime_id TEXT NOT NULL CHECK (length(anime_id) > 0 AND length(anime_id) <= 1024),
    episode_id TEXT NOT NULL CHECK (length(episode_id) > 0 AND length(episode_id) <= 1024),
    provider TEXT NOT NULL CHECK (length(provider) > 0 AND length(provider) <= 1024),
    provider_anime_id TEXT NOT NULL CHECK (length(provider_anime_id) > 0 AND length(provider_anime_id) <= 1024),
    provider_episode_id TEXT NOT NULL CHECK (length(provider_episode_id) > 0 AND length(provider_episode_id) <= 1024),
    PRIMARY KEY (anime_id, episode_id, provider, provider_anime_id, provider_episode_id),
    UNIQUE (provider, provider_anime_id, provider_episode_id),
    FOREIGN KEY (anime_id) REFERENCES anime(id) ON DELETE CASCADE,
    FOREIGN KEY (anime_id, provider, provider_anime_id) REFERENCES source_refs(anime_id, provider, external_id) ON DELETE CASCADE
);
