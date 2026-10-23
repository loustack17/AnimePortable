// SPDX-License-Identifier: MPL-2.0

package bangumi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"testing"

	"animeportable/adapters/network/securehttp"
)

// browseServer answers browse pages for a season of total anime; pages run concurrently.
type browseServer struct {
	t       *testing.T
	total   int
	failAt  int // offset that fails with a 503, or -1
	mu      sync.Mutex
	queries []string
}

func (server *browseServer) Do(request *http.Request) (*securehttp.Response, error) {
	server.mu.Lock()
	server.queries = append(server.queries, request.URL.RawQuery)
	server.mu.Unlock()
	if request.Method != http.MethodGet || request.URL.Path != "/v0/subjects" || request.Header.Get("User-Agent") != userAgent {
		server.t.Errorf("request = %s %s", request.Method, request.URL)
	}
	offset, _ := strconv.Atoi(request.URL.Query().Get("offset"))
	if offset == server.failAt {
		return statusResponse(http.StatusServiceUnavailable, []byte(`{}`)), nil
	}
	data := []map[string]any{}
	for index := offset; index < min(server.total, offset+browsePageSize); index++ {
		data = append(data, map[string]any{"id": index + 1, "type": 2, "name": fmt.Sprintf("作品%d", index+1), "name_cn": "", "date": "2026-04-01", "eps": 12})
	}
	body, err := json.Marshal(map[string]any{"total": server.total, "limit": browsePageSize, "offset": offset, "data": data})
	if err != nil {
		return nil, err
	}
	return rawJSONResponse(body), nil
}

func TestBrowseSeasonListsEveryPageOfTheSeasonStartMonth(t *testing.T) {
	for _, test := range []struct {
		total, requests int
	}{
		{total: 0, requests: 3},
		{total: 130, requests: 3},
		{total: 150, requests: 3},
		{total: 151, requests: 4},
		{total: 260, requests: 6},
		{total: 5000, requests: maxBrowseSubjects / browsePageSize},
	} {
		server := &browseServer{t: t, total: test.total, failAt: -1}
		client := newWithDo(server)
		found, err := client.BrowseSeason(context.Background(), 2026, "spring")
		want := min(test.total, maxBrowseSubjects)
		if err != nil || len(found) != want || len(server.queries) != test.requests {
			t.Fatalf("total %d: found=%d err=%v requests=%d, want %d anime from %d requests", test.total, len(found), err, len(server.queries), want, test.requests)
		}
		for index, candidate := range found {
			if candidate.Ref.ID != strconv.Itoa(index+1) || candidate.Year != 2026 {
				t.Fatalf("total %d: candidate %d = %+v, want id %d in page order", test.total, index, candidate, index+1)
			}
		}
		if !slices.Contains(server.queries, "type=2&year=2026&month=4&limit=50&offset=0") {
			t.Fatalf("queries = %v", server.queries)
		}
	}
}

func TestBrowseSeasonIsFetchedOncePerRun(t *testing.T) {
	server := &browseServer{t: t, total: 40, failAt: -1}
	client := newWithDo(server)
	for range 3 {
		if found, err := client.BrowseSeason(context.Background(), 2025, "fall"); err != nil || len(found) != 40 {
			t.Fatalf("found=%d err=%v", len(found), err)
		}
	}
	if len(server.queries) != 3 {
		t.Fatalf("requests = %d, want one round of 3", len(server.queries))
	}
	if _, err := client.BrowseSeason(context.Background(), 2025, "summer"); err != nil || len(server.queries) != 6 {
		t.Fatalf("another season: err=%v requests=%d", err, len(server.queries))
	}
}

