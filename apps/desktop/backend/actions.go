// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"time"

	"animeportable/adapters/player/mpv"
	"animeportable/core"
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
	return episodesDTO, nil
}

func (service *Service) Following(ctx context.Context) ([]Following, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	_, store, _, err := service.appSnapshot()
	if err != nil {
		return nil, err
	}
	ids, err := store.Following(operationCtx)
	if err != nil {
		return nil, safeError(err)
	}
	history, err := store.History(operationCtx)
	if err != nil {
		return nil, safeError(err)
	}
	watched := make(map[core.AnimeID]map[core.EpisodeID]struct{})
	latest := make(map[core.AnimeID]core.HistoryEntry)
	for _, entry := range history {
		if watched[entry.Progress.AnimeID] == nil {
			watched[entry.Progress.AnimeID] = make(map[core.EpisodeID]struct{})
		}
		watched[entry.Progress.AnimeID][entry.Progress.EpisodeID] = struct{}{}
		previous, exists := latest[entry.Progress.AnimeID]
		if !exists || entry.LastPlayedAt.After(previous.LastPlayedAt) {
			latest[entry.Progress.AnimeID] = entry
		}
	}
	result := make([]Following, 0, len(ids))
	for _, id := range ids {
		entry := Following{AnimeID: string(id)}
		if len(watched[id]) > 0 {
			entry.HasWatched = true
			entry.LatestWatched = string(latest[id].Progress.EpisodeID)
		}
		result = append(result, entry)
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

func (service *Service) Schedule(ctx context.Context, from string, to string) ([]ScheduleItem, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	start, err := parseTime(from)
	if err != nil {
		return nil, err
	}
	end, err := parseTime(to)
	if err != nil || !end.After(start) {
		return nil, ErrInvalidInput
	}
	_, store, source, err := service.appSnapshot()
	if err != nil {
		return nil, err
	}
	items, err := source.Schedule(operationCtx, core.ScheduleQuery{From: start, To: end})
	if err != nil {
		return nil, safeError(err)
	}
	for _, item := range items {
		if item.Anime.Ref.Provider == "" || item.Anime.Ref.ID == "" || (item.Episode.Ref.ID != "" && item.Episode.Ref.Anime != item.Anime.Ref) || item.AirsAt.IsZero() || item.AirsAt.Before(start) || !item.AirsAt.Before(end) || (item.Precision != core.SchedulePrecisionDay && item.Precision != core.SchedulePrecisionTime) {
			return nil, ErrInvalidInput
		}
	}
	animeInputs := make([]core.SourceAnime, 0, len(items))
	for _, item := range items {
		animeInputs = append(animeInputs, item.Anime)
	}
	animeItems, ingestErr := service.ingestAnime(operationCtx, store, animeInputs)
	if ingestErr != nil {
		return nil, safeError(ingestErr)
	}
	byRef := make(map[core.SourceRef]sourceIdentity, len(animeItems))
	for _, item := range animeItems {
		byRef[item.ref] = item
	}
	result := make([]ScheduleItem, 0, len(items))
	for _, item := range items {
		animeItem, ok := byRef[item.Anime.Ref]
		if !ok {
			return nil, ErrUnavailable
		}
		entry := ScheduleItem{AnimeID: string(animeItem.anime.ID), AirsAt: item.AirsAt.UTC().Format(time.RFC3339Nano), Precision: precisionName(item.Precision)}
		if item.Episode.Ref.ID != "" {
			episodes, episodeErr := service.ingestEpisodes(operationCtx, store, animeItem.anime.ID, animeItem.ref, []core.SourceEpisode{item.Episode})
			if episodeErr != nil || len(episodes) != 1 {
				return nil, safeError(episodeErr)
			}
			entry.EpisodeID = episodes[0].ID
		}
		result = append(result, entry)
	}
	return result, nil
}

func precisionName(value core.SchedulePrecision) string {
	if value == core.SchedulePrecisionTime {
		return "time"
	}
	return "day"
}

func (service *Service) History(ctx context.Context) ([]History, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	_, store, _, err := service.appSnapshot()
	if err != nil {
		return nil, err
	}
	entries, err := store.History(operationCtx)
	if err != nil {
		return nil, safeError(err)
	}
	result := make([]History, 0, len(entries))
	for _, entry := range entries {
		result = append(result, historyDTO(entry))
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
	return settingValues(value), nil
}

func (service *Service) SaveSettings(ctx context.Context, input Settings) error {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	appearance, err := parseAppearance(input.Appearance)
	if err != nil {
		return err
	}
	autoplay, err := parseToggle(input.AutoplayNext)
	if err != nil {
		return err
	}
	resume, err := parseToggle(input.ResumePlayback)
	if err != nil {
		return err
	}
	language, err := parseLanguage(input.Language)
	if err != nil {
		return err
	}
	if len(input.MPVPath) > 8192 {
		return ErrInvalidInput
	}
	_, store, _, err := service.appSnapshot()
	if err != nil {
		return err
	}
	return safeError(store.SaveSettings(operationCtx, core.Settings{Appearance: appearance, MPVPath: input.MPVPath, AutoplayNext: autoplay, ResumePlayback: resume, Language: language}))
}

func (service *Service) Play(ctx context.Context, input PlayRequest) error {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	service.playMu.Lock()
	defer service.playMu.Unlock()
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
		if err := app.SwitchEpisode(operationCtx, session, animeID, episodeID, ref, startAt); err == nil {
			return nil
		} else if !terminalPlaybackError(err) {
			return safeError(err)
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
	started, err := app.PlayEpisode(operationCtx, animeID, episodeID, ref, startAt)
	if err != nil {
		return safeError(err)
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

func (service *Service) ensurePlayer(ctx context.Context, store core.Store) (*core.App, error) {
	service.mu.Lock()
	if service.player != nil && service.app != nil {
		app := service.app
		service.mu.Unlock()
		return app, nil
	}
	service.mu.Unlock()
	settings, err := store.Settings(ctx)
	if err != nil {
		return nil, safeError(err)
	}
	player, err := service.newPlayer(settings.MPVPath)
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
	return errors.Is(err, mpv.ErrPlayerClosed) || errors.Is(err, mpv.ErrPlayerFailed) || errors.Is(err, mpv.ErrIPCClosed)
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
	result, err := loader.Load(operationCtx, metadataValue.CoverURL)
	if err != nil {
		return Cover{}, safeError(err)
	}
	return Cover{Bytes: append([]byte(nil), result.Bytes...), MediaType: result.MediaType, Width: result.Width, Height: result.Height}, nil
}
