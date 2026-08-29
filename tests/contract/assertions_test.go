package contract

import (
	"fmt"
	"io"
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
