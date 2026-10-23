// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"animeportable/adapters/player/libmpv"
	"animeportable/core"
	metadata "animeportable/internal/metadata"
)

type sourceIdentity struct {
	anime core.Anime
	ref   core.SourceRef
}

func (service *Service) Catalog(ctx context.Context) ([]Anime, error) {
	return service.remoteAnime(ctx, func(operationCtx context.Context, source core.AnimeSource) ([]core.SourceAnime, error) {
		return source.Catalog(operationCtx)
	})
}

func (service *Service) Search(ctx context.Context, query string) ([]Anime, error) {
	return service.remoteAnime(ctx, func(operationCtx context.Context, source core.AnimeSource) ([]core.SourceAnime, error) {
		return source.Search(operationCtx, query)
	})
}

func (service *Service) remoteAnime(ctx context.Context, fetch func(context.Context, core.AnimeSource) ([]core.SourceAnime, error)) ([]Anime, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	_, store, source, err := service.appSnapshot()
	if err != nil {
		return nil, err
	}
	items, err := fetch(operationCtx, source)
	if err != nil {
		return nil, safeError(err)
	}
	identities, err := service.ingestAnime(operationCtx, store, items)
	if err != nil {
		return nil, safeError(err)
	}
	result := make([]Anime, 0, len(identities))
	for _, identity := range identities {
		item, dtoErr := animeDTO(identity.anime)
		if dtoErr != nil {
			return nil, dtoErr
		}
		result = append(result, item)
	}
	sortAnime(result)
	return result, nil
}

func (service *Service) Library(ctx context.Context) ([]Anime, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	_, store, _, err := service.appSnapshot()
	if err != nil {
		return nil, err
	}
	items, err := store.ListAnime(operationCtx)
	if err != nil {
		return nil, safeError(err)
	}
	result := make([]Anime, 0, len(items))
	for _, item := range items {
		dto, dtoErr := animeDTO(item)
		if dtoErr != nil {
			return nil, dtoErr
		}
		result = append(result, dto)
	}
	sortAnime(result)
	return result, nil
}

func (service *Service) Detail(ctx context.Context, animeID string) (Detail, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return Detail{}, err
	}
	defer done()
	localAnimeID, err := localID(animeID)
	if err != nil {
		return Detail{}, err
	}
	_, store, _, err := service.appSnapshot()
	if err != nil {
		return Detail{}, err
	}
	anime, err := store.Anime(operationCtx, localAnimeID)
	if err != nil {
		return Detail{}, safeError(err)
	}
	animeValue, dtoErr := animeDTO(anime)
	if dtoErr != nil {
		return Detail{}, dtoErr
	}
	result := Detail{Anime: animeValue}
	following, err := store.Following(operationCtx)
	if err != nil {
		return Detail{}, safeError(err)
	}
	result.Following = slices.ContainsFunc(following, func(item core.FollowedAnime) bool { return item.AnimeID == localAnimeID })
	metadataValue, err := store.Metadata(operationCtx, localAnimeID)
	if errors.Is(err, core.ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return Detail{}, safeError(err)
	}
	result.Metadata = &Metadata{Title: metadataValue.Title, NativeTitle: metadataValue.NativeTitle, Description: metadataValue.Description, Season: metadataValue.Season, Year: metadataValue.Year, Studio: metadataValue.Studio, EpisodeCount: metadataValue.EpisodeCount}
	return result, nil
}

// metadataLookupTimeout bounds a background lookup so a slow provider cannot hold an operation slot.
const metadataLookupTimeout = core.MetadataRefreshTimeout

// RefreshMetadata looks up and stores cover and synopsis metadata for an anime, replacing
// any stored record. It reports whether metadata was stored; false means no provider had a
// confident match.
func (service *Service) RefreshMetadata(ctx context.Context, animeID string) (bool, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return false, err
	}
	defer done()
	localAnimeID, err := localID(animeID)
	if err != nil {
		return false, err
	}
	app, store, source, err := service.appSnapshot()
	if err != nil {
		return false, err
	}
	service.mu.Lock()
	providers := service.metadata
	service.mu.Unlock()
	lookupCtx, cancel := context.WithTimeout(operationCtx, metadataLookupTimeout)
	defer cancel()
	service.fillCatalogInfo(lookupCtx, store, source, localAnimeID)
	found, err := app.RefreshMetadata(lookupCtx, localAnimeID, providers)
	return found, safeError(err)
}

