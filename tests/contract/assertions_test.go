// SPDX-License-Identifier: MPL-2.0

package contract

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"animeportable/core"
)

type deceptiveError string

func (err deceptiveError) Error() string { return string(err) }
func (deceptiveError) GoString() string  { return "redacted" }

type plusVOnlyError struct{}

func (plusVOnlyError) Error() string { return "safe" }
func (plusVOnlyError) Format(state fmt.State, verb rune) {
	if verb == 'v' && state.Flag('+') {
		_, _ = io.WriteString(state, "plus-v-secret")
		return
	}
	_, _ = io.WriteString(state, "safe")
}

func TestValidatorsRejectBrokenAdapters(t *testing.T) {
	anime := core.SourceRef{Provider: "source", ID: "anime"}
	episode := core.EpisodeRef{Anime: anime, ID: "episode"}
	cases := map[string]func() error{
		"source ref":   func() error { return validateSourceRef(core.SourceRef{}) },
		"metadata ref": func() error { return validateMetadataRef(core.MetadataRef{}) },
		"URL":          func() error { return validateAbsoluteURL("relative") },
		"forbidden":    func() error { return validateForbidden("raw-secret", []string{"secret"}) },
		"error string": func() error { return validateForbidden(deceptiveError("raw-secret"), []string{"secret"}) },
		"membership": func() error {
			return validateSourceMembership([]core.SourceRef{anime}, []core.SourceRef{{Provider: "source", ID: "missing"}})
		},
		"duplicate membership": func() error {
			return validateSourceMembership([]core.SourceRef{anime}, []core.SourceRef{anime, anime})
		},
		"episode order": func() error {
			return validateEpisodeOrder([]core.SourceEpisode{{Ref: episode}}, anime, []core.EpisodeRef{{Anime: anime, ID: "other"}})
		},
		"schedule": func() error {
			return validateSchedule([]core.SourceScheduleItem{{Anime: core.SourceAnime{Ref: anime}, Episode: core.SourceEpisode{Ref: episode}, AirsAt: time.Time{}}}, []core.SourceScheduleItem{{Anime: core.SourceAnime{Ref: anime}, Episode: core.SourceEpisode{Ref: episode}, AirsAt: time.Time{}}})
		},
		"metadata candidates": func() error {
			return validateMetadataCandidates(nil, []core.MetadataRef{{Provider: "metadata", ID: "missing"}})
		},
		"duplicate metadata candidates": func() error {
			ref := core.MetadataRef{Provider: "metadata", ID: "anime"}
			return validateMetadataCandidates([]core.MetadataCandidate{{Ref: ref}}, []core.MetadataRef{ref, ref})
		},
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("validator accepted broken adapter output")
			}
		})
	}
}

func TestScheduleValidatorAllowsDistinctEpisodesAtSameTime(t *testing.T) {
	anime := core.SourceRef{Provider: "source", ID: "anime"}
	instant := time.Date(2026, time.January, 1, 10, 0, 0, 0, time.UTC)
	items := []core.SourceScheduleItem{
		{Anime: core.SourceAnime{Ref: anime, Title: "Anime"}, Episode: core.SourceEpisode{Ref: core.EpisodeRef{Anime: anime, ID: "1"}}, AirsAt: instant, Precision: core.SchedulePrecisionTime},
		{Anime: core.SourceAnime{Ref: anime, Title: "Anime"}, Episode: core.SourceEpisode{Ref: core.EpisodeRef{Anime: anime, ID: "2"}}, AirsAt: instant, Precision: core.SchedulePrecisionTime},
	}
	if err := validateSchedule(items, items); err != nil {
		t.Fatal(err)
	}
}

func TestForbiddenValidatorChecksPlusVFormatting(t *testing.T) {
	value := plusVOnlyError{}
	if got := fmt.Sprintf("%v", value); got != "safe" {
		t.Fatalf("%%v = %q", got)
	}
	if got := fmt.Sprintf("%#v", value); got != "safe" {
		t.Fatalf("%%#v = %q", got)
	}
	if got := fmt.Sprintf("%+v", value); got != "plus-v-secret" {
		t.Fatalf("%%+v = %q", got)
	}
	if err := validateForbidden(value, []string{"plus-v-secret"}); err == nil {
		t.Fatal("validator missed a secret exposed only by plus-v formatting")
	}
}

func TestMetadataDisplayValidatorsRejectUnsafeOrNoncanonicalFields(t *testing.T) {
	ref := core.MetadataRef{Provider: "metadata", ID: "anime"}
	candidate := core.MetadataCandidate{Ref: ref, Title: "Anime", NativeTitle: "アニメ", Season: "WINTER"}
	metadata := core.AnimeMetadata{
		Ref:         ref,
		Title:       "Anime",
		NativeTitle: "アニメ",
		Description: "Plain synopsis",
		CoverURL:    "https://images.example/cover.jpg",
		Season:      "WINTER",
		Studio:      "Studio",
	}
	cases := map[string]func() error{
		"candidate title markup": func() error {
			value := candidate
			value.Title = "<b>Anime</b>"
			return validateMetadataCandidate(value)
		},
		"candidate title Markdown": func() error {
			value := candidate
			value.Title = "**Anime**"
			return validateMetadataCandidate(value)
		},
		"candidate native title control": func() error {
			value := candidate
			value.NativeTitle = "ア\x00ニメ"
			return validateMetadataCandidate(value)
		},
		"candidate season whitespace": func() error {
			value := candidate
			value.Season = " WINTER "
			return validateMetadataCandidate(value)
		},
		"metadata title markup": func() error {
			value := metadata
			value.Title = "<b>Anime</b>"
			return validateAnimeMetadata(value)
		},
		"metadata native title newline": func() error {
			value := metadata
			value.NativeTitle = "アニメ\n二期"
			return validateAnimeMetadata(value)
		},
		"metadata description markup": func() error {
			value := metadata
			value.Description = "<p>Synopsis</p>"
			return validateAnimeMetadata(value)
		},
		"metadata description encoded markup": func() error {
			value := metadata
			value.Description = "&lt;b&gt;Synopsis&lt;/b&gt;"
			return validateAnimeMetadata(value)
		},
		"metadata season markup": func() error {
			value := metadata
			value.Season = "<i>WINTER</i>"
			return validateAnimeMetadata(value)
		},
		"metadata studio control": func() error {
			value := metadata
			value.Studio = "Studio\x00Secret"
			return validateAnimeMetadata(value)
		},
		"metadata insecure cover": func() error {
			value := metadata
			value.CoverURL = "http://images.example/cover.jpg"
			return validateAnimeMetadata(value)
		},
		"metadata oversized cover": func() error {
			value := metadata
			value.CoverURL = "https://images.example/" + strings.Repeat("x", 8<<10)
			return validateAnimeMetadata(value)
		},
	}
	for name, validate := range cases {
		t.Run(name, func(t *testing.T) {
			if err := validate(); err == nil {
				t.Fatal("validator accepted unsafe metadata")
			}
		})
	}
	if err := validateMetadataCandidate(candidate); err != nil {
		t.Fatal(err)
	}
	if err := validateAnimeMetadata(metadata); err != nil {
		t.Fatal(err)
	}
}
