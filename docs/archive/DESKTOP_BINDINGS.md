<!-- SPDX-License-Identifier: MPL-2.0 -->

# Desktop binding contract

Loop 21 exposes `backend.Service` through generated Wails TypeScript bindings. The visual shell remains unchanged; Loop 22 implements navigation.

## Actions

| Actions | Data access |
| --- | --- |
| Library, Detail, Following, History, Settings | Local SQLite only |
| Follow, Unfollow, RemoveHistory, SaveSettings | Local SQLite writes |
| Catalog, Search, Episodes, Schedule | Explicit source requests through the restricted backend client |
| GetCover | Looks up metadata by local anime ID, then loads a validated cover from an allowed origin |
| Play | Resolves persisted local anime/episode mappings and starts or switches the tracked MPV session |

Wails injects request contexts. Provider references, stream URLs, credentials, proxy addresses and IPC endpoints are not binding inputs or outputs. Lifecycle hooks are not frontend actions. The binding surface and DTO shapes are guarded by `apps/desktop/main_test.go`.

## Representation

- Anime and episode identities are opaque local strings. Source-anime ownership is stored atomically; repeated imports reuse the persisted local identity.
- Playback positions and durations, including `PlayRequest.startAt`, are milliseconds. Timestamps are UTC RFC3339 strings.
- `Cover.bytes` is a base64 string in generated TypeScript because Go serializes `[]byte` as base64. `mediaType`, `width` and `height` accompany validated JPEG/PNG content. No URL fetch parameter is accepted.
- Settings use explicit string values: appearance `unspecified/system/light/dark`, toggles `unspecified/enabled/disabled`, language `unspecified/zh-TW/en`. MPV path is a local executable preference, not a command line.
- Detail returns cached metadata when available. Metadata enrichment, automatic refresh, ordered episode caching and UI presentation remain in their planned later loops.
- Following is offline. Stored episode mappings prove previously discovered episodes, not current remote availability or chronological order. Latest-episode and new-episode information must not be inferred from random IDs.

## Ownership

The service owns its SQLite connection and restricted clients. MPV discovery and process creation are deferred until Play. The existing player owns proxy and IPC resources; the core owns tracked progress and same-session switching. Shutdown cancels and waits for requests, closes the tracked session while SQLite is still writable, then closes remaining dependencies and SQLite.

Successful playback outlives the request that started it. Frontend request cancellation is not a stop-player action.