// fillCatalogInfo reads the catalog row of an anime stored before catalog info was kept
// (migration 0011), so its year and episode count help the lookup. It is best effort: without
// the row the lookup runs on the title alone, as before. A row whose values are all unclear
// ("1-12+OVA", "2023/2024") stays unknown, so its catalog is read again at its next lookup;
// that is at most once per anime and app run (the detail page looks an anime up once).
func (service *Service) fillCatalogInfo(ctx context.Context, store core.Store, source core.AnimeSource, animeID core.AnimeID) {
	anime, err := store.Anime(ctx, animeID)
	if err != nil || anime.Catalog != (core.CatalogInfo{}) {
		return
	}
	refs, err := store.SourceRefs(ctx, animeID)
	if err != nil || len(refs) == 0 {
		return
	}
	catalog, err := source.Catalog(ctx)
	if err != nil {
		return
	}
	for _, item := range catalog {
		if slices.Contains(refs, item.Ref) {
			_, _ = service.ingestAnime(ctx, store, []core.SourceAnime{item})
			return
		}
	}
}

// StoredEpisodes lists the episodes stored the last time the anime's list was loaded, from local
// data only, so a screen can show them at once and refresh them with Episodes behind them. The
// stored list has no episode titles; an anime never loaded has none.
func (service *Service) StoredEpisodes(ctx context.Context, animeID string) ([]Episode, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	localAnimeID, err := localID(animeID)
	if err != nil {
		return nil, err
	}
	_, store, _, err := service.appSnapshot()
	if err != nil {
		return nil, err
	}
	mappings, err := store.EpisodeMappings(operationCtx, localAnimeID)
	if err != nil {
		return nil, safeError(err)
	}
	episodes := make([]Episode, 0, len(mappings))
	for _, mapping := range mappings {
		number, ok := metadata.NormalizePlainText(mapping.Number, metadata.TitleLimits())
		// A mapping stored before episode numbers were kept has none; the refresh fills it.
		if !ok || number == "" {
			continue
		}
		episodes = append(episodes, Episode{ID: string(mapping.EpisodeID), AnimeID: string(localAnimeID), Number: number})
	}
	orderEpisodes(episodes)
	return episodes, nil
}

func (service *Service) Episodes(ctx context.Context, animeID string) ([]Episode, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	localAnimeID, err := localID(animeID)
	if err != nil {
		return nil, err
	}
	_, store, source, err := service.appSnapshot()
	if err != nil {
		return nil, err
	}
	refs, err := store.SourceRefs(operationCtx, localAnimeID)
	if err != nil {
		return nil, safeError(err)
	}
	if len(refs) == 0 {
		return nil, ErrUnavailable
	}
	var episodes []core.SourceEpisode
	var parent core.SourceRef
	for _, ref := range refs {

		episodes, err = source.Episodes(operationCtx, ref)
		if err == nil {
			parent = ref
			break
		}
		if contextErr := operationCtx.Err(); contextErr != nil {
			return nil, safeError(contextErr)
		}
	}
	if err != nil {
		return nil, safeError(err)
	}
	episodesDTO, err := service.ingestEpisodes(operationCtx, store, localAnimeID, parent, episodes)
	if err != nil {
		return nil, safeError(err)
	}
	orderEpisodes(episodesDTO)
	return episodesDTO, nil
}

// orderEpisodes sorts episodes by number, ascending. Anime1 lists some series newest first.
// Episodes without a plain number (specials) keep their relative order after the numbered ones.
func orderEpisodes(episodes []Episode) {
	sort.SliceStable(episodes, func(left, right int) bool {
		leftNumber, leftOK := core.EpisodeNumberValue(episodes[left].Number)
		rightNumber, rightOK := core.EpisodeNumberValue(episodes[right].Number)
		if leftOK != rightOK {
			return leftOK
		}
		return leftOK && leftNumber < rightNumber
	})
}

