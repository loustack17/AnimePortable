// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
	"testing"
)

type metadataRefreshStore struct {
	fakeStore
	title string
	saved []AnimeMetadata
}

func (store *metadataRefreshStore) Anime(_ context.Context, id AnimeID) (Anime, error) {
	return Anime{ID: id, Title: store.title}, nil
}

func (store *metadataRefreshStore) SaveMetadata(_ context.Context, _ AnimeID, metadata AnimeMetadata) error {
	store.saved = append(store.saved, metadata)
	return nil
}

type scriptedMetadataProvider struct {
	candidates []MetadataCandidate
	searchErr  error
	getErr     error
	gets       []MetadataRef
}

func (provider *scriptedMetadataProvider) Search(context.Context, MetadataQuery) ([]MetadataCandidate, error) {
	return provider.candidates, provider.searchErr
}

func (provider *scriptedMetadataProvider) Get(_ context.Context, ref MetadataRef) (AnimeMetadata, error) {
	provider.gets = append(provider.gets, ref)
	if provider.getErr != nil {
		return AnimeMetadata{}, provider.getErr
	}
	return AnimeMetadata{Ref: ref, Title: "fetched", CoverURL: "https://example.test/cover.jpg"}, nil
}

func TestRefreshMetadataStoresAcceptedMatchFromItsProvider(t *testing.T) {
	store := &metadataRefreshStore{title: "葬送的芙莉蓮"}
	anilist := &scriptedMetadataProvider{searchErr: errors.New("down")}
	bangumi := &scriptedMetadataProvider{candidates: []MetadataCandidate{{Ref: MetadataRef{Provider: "bangumi", ID: "400602"}, Title: "葬送的芙莉莲", Year: 2023}}}
	found, err := NewApp(nil, nil, store).RefreshMetadata(context.Background(), "anime", []MetadataProvider{anilist, bangumi})
	if err != nil || !found {
		t.Fatalf("found=%t err=%v", found, err)
	}
	if len(bangumi.gets) != 1 || len(anilist.gets) != 0 || len(store.saved) != 1 || store.saved[0].Ref.ID != "400602" {
		t.Fatalf("gets anilist=%v bangumi=%v saved=%+v", anilist.gets, bangumi.gets, store.saved)
	}
}

func TestRefreshMetadataStoresNothingWithoutAcceptedMatch(t *testing.T) {
	store := &metadataRefreshStore{title: "Great Anime"}
	provider := &scriptedMetadataProvider{candidates: []MetadataCandidate{{Ref: MetadataRef{Provider: "anilist", ID: "1"}, Title: "Unrelated Work"}}}
	found, err := NewApp(nil, nil, store).RefreshMetadata(context.Background(), "anime", []MetadataProvider{provider})
	if err != nil || found || len(provider.gets) != 0 || len(store.saved) != 0 {
		t.Fatalf("found=%t err=%v gets=%v saved=%v", found, err, provider.gets, store.saved)
	}
}

func TestRefreshMetadataReportsUnavailableWhenEveryProviderFails(t *testing.T) {
	store := &metadataRefreshStore{title: "Great Anime"}
	providers := []MetadataProvider{&scriptedMetadataProvider{searchErr: errors.New("a")}, &scriptedMetadataProvider{searchErr: errors.New("b")}}
	if _, err := NewApp(nil, nil, store).RefreshMetadata(context.Background(), "anime", providers); !errors.Is(err, ErrMetadataUnavailable) {
		t.Fatalf("err = %v, want ErrMetadataUnavailable", err)
	}
}

func TestRefreshMetadataReportsUnavailableWhenGetFailsAfterMatch(t *testing.T) {
	store := &metadataRefreshStore{title: "葬送的芙莉蓮"}
	provider := &scriptedMetadataProvider{getErr: errors.New("down"), candidates: []MetadataCandidate{{Ref: MetadataRef{Provider: "bangumi", ID: "1"}, Title: "葬送的芙莉蓮"}}}
	found, err := NewApp(nil, nil, store).RefreshMetadata(context.Background(), "anime", []MetadataProvider{provider})
	if found || !errors.Is(err, ErrMetadataUnavailable) || len(store.saved) != 0 {
		t.Fatalf("found=%t err=%v saved=%v", found, err, store.saved)
	}
}

func TestRefreshMetadataWithoutProvidersFindsNothing(t *testing.T) {
	found, err := NewApp(nil, nil, &metadataRefreshStore{title: "Anime"}).RefreshMetadata(context.Background(), "anime", nil)
	if found || err != nil {
		t.Fatalf("found=%t err=%v", found, err)
	}
}
