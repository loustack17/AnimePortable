// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/adapters/persistence/sqlite"
	"animeportable/adapters/player/mpv"
	"animeportable/adapters/source/anime1"
	"animeportable/core"
)

//go:embed fixture.json
var fixtureJSON []byte

type fixtureSpec struct {
	Version int `json:"version"`
	Anime   struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		NativeTitle string `json:"nativeTitle"`
		Provider    string `json:"provider"`
		SourceID    string `json:"sourceId"`
	} `json:"anime"`
	Episode struct {
		ID                string `json:"id"`
		ProviderEpisodeID string `json:"providerEpisodeId"`
	} `json:"episode"`
	History struct {
		PositionMS int64  `json:"positionMs"`
		DurationMS int64  `json:"durationMs"`
		UpdatedAt  string `json:"updatedAt"`
	} `json:"history"`
}
type prepareOptions struct{ CategoryID, PostID, MPVPath string }

func fixtureDigest() string {
	sum := sha256.Sum256(fixtureJSON)
	return hex.EncodeToString(sum[:])
}

func decodeFixture(data []byte) (fixtureSpec, error) {
	var spec fixtureSpec
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return spec, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return spec, errors.New("trailing fixture data")
	}
	if spec.Version != 1 || spec.Anime.ID == "" || spec.Anime.Title == "" || spec.Anime.Provider != "synthetic" || spec.Anime.SourceID == "" || spec.Episode.ID == "" || spec.Episode.ProviderEpisodeID == "" {
		return spec, errors.New("invalid fixture identity or version")
	}
	if spec.History.PositionMS <= 0 || spec.History.DurationMS <= spec.History.PositionMS || spec.History.DurationMS > int64((24*time.Hour)/time.Millisecond) {
		return spec, errors.New("invalid fixture playback duration")
	}
	if _, err := time.Parse(time.RFC3339Nano, spec.History.UpdatedAt); err != nil {
		return spec, errors.New("invalid fixture timestamp")
	}
	return spec, nil
}
func loadFixture() (fixtureSpec, error) { return decodeFixture(fixtureJSON) }

func positiveDecimal(value string, limit int) bool {
	if len(value) == 0 || len(value) > limit || value[0] == '0' {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func preflightEpisode(ctx context.Context, category, post string) error {
	if !positiveDecimal(category, 10) || !positiveDecimal(post, 20) {
		return errors.New("invalid source identity")
	}
	client, err := securehttp.New(securehttp.Config{AllowedOrigins: anime1.AllowedOrigins()})
	if err != nil {
		return errors.New("UPSTREAM_UNAVAILABLE: source client could not initialize")
	}
	defer client.CloseIdleConnections()
	episodes, err := anime1.New(client).Episodes(ctx, core.SourceRef{Provider: "anime1", ID: category})
	if err != nil {
		return errors.New("UPSTREAM_UNAVAILABLE: episode membership could not be verified; playback remains BLOCKED")
	}
	for _, episode := range episodes {
		if episode.Ref.Anime.Provider == "anime1" && episode.Ref.Anime.ID == category && episode.Ref.ID == post {
			return nil
		}
	}
	return errors.New("SOURCE_IDENTITY_MISMATCH: post was not found in the category; no profile prepared")
}

func prepare(p profilePaths, input prepareOptions, preflight func(context.Context, string, string) error) (result error) {
	guard, err := acquire(p)
	if err != nil {
		return err
	}
	retain := false
	defer func() {
		if retain {
			_ = guard.file.Close()
		} else {
			result = errors.Join(result, guard.release())
		}
	}()
	if _, err := os.Lstat(p.root); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return err
		}
		return errors.New("profile already exists; prepare never overwrites; close app and reset first")
	}
	spec, err := loadFixture()
	if err != nil {
		return err
	}
	mode := "synthetic"
	if input.CategoryID != "" || input.PostID != "" {
		if !positiveDecimal(input.CategoryID, 10) || !positiveDecimal(input.PostID, 20) {
			return errors.New("prepare requires a complete positive numeric category/post pair (10/20 digits maximum)")
		}
		if preflight == nil {
			return errors.New("live preparation requires membership preflight")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		err = preflight(ctx, input.CategoryID, input.PostID)
		cancel()
		if err != nil {
			return err
		}
		mode = "real"
	}
	if input.MPVPath != "" {
		if !filepath.IsAbs(input.MPVPath) {
			return errors.New("--mpv-path must be absolute")
		}
		if _, err := mpv.Find(input.MPVPath); err != nil {
			return errors.New("configured MPV executable is invalid")
		}
	}
	retain = true
	if err := makeDirectories(filepath.Dir(p.database)); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := seedStore(ctx, p.database, spec, input); err != nil {
		return fmt.Errorf("fixture preparation incomplete; guard retained: %w", err)
	}
	entries, err := inventory(p, false)
	if err != nil {
		return err
	}
	owner := ownership{Version: 1, Root: p.root, Mode: mode, FixtureSHA256: fixtureDigest(), Entries: entries}
	if err := writeMarker(p, owner, true); err != nil {
		return err
	}
	if _, err := validateOwned(p); err != nil {
		return err
	}
	retain = false
	fmt.Printf("Prepared %s fixture: %s\nContinue Watching / Following: %s\nSaved position: %d ms\n", mode, p.database, spec.Anime.Title, spec.History.PositionMS)
	if mode == "real" {
		fmt.Println("Episode membership verified; real Home-to-MPV playback is still HUMAN_PENDING.")
	} else {
		fmt.Println("Synthetic fixture: populated/offline state only; it cannot prove real playback.")
	}
	return nil
}

func expectedState(spec fixtureSpec, input prepareOptions) (core.Anime, core.SourceRef, core.EpisodeMapping, core.HistoryEntry, core.Settings, error) {
	when, err := time.Parse(time.RFC3339Nano, spec.History.UpdatedAt)
	anime := core.Anime{ID: core.AnimeID(spec.Anime.ID), Title: spec.Anime.Title, NativeTitle: spec.Anime.NativeTitle}
	ref := core.SourceRef{Provider: spec.Anime.Provider, ID: spec.Anime.SourceID}
	providerEpisode := spec.Episode.ProviderEpisodeID
	if input.CategoryID != "" {
		ref = core.SourceRef{Provider: "anime1", ID: input.CategoryID}
		providerEpisode = input.PostID
	}
	mapping := core.EpisodeMapping{AnimeID: anime.ID, EpisodeID: core.EpisodeID(spec.Episode.ID), Ref: core.EpisodeRef{Anime: ref, ID: providerEpisode}}
	history := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: anime.ID, EpisodeID: mapping.EpisodeID, Position: time.Duration(spec.History.PositionMS) * time.Millisecond, Duration: time.Duration(spec.History.DurationMS) * time.Millisecond, UpdatedAt: when}, LastPlayedAt: when}
	settings := core.Settings{Appearance: core.AppearanceDark, MPVPath: input.MPVPath, AutoplayNext: core.ToggleDisabled, ResumePlayback: core.ToggleEnabled, Language: core.LanguageTraditionalChinese}
	return anime, ref, mapping, history, settings, err
}