// Following lists followed anime from local data only; it never waits on the source.
func (service *Service) Following(ctx context.Context) ([]Following, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	app, _, _, err := service.appSnapshot()
	if err != nil {
		return nil, err
	}
	entries, err := app.ListFollowing(operationCtx)
	if err != nil {
		return nil, safeError(err)
	}
	result := make([]Following, 0, len(entries))
	for _, entry := range entries {
		result = append(result, Following{
			AnimeID:               string(entry.AnimeID),
			LatestAvailable:       string(entry.LatestAvailable),
			LatestAvailableNumber: entry.LatestAvailableNumber,
			LatestWatched:         string(entry.LatestWatched),
			LatestWatchedNumber:   entry.LatestWatchedNumber,
			HasAvailable:          entry.HasAvailable,
			HasWatched:            entry.HasWatched,
			NewEpisode:            entry.NewEpisode,
			RecentlyUpdated:       entry.RecentlyUpdated,
			LatestSeenAt:          formatOptionalTime(entry.LatestSeenAt),
		})
	}
	return result, nil
}

func (service *Service) Follow(ctx context.Context, animeID string) error {
	return service.setFollowing(ctx, animeID, true)
}

func (service *Service) Unfollow(ctx context.Context, animeID string) error {
	return service.setFollowing(ctx, animeID, false)
}

func (service *Service) setFollowing(ctx context.Context, animeID string, following bool) error {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	localAnimeID, err := localID(animeID)
	if err != nil {
		return err
	}
	_, store, _, err := service.appSnapshot()
	if err != nil {
		return err
	}
	if err := store.SetFollowing(operationCtx, localAnimeID, following); err != nil {
		return safeError(err)
	}
	return nil
}

// scheduleLookupTimeout bounds a schedule fetch so a slow provider cannot hold an operation slot.
const scheduleLookupTimeout = core.ScheduleStepTimeout

// Schedule fetches the weekly airing schedule, replaces the stored one with it, adds the
// shows the source catalog has under the exact title to the local library, and returns it
// matched to local anime.
func (service *Service) Schedule(ctx context.Context) ([]ScheduleItem, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	_, store, source, err := service.appSnapshot()
	if err != nil {
		return nil, err
	}
	service.mu.Lock()
	provider := service.schedule
	service.mu.Unlock()
	if provider == nil {
		return nil, ErrUnavailable
	}
	lookupCtx, cancel := context.WithTimeout(operationCtx, scheduleLookupTimeout)
	entries, err := provider.Schedule(lookupCtx)
	cancel()
	if err != nil {
		return nil, safeError(err)
	}
	if len(entries) > core.MaxScheduleEntries {
		return nil, ErrInvalidInput
	}
	if err := store.ReplaceSchedule(operationCtx, entries); err != nil {
		return nil, safeError(err)
	}
	service.addScheduledCatalogAnime(operationCtx, store, source, entries)
	return storedSchedule(operationCtx, store)
}

// addScheduledCatalogAnime stores the catalog anime whose title matches a schedule entry, so
// such a show opens even if the user never searched for it. It is best effort: the schedule
// is already stored, so a catalog that cannot be fetched, or an item that cannot be stored,
// only leaves those shows unlinked.
func (service *Service) addScheduledCatalogAnime(ctx context.Context, store core.Store, source core.AnimeSource, entries []core.ScheduleEntry) {
	catalogCtx, cancel := context.WithTimeout(ctx, scheduleLookupTimeout)
	catalog, err := source.Catalog(catalogCtx)
	cancel()
	if err != nil {
		return
	}
	candidates := make([]core.Anime, 0, len(catalog))
	for index, item := range catalog {
		candidates = append(candidates, core.Anime{ID: core.AnimeID(strconv.Itoa(index)), Title: item.Title, NativeTitle: item.NativeTitle})
	}
	unmatched := make([]core.ScheduleEntry, len(entries))
	for index, entry := range entries {
		// Only candidate IDs, which are catalog positions, may come back.
		entry.AnimeID = ""
		unmatched[index] = entry
	}
	for _, entry := range core.MatchSchedule(unmatched, candidates) {
		index, err := strconv.Atoi(string(entry.AnimeID))
		if err != nil || index < 0 || index >= len(catalog) {
			continue
		}
		// One item that cannot be stored is skipped; a cancelled refresh stops.
		if _, err := service.ingestAnime(ctx, store, []core.SourceAnime{catalog[index]}); err != nil && ctx.Err() != nil {
			return
		}
	}
}

// CachedSchedule lists the stored weekly schedule from local data only; it never waits on
// the provider.
func (service *Service) CachedSchedule(ctx context.Context) ([]ScheduleItem, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	_, store, _, err := service.appSnapshot()
	if err != nil {
		return nil, err
	}
	return storedSchedule(operationCtx, store)
}

