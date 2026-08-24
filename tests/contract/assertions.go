package contract

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"animeportable/core"
)

func validateSourceRef(ref core.SourceRef) error {
	if strings.TrimSpace(ref.Provider) == "" || strings.TrimSpace(ref.ID) == "" {
		return fmt.Errorf("invalid source ref: %#v", ref)
	}
	return nil
}

func validateMetadataRef(ref core.MetadataRef) error {
	if strings.TrimSpace(ref.Provider) == "" || strings.TrimSpace(ref.ID) == "" {
		return fmt.Errorf("invalid metadata ref: %#v", ref)
	}
	return nil
}

func validateAbsoluteURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return errors.New("playback URL is invalid")
	}
	if parsed.IsAbs() && parsed.Host != "" {
		return nil
	}
	return errors.New("playback URL is not absolute")
}

func validateForbidden(value any, forbidden []string) error {
	formatted := fmt.Sprintf("%v\n%#v", value, value)
	for _, sentinel := range forbidden {
		if sentinel != "" && strings.Contains(formatted, sentinel) {
			return errors.New("value contains forbidden content")
		}
	}
	return nil
}

func validateSourceMembership(actual []core.SourceRef, expected []core.SourceRef) error {
	if len(expected) == 0 {
		return fmt.Errorf("expected source refs are empty")
	}
	for _, ref := range actual {
		if err := validateSourceRef(ref); err != nil {
			return err
		}
	}
	consumed := make([]bool, len(actual))
	for _, expectedRef := range expected {
		found := false
		for index, actualRef := range actual {
			if !consumed[index] && actualRef == expectedRef {
				consumed[index] = true
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing source ref %#v", expectedRef)
		}
	}
	return nil
}

func validateEpisodeOrder(actual []core.SourceEpisode, anime core.SourceRef, expected []core.EpisodeRef) error {
	if len(expected) == 0 {
		return fmt.Errorf("expected episodes are empty")
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("episode count = %d, want %d", len(actual), len(expected))
	}
	for index, episode := range actual {
		if episode.Ref.Anime != anime || strings.TrimSpace(episode.Ref.ID) == "" {
			return fmt.Errorf("invalid episode ref: %#v", episode.Ref)
		}
		if episode.Ref != expected[index] {
			return fmt.Errorf("episode %d = %#v, want %#v", index, episode.Ref, expected[index])
		}
	}
	return nil
}

func validateSchedule(items []core.SourceScheduleItem, expected []core.EpisodeRef) error {
	if len(expected) == 0 {
		return fmt.Errorf("expected schedule is empty")
	}
	if len(items) != len(expected) {
		return fmt.Errorf("schedule count = %d, want %d", len(items), len(expected))
	}
	var previous time.Time
	for index, item := range items {
		if err := validateSourceRef(item.Anime.Ref); err != nil {
			return err
		}
		if item.Episode.Ref.Anime != item.Anime.Ref || strings.TrimSpace(item.Episode.Ref.ID) == "" {
			return fmt.Errorf("unlinked schedule item: %#v", item)
		}
		if item.AirsAt.IsZero() || (!previous.IsZero() && item.AirsAt.Before(previous)) {
			return fmt.Errorf("schedule is not normalized at %d", index)
		}
		if item.Episode.Ref != expected[index] {
			return fmt.Errorf("schedule episode %d = %#v, want %#v", index, item.Episode.Ref, expected[index])
		}
		previous = item.AirsAt
	}
	return nil
}

func validateMetadataCandidates(actual []core.MetadataCandidate, expected []core.MetadataRef) error {
	if len(expected) == 0 {
		return fmt.Errorf("expected metadata refs are empty")
	}
	refs := make([]core.MetadataRef, 0, len(actual))
	for _, candidate := range actual {
		if err := validateMetadataRef(candidate.Ref); err != nil {
			return err
		}
		refs = append(refs, candidate.Ref)
	}
	consumed := make([]bool, len(refs))
	for _, expectedRef := range expected {
		found := false
		for index, actualRef := range refs {
			if !consumed[index] && actualRef == expectedRef {
				consumed[index] = true
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing metadata ref %#v", expectedRef)
		}
	}
	return nil
}

func equal[T any](actual, expected T) error {
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("actual = %#v, expected %#v", actual, expected)
	}
	return nil
}
