// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"animeportable/core"
)

func TestMetadataRejectsUnsafeRemoteContent(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	base := core.AnimeMetadata{
		Ref:         core.MetadataRef{Provider: "provider", ID: "metadata"},
		Title:       "Title",
		NativeTitle: "Native title",
		Description: "Plain description",
		CoverURL:    "https://example.test/cover.jpg",
		Season:      "SPRING",
		Studio:      "Studio",
	}
	tests := []struct {
		name   string
		mutate func(*core.AnimeMetadata)
	}{
		{name: "title markup", mutate: func(value *core.AnimeMetadata) { value.Title = "<b>Title</b>" }},
		{name: "native title encoded markup", mutate: func(value *core.AnimeMetadata) { value.NativeTitle = "&lt;script&gt;bad&lt;/script&gt;" }},
		{name: "description markup", mutate: func(value *core.AnimeMetadata) { value.Description = "<script>bad</script>" }},
		{name: "studio control", mutate: func(value *core.AnimeMetadata) { value.Studio = "Studio\x00bad" }},
		{name: "season noncanonical", mutate: func(value *core.AnimeMetadata) { value.Season = " SPRING " }},
		{name: "cover HTTP", mutate: func(value *core.AnimeMetadata) { value.CoverURL = "http://example.test/cover.jpg" }},
		{name: "cover userinfo", mutate: func(value *core.AnimeMetadata) { value.CoverURL = "https://user:secret@example.test/cover.jpg" }},
		{name: "cover fragment", mutate: func(value *core.AnimeMetadata) { value.CoverURL = "https://example.test/cover.jpg#fragment" }},
		{name: "cover custom port", mutate: func(value *core.AnimeMetadata) { value.CoverURL = "https://example.test:444/cover.jpg" }},
		{name: "cover opaque", mutate: func(value *core.AnimeMetadata) { value.CoverURL = "https:cover.jpg" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata := base
			test.mutate(&metadata)
			if err := store.SaveMetadata(ctx, "anime", metadata); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestMetadataRejectsCorruptCachedContent(t *testing.T) {
	tests := []struct {
		name  string
		query string
		value string
	}{
		{name: "raw markup", query: `UPDATE metadata SET description = ? WHERE anime_id = 'anime'`, value: "<script>cached-secret</script>"},
		{name: "unsafe cover", query: `UPDATE metadata SET cover_url = ? WHERE anime_id = 'anime'`, value: "http://cached-secret.example/cover.jpg"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
			defer store.Close()
			ctx := context.Background()
			if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
				t.Fatal(err)
			}
			metadata := core.AnimeMetadata{
				Ref:         core.MetadataRef{Provider: "provider", ID: "metadata"},
				Title:       "Title",
				Description: "Description",
				CoverURL:    "https://example.test/cover.jpg",
			}
			if err := store.SaveMetadata(ctx, "anime", metadata); err != nil {
				t.Fatal(err)
			}
			if err := store.withDB(ctx, func(db *sql.DB) error {
				_, err := db.ExecContext(ctx, test.query, test.value)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			got, err := store.Metadata(ctx, "anime")
			if got != (core.AnimeMetadata{}) || !errors.Is(err, ErrStorage) {
				t.Fatalf("metadata = %#v, error = %v", got, err)
			}
			if strings.Contains(err.Error(), "cached-secret") {
				t.Fatalf("error exposed cached content: %v", err)
			}
		})
	}
}
