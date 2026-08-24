package contract

import (
	"testing"
	"time"

	"animeportable/core"
)

type deceptiveError string

func (err deceptiveError) Error() string { return string(err) }
func (deceptiveError) GoString() string  { return "redacted" }

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
			return validateSchedule([]core.SourceScheduleItem{{Anime: core.SourceAnime{Ref: anime}, Episode: core.SourceEpisode{Ref: episode}, AirsAt: time.Time{}}}, []core.EpisodeRef{episode})
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
