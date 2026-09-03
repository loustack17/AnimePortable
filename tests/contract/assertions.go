// SPDX-License-Identifier: MPL-2.0

package contract

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"animeportable/core"
	metadatapolicy "animeportable/internal/metadata"
)

const (
	metadataTitleBytes       = 4 << 10
	metadataTitleRunes       = 1024
	metadataDescriptionBytes = 64 << 10
	metadataDescriptionRunes = 16384
	metadataSeasonBytes      = 128
	metadataSeasonRunes      = 128
	metadataStudioBytes      = 1 << 10
	metadataStudioRunes      = 1024
	metadataCoverURLBytes    = 8 << 10
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
	formatted := fmt.Sprintf("%v\n%+v\n%#v", value, value, value)
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

func validateSchedule(items []core.SourceScheduleItem, expected []core.SourceScheduleItem) error {
	if len(expected) == 0 {
		return fmt.Errorf("expected schedule is empty")
	}
	if len(items) != len(expected) {
		return fmt.Errorf("schedule count = %d, want %d", len(items), len(expected))
	}
	var previous time.Time
	type scheduleIdentity struct {
		anime     core.SourceRef
		episode   core.EpisodeRef
		airsAtUTC string
		precision core.SchedulePrecision
	}
	seen := make(map[scheduleIdentity]struct{}, len(items))
	for index, item := range items {
		if err := validateScheduleItem(item); err != nil {
			return err
		}
		if item.AirsAt.IsZero() || (!previous.IsZero() && item.AirsAt.Before(previous)) {
			return fmt.Errorf("schedule is not normalized at %d", index)
		}
		key := scheduleIdentity{anime: item.Anime.Ref, episode: item.Episode.Ref, airsAtUTC: item.AirsAt.UTC().Format(time.RFC3339Nano), precision: item.Precision}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate schedule item at %d", index)
		}
		seen[key] = struct{}{}
		if !reflect.DeepEqual(item, expected[index]) {
			return fmt.Errorf("schedule item %d = %#v, want %#v", index, item, expected[index])
		}
		previous = item.AirsAt
	}
	return nil
}

func validateScheduleItem(item core.SourceScheduleItem) error {
	if err := validateSourceRef(item.Anime.Ref); err != nil {
		return err
	}
	if strings.TrimSpace(item.Anime.Title) == "" || item.Episode.Ref.Anime != item.Anime.Ref {
		return fmt.Errorf("unlinked schedule item: %#v", item)
	}
	if item.Episode.Ref.ID == "" {
		if item.Episode.Number != "" || item.Episode.Title != "" {
			return fmt.Errorf("unknown schedule episode has metadata: %#v", item)
		}
	} else if strings.TrimSpace(item.Episode.Ref.ID) == "" {
		return fmt.Errorf("invalid schedule episode: %#v", item)
	}
	if item.AirsAt.IsZero() || item.Precision != core.SchedulePrecisionDay && item.Precision != core.SchedulePrecisionTime {
		return fmt.Errorf("invalid schedule time: %#v", item)
	}
	return nil
}

func validateMetadataCandidates(actual []core.MetadataCandidate, expected []core.MetadataRef) error {
	if len(expected) == 0 {
		return fmt.Errorf("expected metadata refs are empty")
	}
	refs := make([]core.MetadataRef, 0, len(actual))
	for _, candidate := range actual {
		if err := validateMetadataCandidate(candidate); err != nil {
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

func validateMetadataCandidate(candidate core.MetadataCandidate) error {
	if err := validateMetadataRef(candidate.Ref); err != nil {
		return err
	}
	for _, field := range []struct {
		name     string
		value    string
		maxBytes int
		maxRunes int
	}{
		{name: "title", value: candidate.Title, maxBytes: metadataTitleBytes, maxRunes: metadataTitleRunes},
		{name: "native title", value: candidate.NativeTitle, maxBytes: metadataTitleBytes, maxRunes: metadataTitleRunes},
		{name: "season", value: candidate.Season, maxBytes: metadataSeasonBytes, maxRunes: metadataSeasonRunes},
	} {
		if err := validateMetadataText(field.name, field.value, field.maxBytes, field.maxRunes); err != nil {
			return err
		}
	}
	return nil
}

func validateAnimeMetadata(value core.AnimeMetadata) error {
	if err := validateMetadataRef(value.Ref); err != nil {
		return err
	}
	fields := []struct {
		name     string
		value    string
		maxBytes int
		maxRunes int
	}{
		{name: "title", value: value.Title, maxBytes: metadataTitleBytes, maxRunes: metadataTitleRunes},
		{name: "native title", value: value.NativeTitle, maxBytes: metadataTitleBytes, maxRunes: metadataTitleRunes},
		{name: "description", value: value.Description, maxBytes: metadataDescriptionBytes, maxRunes: metadataDescriptionRunes},
		{name: "season", value: value.Season, maxBytes: metadataSeasonBytes, maxRunes: metadataSeasonRunes},
		{name: "studio", value: value.Studio, maxBytes: metadataStudioBytes, maxRunes: metadataStudioRunes},
	}
	for _, field := range fields {
		if err := validateMetadataText(field.name, field.value, field.maxBytes, field.maxRunes); err != nil {
			return err
		}
	}
	return validateMetadataCoverURL(value.CoverURL)
}

func validateMetadataText(name, value string, maxBytes, maxRunes int) error {
	if !utf8.ValidString(value) || len(value) > maxBytes || utf8.RuneCountInString(value) > maxRunes || strings.Join(strings.Fields(value), " ") != value {
		return fmt.Errorf("metadata %s is not bounded canonical text", name)
	}
	for _, character := range value {
		if unicode.IsControl(character) || character == '<' || character == '>' {
			return fmt.Errorf("metadata %s contains unsafe text", name)
		}
	}
	limits := metadatapolicy.PlainTextLimits{MaxInputBytes: maxBytes, MaxOutputBytes: maxBytes, MaxOutputRunes: maxRunes}
	if !metadatapolicy.IsCanonicalPlainText(value, limits) {
		return fmt.Errorf("metadata %s does not satisfy the canonical display policy", name)
	}
	return nil
}

func validateMetadataCoverURL(value string) error {
	if value == "" {
		return nil
	}
	if !utf8.ValidString(value) || len(value) > metadataCoverURLBytes {
		return errors.New("metadata cover URL is invalid")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return errors.New("metadata cover URL contains a control character")
		}
	}
	target, err := url.Parse(value)
	if err != nil || target == nil || !strings.EqualFold(target.Scheme, "https") || target.Opaque != "" || target.User != nil || target.Fragment != "" || target.Host == "" || target.Hostname() == "" || target.Port() != "" || strings.HasSuffix(target.Host, ":") {
		return errors.New("metadata cover URL is not structurally safe")
	}
	return nil
}

func equal[T any](actual, expected T) error {
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("actual = %#v, expected %#v", actual, expected)
	}
	return nil
}
