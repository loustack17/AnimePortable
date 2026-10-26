// SPDX-License-Identifier: MPL-2.0

package fltkhome

import "animeportable/apps/desktop/backend"

const detailEpisodePageSize = 8

type detailModel struct {
	animeID  string
	episodes []backend.Episode
	history  map[string]backend.History
	selected int
	page     int
	// refreshed is set once Anime1's list arrived (or failed) for this anime.
	refreshed bool
}

func (model *detailModel) open(animeID string) {
	model.animeID = animeID
	model.episodes = nil
	model.history = nil
	model.selected = 0
	model.page = 0
	model.refreshed = false
}

// applyStored shows the stored list while Anime1's answer is still on its way. It reports
// whether the list changed; it never replaces a list Anime1 already gave.
func (model *detailModel) applyStored(episodes []backend.Episode) bool {
	if model.refreshed || len(episodes) == 0 {
		return false
	}
	model.setEpisodes(episodes)
	return true
}

// applyRefresh takes Anime1's answer. It reports whether the page has no list to offer: a
// failed refresh keeps a stored list shown, because its episodes still play.
func (model *detailModel) applyRefresh(episodes []backend.Episode, err error) (failed bool) {
	model.refreshed = true
	if err != nil {
		return len(model.episodes) == 0
	}
	model.setEpisodes(episodes)
	return false
}

func (model *detailModel) close() {
	model.open("")
}

func (model *detailModel) setEpisodes(episodes []backend.Episode) {
	selectedID := model.selectedEpisodeID()
	model.episodes = append(model.episodes[:0], episodes...)
	model.selected = 0
	if selectedID != "" {
		for index, episode := range model.episodes {
			if episode.ID == selectedID {
				model.selected = index
				break
			}
		}
	}
	model.page = model.selected / detailEpisodePageSize
}

func (model *detailModel) setHistory(history []backend.History) {
	model.history = make(map[string]backend.History)
	for _, item := range history {
		if item.AnimeID == model.animeID && item.EpisodeID != "" {
			model.history[item.EpisodeID] = item
		}
	}
}

func (model *detailModel) visibleEpisodes() []backend.Episode {
	start := model.page * detailEpisodePageSize
	if start >= len(model.episodes) {
		return nil
	}
	return model.episodes[start:min(len(model.episodes), start+detailEpisodePageSize)]
}

func (model *detailModel) selectedEpisodeID() string {
	if model.selected < 0 || model.selected >= len(model.episodes) {
		return ""
	}
	return model.episodes[model.selected].ID
}

func (model *detailModel) selectEpisode(index int) {
	if len(model.episodes) == 0 {
		model.selected = 0
		model.page = 0
		return
	}
	model.selected = max(0, min(len(model.episodes)-1, index))
	model.page = model.selected / detailEpisodePageSize
}

func (model *detailModel) moveSelection(delta int) {
	model.selectEpisode(model.selected + delta)
}

func (model *detailModel) movePage(delta int) {
	if len(model.episodes) == 0 || delta == 0 {
		return
	}
	model.page = movedPage(model.page, delta, len(model.episodes), detailEpisodePageSize)
	model.selected = model.page * detailEpisodePageSize
}

func (model *detailModel) pageCount() int {
	return pageCount(len(model.episodes), detailEpisodePageSize)
}

func (model *detailModel) playRequest() (backend.PlayRequest, bool) {
	if model.animeID == "" || model.selected < 0 || model.selected >= len(model.episodes) {
		return backend.PlayRequest{}, false
	}
	episode := model.episodes[model.selected]
	if episode.ID == "" || episode.AnimeID != "" && episode.AnimeID != model.animeID {
		return backend.PlayRequest{}, false
	}
	return backend.PlayRequest{AnimeID: model.animeID, EpisodeID: episode.ID}, true
}

func (model *detailModel) continueRequest() (backend.PlayRequest, bool) {
	var selected backend.History
	found := false
	for _, episode := range model.episodes {
		history, ok := model.history[episode.ID]
		if !ok || episode.ID == "" || episode.AnimeID != "" && episode.AnimeID != model.animeID || history.Completed || history.Position <= 0 {
			continue
		}
		if !found || historyOrder(history) > historyOrder(selected) {
			selected = history
			found = true
		}
	}
	if !found || model.animeID == "" {
		return backend.PlayRequest{}, false
	}
	return backend.PlayRequest{AnimeID: model.animeID, EpisodeID: selected.EpisodeID, StartAt: selected.Position}, true
}

func (model *detailModel) historyEntry(episode backend.Episode) (backend.History, bool) {
	history, ok := model.history[episode.ID]
	return history, ok && episode.ID != "" && history.AnimeID == model.animeID
}

func historyOrder(item backend.History) string {
	if item.UpdatedAt != "" {
		return item.UpdatedAt
	}
	return item.LastPlayed
}