// storedSchedule matches the stored schedule to local anime. A matched entry shows the
// local title, the one the rest of the app uses.
func storedSchedule(ctx context.Context, store core.Store) ([]ScheduleItem, error) {
	entries, err := store.Schedule(ctx)
	if err != nil {
		return nil, safeError(err)
	}
	library, err := store.ListAnime(ctx)
	if err != nil {
		return nil, safeError(err)
	}
	titles := make(map[core.AnimeID]string, len(library))
	for _, anime := range library {
		titles[anime.ID] = anime.Title
	}
	result := make([]ScheduleItem, 0, len(entries))
	for _, entry := range core.MatchSchedule(entries, library) {
		item := ScheduleItem{AnimeID: string(entry.AnimeID), Title: entry.Title, Query: core.ScheduleSearchQuery(entry.Title), Weekday: int(entry.Weekday)}
		if title := strings.TrimSpace(titles[entry.AnimeID]); entry.AnimeID != "" && title != "" {
			item.Title = title
		}
		result = append(result, item)
	}
	return result, nil
}

func (service *Service) History(ctx context.Context) ([]History, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	app, _, _, err := service.appSnapshot()
	if err != nil {
		return nil, err
	}
	entries, err := app.ListHistory(operationCtx)
	if err != nil {
		return nil, safeError(err)
	}
	result := make([]History, 0, len(entries))
	for _, entry := range entries {
		item := historyDTO(entry.HistoryEntry)
		item.EpisodeNumber = entry.EpisodeNumber
		result = append(result, item)
	}
	return result, nil
}

func (service *Service) RemoveHistory(ctx context.Context, animeID string) error {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	id, err := localID(animeID)
	if err != nil {
		return err
	}
	_, store, _, err := service.appSnapshot()
	if err != nil {
		return err
	}
	return safeError(store.RemoveHistory(operationCtx, id))
}

func (service *Service) Settings(ctx context.Context) (Settings, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return Settings{}, err
	}
	defer done()
	_, store, _, err := service.appSnapshot()
	if err != nil {
		return Settings{}, err
	}
	value, err := store.Settings(operationCtx)
	if err != nil {
		return Settings{}, safeError(err)
	}
	// Resolved so no caller sees "unspecified" or needs its own defaults.
	return settingValues(value.Resolved()), nil
}

// SaveSettings stores the settings page's values. The speed is not one of them: it is kept as
// stored so a page that loaded earlier cannot overwrite a speed chosen in the player since.
func (service *Service) SaveSettings(ctx context.Context, input Settings) error {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	settings, err := settingsFromInput(input)
	if err != nil {
		return err
	}
	_, store, _, err := service.appSnapshot()
	if err != nil {
		return err
	}
	service.settingsMu.Lock()
	defer service.settingsMu.Unlock()
	stored, err := store.Settings(operationCtx)
	if err != nil {
		return safeError(err)
	}
	settings.PlaybackSpeed = stored.PlaybackSpeed
	if err := store.SaveSettings(operationCtx, settings); err != nil {
		return safeError(err)
	}
	// New connections use the saved proxy; clients close their idle ones when it changes.
	resolved := settings.Resolved()
	service.proxy.Set(resolved.ProxyMode, resolved.ProxyAddress)
	return nil
}

// SavePlaybackSpeed remembers the speed chosen in the player, in percent, and leaves every
// other stored setting as it is.
func (service *Service) SavePlaybackSpeed(ctx context.Context, percent int) error {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	speed := core.PlaybackSpeed(percent)
	if speed == core.PlaybackSpeedUnspecified || !speed.Valid() {
		return ErrInvalidInput
	}
	_, store, _, err := service.appSnapshot()
	if err != nil {
		return err
	}
	service.settingsMu.Lock()
	defer service.settingsMu.Unlock()
	stored, err := store.Settings(operationCtx)
	if err != nil {
		return safeError(err)
	}
	stored.PlaybackSpeed = speed
	return safeError(store.SaveSettings(operationCtx, stored))
}

// playError carries the unsanitized cause of a playback failure to Play, which logs the
// fixed category and then returns the sanitized error to the caller.
type playError struct{ cause error }

func (e *playError) Error() string { return e.cause.Error() }
func (e *playError) Unwrap() error { return e.cause }