func TestBrowseSeasonFailsWholeAndCachesNothing(t *testing.T) {
	server := &browseServer{t: t, total: 130, failAt: 50}
	client := newWithDo(server)
	if found, err := client.BrowseSeason(context.Background(), 2026, "winter"); err == nil || found != nil {
		t.Fatalf("a failed page: found=%d err=%v, want an error and nothing", len(found), err)
	}
	server.failAt = -1
	if found, err := client.BrowseSeason(context.Background(), 2026, "winter"); err != nil || len(found) != 130 {
		t.Fatalf("after the failure: found=%d err=%v", len(found), err)
	}
	for _, test := range []struct {
		year   int
		season string
	}{{2026, "冬"}, {2026, ""}, {1899, "winter"}, {3001, "winter"}} {
		if _, err := client.BrowseSeason(context.Background(), test.year, test.season); !errors.Is(err, errMetadataInvalidQuery) {
			t.Fatalf("BrowseSeason(%d, %q) = %v", test.year, test.season, err)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.BrowseSeason(canceled, 2024, "fall"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled = %v", err)
	}
}

// gatedServer holds every browse request until release closes; a request at failAt fails at once.
type gatedServer struct {
	browseServer
	release  chan struct{}
	canceled chan struct{}
	arrived  int // requests seen, counted before they are held
}

func (server *gatedServer) Do(request *http.Request) (*securehttp.Response, error) {
	server.mu.Lock()
	server.arrived++
	server.mu.Unlock()
	offset, _ := strconv.Atoi(request.URL.Query().Get("offset"))
	if offset != server.failAt {
		select {
		case <-server.release:
		case <-request.Context().Done():
			server.canceled <- struct{}{}
			return nil, request.Context().Err()
		}
	}
	return server.browseServer.Do(request)
}

func TestBrowseSeasonSharesOneFetchBetweenConcurrentLookups(t *testing.T) {
	server := &gatedServer{browseServer: browseServer{t: t, total: 120, failAt: -1}, release: make(chan struct{}), canceled: make(chan struct{}, 8)}
	client := newWithDo(server)
	var wait sync.WaitGroup
	results := make([]int, 4)
	for index := range results {
		wait.Add(1)
		go func() {
			defer wait.Done()
			found, err := client.BrowseSeason(context.Background(), 2026, "fall")
			if err != nil {
				t.Errorf("lookup %d: %v", index, err)
			}
			results[index] = len(found)
		}()
	}
	for {
		server.mu.Lock()
		asked := server.arrived
		server.mu.Unlock()
		if asked >= 3 {
			break
		}
		runtime.Gosched()
	}
	close(server.release)
	wait.Wait()
	if len(server.queries) != 3 || !slices.Equal(results, []int{120, 120, 120, 120}) {
		t.Fatalf("requests=%d results=%v, want one fetch of 3 pages shared by all", len(server.queries), results)
	}
}

func TestBrowseSeasonFailedPageCancelsTheOthers(t *testing.T) {
	server := &gatedServer{browseServer: browseServer{t: t, total: 130, failAt: 100}, release: make(chan struct{}), canceled: make(chan struct{}, 8)}
	if _, err := newWithDo(server).BrowseSeason(context.Background(), 2026, "spring"); err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the failed page's error", err)
	}
	if len(server.canceled) != 2 {
		t.Fatalf("canceled sibling requests = %d, want 2", len(server.canceled))
	}
}

func TestBrowseSeasonWaiterFetchesAgainWhenTheSharedFetchIsCancelled(t *testing.T) {
	server := &gatedServer{browseServer: browseServer{t: t, total: 120, failAt: -1}, release: make(chan struct{}), canceled: make(chan struct{}, 8)}
	client := newWithDo(server)
	first, cancelFirst := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() {
		_, err := client.BrowseSeason(first, 2026, "winter")
		firstDone <- err
	}()
	arrived := func() int {
		server.mu.Lock()
		defer server.mu.Unlock()
		return server.arrived
	}
	for arrived() < 3 {
		runtime.Gosched()
	}
	second := make(chan int, 1)
	go func() {
		found, err := client.BrowseSeason(context.Background(), 2026, "winter")
		if err != nil {
			t.Errorf("second lookup: %v", err)
		}
		second <- len(found)
	}()
	cancelFirst() // the page of the first lookup closes
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("first lookup = %v, want context.Canceled", err)
	}
	for arrived() < 6 {
		runtime.Gosched()
	}
	close(server.release)
	if got := <-second; got != 120 {
		t.Fatalf("second lookup found %d, want the whole season of 120", got)
	}
}
