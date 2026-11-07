// SPDX-License-Identifier: MPL-2.0

package contract

import (
	"strings"
	"testing"

	"animeportable/core"
)

func TestMetadataDisplayValidatorsRejectUnsafeOrNoncanonicalFields(t *testing.T) {
	ref := core.MetadataRef{Provider: "metadata", ID: "anime"}
	candidate := core.MetadataCandidate{Ref: ref, Title: "Anime", NativeTitle: "アニメ"}
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