func (service *Service) Play(ctx context.Context, input PlayRequest) error {
	failures := service.failures
	err := service.play(ctx, input)
	var failed *playError
	if errors.As(err, &failed) {
		err = failed.cause
		failures.recordPlayFailure(err)
		return safeError(err)
	}
	failures.recordPlayFailure(err)
	return err
}

func (service *Service) play(ctx context.Context, input PlayRequest) error {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	service.playMu.Lock()
	defer service.playMu.Unlock()
	// After the lock: a rejection from the previous session must not count for this attempt.
	service.failures.beginAttempt()
	animeID, err := localID(input.AnimeID)
	if err != nil {
		return err
	}
	episodeID, err := episodeID(input.EpisodeID)
	if err != nil {
		return err
	}
	if !validMillis(input.StartAt) {
		return ErrInvalidInput
	}
	app, store, _, err := service.appSnapshot()
	if err != nil {
		return err
	}
	mappings, err := store.EpisodeMappings(operationCtx, animeID)
	if err != nil {
		return safeError(err)
	}
	var ref core.EpisodeRef
	for _, mapping := range mappings {
		if mapping.EpisodeID == episodeID {
			ref = mapping.Ref
			break
		}
	}
	if ref.ID == "" || ref.Anime.ID == "" {
		return core.ErrNotFound
	}
	app, err = service.ensurePlayer(operationCtx, store)
	if err != nil {
		return err
	}
	service.mu.Lock()
	session := service.session
	service.mu.Unlock()
	startAt := time.Duration(input.StartAt) * time.Millisecond
	if session != nil {
		if err := app.SwitchEpisode(operationCtx, session, animeID, episodeID, ref, startAt, input.Paused); err == nil {
			return nil
		} else if !terminalPlaybackError(err) {
			return &playError{cause: err}
		} else if closeErr := session.Close(); closeErr != nil {
			return safeError(closeErr)
		}
		service.mu.Lock()
		if service.session == session {
			service.session = nil
			service.player = nil
			service.app = core.NewApp(service.source, nil, service.store)
		}
		service.mu.Unlock()
		app, err = service.ensurePlayer(operationCtx, store)
		if err != nil {
			return err
		}
	}
	started, err := app.PlayEpisode(operationCtx, animeID, episodeID, ref, startAt, input.Paused)
	if err != nil {
		return &playError{cause: err}
	}
	service.mu.Lock()
	old := service.session
	service.session = started
	service.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

func (service *Service) StopPlayback(ctx context.Context) error {
	_, done, err := service.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	service.playMu.Lock()
	defer service.playMu.Unlock()
	service.mu.Lock()
	session := service.session
	service.session = nil
	service.mu.Unlock()
	if session == nil {
		return nil
	}
	return safeError(session.Close())
}

func (service *Service) ensurePlayer(ctx context.Context, store core.Store) (*core.App, error) {
	service.mu.Lock()
	if service.player != nil && service.app != nil {
		app := service.app
		service.mu.Unlock()
		return app, nil
	}
	service.mu.Unlock()
	player, err := service.newPlayer()
	if err != nil {
		return nil, safeError(err)
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed {
		return nil, ErrClosed
	}
	if service.player == nil {
		service.player = player
		service.app = core.NewApp(service.source, player, service.store)
	}
	return service.app, nil
}

func terminalPlaybackError(err error) bool {
	return errors.Is(err, libmpv.ErrPlayerClosed) || errors.Is(err, libmpv.ErrPlayerFailed)
}

func (service *Service) GetCover(ctx context.Context, animeID string) (Cover, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return Cover{}, err
	}
	defer done()
	id, err := localID(animeID)
	if err != nil {
		return Cover{}, err
	}
	service.mu.Lock()
	loader := service.cover
	store := service.store
	service.mu.Unlock()
	if loader == nil || store == nil {
		return Cover{}, ErrUnavailable
	}
	metadataValue, err := store.Metadata(operationCtx, id)
	if err != nil {
		return Cover{}, safeError(err)
	}
	if metadataValue.CoverURL == "" {
		return Cover{}, core.ErrNotFound
	}
	result, err := loader.Load(operationCtx, metadataValue.CoverURL)
	if err != nil {
		return Cover{}, safeError(err)
	}
	return Cover{Bytes: append([]byte(nil), result.Bytes...), MediaType: result.MediaType, Width: result.Width, Height: result.Height}, nil
}
