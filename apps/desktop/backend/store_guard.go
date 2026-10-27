// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"fmt"
	"sync/atomic"

	"animeportable/core"
)

// ErrRestartRequired is returned for a write after the data folder was moved: the open
// database is the old copy, so anything written to it would be lost at the next start.
var ErrRestartRequired = fmt.Errorf("data folder moved, restart required: %w", ErrUnavailable)

// writeGuard passes everything to the store until refuse is set, then rejects every write.
// Reads (through the embedded store), the snapshot and Close keep working so the app can still shut down cleanly.
type writeGuard struct {
	core.Store
	refuse *atomic.Bool
}

func (g writeGuard) check() error {
	if g.refuse.Load() {
		return ErrRestartRequired
	}
	return nil
}

func (g writeGuard) SaveAnime(ctx context.Context, anime core.Anime) error {
	if err := g.check(); err != nil {
		return err
	}
	return g.Store.SaveAnime(ctx, anime)
}

func (g writeGuard) SaveSourceRef(ctx context.Context, animeID core.AnimeID, ref core.SourceRef) error {
	if err := g.check(); err != nil {
		return err
	}
	return g.Store.SaveSourceRef(ctx, animeID, ref)
}

func (g writeGuard) SaveEpisodeMapping(ctx context.Context, mapping core.EpisodeMapping) error {
	if err := g.check(); err != nil {
		return err
	}
	return g.Store.SaveEpisodeMapping(ctx, mapping)
}

func (g writeGuard) SaveMetadata(ctx context.Context, animeID core.AnimeID, metadata core.AnimeMetadata) error {
	if err := g.check(); err != nil {
		return err
	}
	return g.Store.SaveMetadata(ctx, animeID, metadata)
}

func (g writeGuard) SetFollowing(ctx context.Context, animeID core.AnimeID, following bool) error {
	if err := g.check(); err != nil {
		return err
	}
	return g.Store.SetFollowing(ctx, animeID, following)
}

func (g writeGuard) RemoveHistory(ctx context.Context, animeID core.AnimeID) error {
	if err := g.check(); err != nil {
		return err
	}
	return g.Store.RemoveHistory(ctx, animeID)
}

func (g writeGuard) SavePlaybackCheckpoint(ctx context.Context, entry core.HistoryEntry) error {
	if err := g.check(); err != nil {
		return err
	}
	return g.Store.SavePlaybackCheckpoint(ctx, entry)
}

func (g writeGuard) SaveSettings(ctx context.Context, settings core.Settings) error {
	if err := g.check(); err != nil {
		return err
	}
	return g.Store.SaveSettings(ctx, settings)
}

func (g writeGuard) ReplaceSchedule(ctx context.Context, entries []core.ScheduleEntry) error {
	if err := g.check(); err != nil {
		return err
	}
	return g.Store.ReplaceSchedule(ctx, entries)
}

// IngestSourceAnime is the store's atomic write for source search results; see ingestAnime.
func (g writeGuard) IngestSourceAnime(ctx context.Context, candidate core.Anime, ref core.SourceRef) (core.Anime, error) {
	if err := g.check(); err != nil {
		return core.Anime{}, err
	}
	ingester, ok := g.Store.(sourceIngester)
	if !ok {
		return core.Anime{}, ErrUnavailable
	}
	return ingester.IngestSourceAnime(ctx, candidate, ref)
}

func (g writeGuard) SnapshotTo(ctx context.Context, target string) error {
	snapshotter, ok := g.Store.(DatabaseSnapshotter)
	if !ok {
		return ErrUnavailable
	}
	return snapshotter.SnapshotTo(ctx, target)
}

func (g writeGuard) Close() error {
	if closer, ok := g.Store.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