func seedStore(ctx context.Context, path string, spec fixtureSpec, input prepareOptions) error {
	anime, ref, mapping, history, settings, err := expectedState(spec, input)
	if err != nil {
		return err
	}
	store, err := sqlite.Open(ctx, path)
	if err != nil {
		return err
	}
	writeErr := func() error {
		if err := store.SaveAnime(ctx, anime); err != nil {
			return err
		}
		if err := store.SaveSourceRef(ctx, anime.ID, ref); err != nil {
			return err
		}
		if err := store.SaveEpisodeMapping(ctx, mapping); err != nil {
			return err
		}
		if err := store.SetFollowing(ctx, anime.ID, true); err != nil {
			return err
		}
		if err := store.SavePlaybackCheckpoint(ctx, history); err != nil {
			return err
		}
		return store.SaveSettings(ctx, settings)
	}()
	if err := errors.Join(writeErr, store.Close()); err != nil {
		return err
	}
	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		return err
	}
	verifyErr := func() error {
		actualAnime, err := reopened.ListAnime(ctx)
		if err != nil {
			return err
		}
		refs, err := reopened.SourceRefs(ctx, anime.ID)
		if err != nil {
			return err
		}
		mappings, err := reopened.EpisodeMappings(ctx, anime.ID)
		if err != nil {
			return err
		}
		follows, err := reopened.Following(ctx)
		if err != nil {
			return err
		}
		histories, err := reopened.History(ctx)
		if err != nil {
			return err
		}
		progress, err := reopened.Progress(ctx, anime.ID, mapping.EpisodeID)
		if err != nil {
			return err
		}
		savedSettings, err := reopened.Settings(ctx)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(actualAnime, []core.Anime{anime}) || !reflect.DeepEqual(refs, []core.SourceRef{ref}) || !reflect.DeepEqual(mappings, []core.EpisodeMapping{mapping}) || !reflect.DeepEqual(follows, []core.AnimeID{anime.ID}) || !reflect.DeepEqual(histories, []core.HistoryEntry{history}) || !reflect.DeepEqual(progress, history.Progress) || savedSettings != settings {
			return errors.New("fixture state failed reopen verification")
		}
		return nil
	}()
	return errors.Join(verifyErr, reopened.Close())
}
