// SPDX-License-Identifier: MPL-2.0

package aniskip

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
)

type fakeDo struct {
	status      int
	contentType string
	body        string
	err         error
	requests    []*http.Request
}

func (f *fakeDo) Do(request *http.Request) (*securehttp.Response, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return nil, f.err
	}
	contentType := f.contentType
	if contentType == "" {
		contentType = "application/json; charset=utf-8"
	}
	return &securehttp.Response{StatusCode: f.status, Header: http.Header{"Content-Type": {contentType}}, Body: []byte(f.body)}, nil
}

const twoOpenings = `{"found":true,"results":[
 {"interval":{"startTime":90.5,"endTime":180.5},"skipType":"op","skipId":"a","episodeLength":1420.1},
 {"interval":{"startTime":0,"endTime":89},"skipType":"mixed-op","skipId":"b","episodeLength":1440}
],"message":"Successfully found skip times","statusCode":200}`

func TestOpeningsSendsOnlyTheThreeNumbers(t *testing.T) {
	do := &fakeDo{status: http.StatusOK, body: twoOpenings}
	got, err := newWithDo(do, "0.3.0-alpha").Openings(context.Background(), 52991, 7, 1420600*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	want := []core.OpeningCandidate{
		{Opening: core.Opening{Start: 90500 * time.Millisecond, End: 180500 * time.Millisecond}, EpisodeLength: 1420100 * time.Millisecond},
		{Opening: core.Opening{Start: 0, End: 89 * time.Second}, EpisodeLength: 1440 * time.Second},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("candidates = %+v", got)
	}
	request := do.requests[0]
	if request.Method != http.MethodGet || request.URL.String() != "https://api.aniskip.com/v2/skip-times/52991/7?types=op&types=mixed-op&episodeLength=1421" {
		t.Fatal(request.Method, request.URL)
	}
	if request.Body != nil || request.Header.Get("User-Agent") != "AnimePortable/0.3.0-alpha" || request.Header.Get("Cookie") != "" || len(request.Header) != 2 {
		t.Fatal(request.Header)
	}
}

func TestOpeningsWithoutDataAreEmpty(t *testing.T) {
	for name, do := range map[string]*fakeDo{
		"404":       {status: http.StatusNotFound, body: `{"found":false,"results":[],"statusCode":404}`},
		"not found": {status: http.StatusOK, body: `{"found":false,"results":[]}`},
	} {
		got, err := newWithDo(do, "1").Openings(context.Background(), 1, 1, time.Minute)
		if err != nil || len(got) != 0 {
			t.Errorf("%s: %v, %v", name, got, err)
		}
	}
}

func TestOpeningsDropResultsThatAreNotCompleteOpenings(t *testing.T) {
	body := `{"found":true,"results":[
	 {"interval":{"startTime":10,"endTime":100},"skipType":"ed","episodeLength":1400},
	 {"interval":{"startTime":10,"endTime":100},"skipType":"recap","episodeLength":1400},
	 {"interval":{"endTime":100},"skipType":"op","episodeLength":1400},
	 {"interval":{"startTime":-1,"endTime":100},"skipType":"op","episodeLength":1400},
	 {"interval":{"startTime":10,"endTime":100},"skipType":"op"},
	 {"interval":{"startTime":10,"endTime":1e300},"skipType":"op","episodeLength":1400},
	 {"interval":{"startTime":10,"endTime":100},"skipType":"op","episodeLength":1400}
	]}`
	got, err := newWithDo(&fakeDo{status: http.StatusOK, body: body}, "1").Openings(context.Background(), 1, 1, 1400*time.Second)
	if err != nil || len(got) != 1 || got[0].Start != 10*time.Second {
		t.Fatal(got, err)
	}
}

func TestOpeningsRejectMalformedAnswers(t *testing.T) {
	many := `{"found":true,"results":[` + strings.TrimSuffix(strings.Repeat(`{"interval":{"startTime":1,"endTime":2},"skipType":"op","episodeLength":3},`, maxResults+1), ",") + `]}`
	for name, do := range map[string]*fakeDo{
		"server error":  {status: http.StatusInternalServerError, body: `{}`},
		"not json":      {status: http.StatusOK, body: `<html></html>`, contentType: "text/html"},
		"bad json":      {status: http.StatusOK, body: `{"found":true,"results":[`},
		"wrong type":    {status: http.StatusOK, body: `{"found":true,"results":{"a":1}}`},
		"too large":     {status: http.StatusOK, body: `{"found":false,"pad":"` + strings.Repeat("x", MaxResponseBytes) + `"}`},
		"too many":      {status: http.StatusOK, body: many},
		"network error": {err: errors.New("offline")},
	} {
		if got, err := newWithDo(do, "1").Openings(context.Background(), 1, 1, time.Minute); err == nil {
			t.Errorf("%s: accepted %v", name, got)
		}
	}
}

func TestOpeningsRefuseInvalidInputWithoutARequest(t *testing.T) {
	for _, input := range []struct {
		id, episode int
		length      time.Duration
	}{{0, 1, time.Minute}, {1, 0, time.Minute}, {1, 1, 0}, {1, 1, 400 * time.Millisecond}, {1, 1, 25 * time.Hour}} {
		do := &fakeDo{status: http.StatusOK, body: twoOpenings}
		if _, err := newWithDo(do, "1").Openings(context.Background(), input.id, input.episode, input.length); err == nil || len(do.requests) != 0 {
			t.Errorf("%+v: err %v, %d requests", input, err, len(do.requests))
		}
	}
}

func TestAllowedOriginsIsAniSkipOnly(t *testing.T) {
	if got := AllowedOrigins(); len(got) != 1 || got[0] != "https://api.aniskip.com" {
		t.Fatal(got)
	}
}
