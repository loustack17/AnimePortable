// SPDX-License-Identifier: MPL-2.0

package internal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	metadatapolicy "animeportable/internal/metadata"
)

var (
	errTestUnavailable = errors.New("test: unavailable")
	errTestMalformed   = errors.New("test: malformed")
	testRules          = Rules{MaxBodyBytes: 64, MaxNestingDepth: 4, Unavailable: errTestUnavailable, Malformed: errTestMalformed}
)

func jsonAnswer(status int, body string) *securehttp.Response {
	return &securehttp.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(body)}
}

func TestRulesCheckResponse(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	secure := &securehttp.Error{Kind: securehttp.KindNetwork}
	htmlAnswer := jsonAnswer(http.StatusOK, `{}`)
	htmlAnswer.Header.Set("Content-Type", "text/html")
	tests := []struct {
		name              string
		ctx               context.Context
		response          *securehttp.Response
		err               error
		notFoundIsMissing bool
		want              error
	}{
		{name: "ok", ctx: context.Background(), response: jsonAnswer(http.StatusOK, `{}`)},
		{name: "request error kept when safe", ctx: context.Background(), err: secure, want: secure},
		{name: "request error replaced", ctx: context.Background(), err: errors.New("raw"), want: errTestUnavailable},
		{name: "no response", ctx: context.Background(), want: errTestUnavailable},
		{name: "nil context", response: jsonAnswer(http.StatusOK, `{}`), want: errTestUnavailable},
		{name: "canceled after answer", ctx: canceled, response: jsonAnswer(http.StatusOK, `{}`), want: context.Canceled},
		{name: "not found is missing", ctx: context.Background(), response: jsonAnswer(http.StatusNotFound, `{}`), notFoundIsMissing: true, want: core.ErrNotFound},
		{name: "not found is an outage", ctx: context.Background(), response: jsonAnswer(http.StatusNotFound, `{}`), want: errTestUnavailable},
		{name: "server error", ctx: context.Background(), response: jsonAnswer(http.StatusBadGateway, `{}`), want: errTestUnavailable},
		{name: "redirect status", ctx: context.Background(), response: jsonAnswer(http.StatusMultipleChoices, `{}`), want: errTestUnavailable},
		{name: "body too large", ctx: context.Background(), response: jsonAnswer(http.StatusOK, `{"a":"`+strings.Repeat("x", 64)+`"}`), want: errTestMalformed},
		{name: "not json", ctx: context.Background(), response: htmlAnswer, want: errTestMalformed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := testRules.CheckResponse(test.ctx, test.response, test.err, test.notFoundIsMissing)
			if !errors.Is(err, test.want) || test.want == nil && err != nil {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if (err == nil) != (response != nil) {
				t.Fatalf("response = %v with error %v", response, err)
			}
		})
	}
}

func TestRulesDecode(t *testing.T) {
	var value struct{ A int }
	if err := testRules.Decode([]byte(`{"A":1}`), &value); err != nil || value.A != 1 {
		t.Fatalf("Decode = %v, value %+v", err, value)
	}
	if err := testRules.Decode([]byte(`{"A":1,"A":2}`), &value); !errors.Is(err, errTestMalformed) {
		t.Fatalf("duplicate key error = %v", err)
	}
}

func TestBoundedText(t *testing.T) {
	if got, ok := BoundedText(nil, metadatapolicy.TitleLimits()); !ok || got != "" {
		t.Fatalf("missing field = %q, %v", got, ok)
	}
	value := " <b>Title</b>  one "
	if got, ok := BoundedText(&value, metadatapolicy.TitleLimits()); !ok || got != "Title one" {
		t.Fatalf("BoundedText = %q, %v", got, ok)
	}
	long := strings.Repeat("x", metadatapolicy.MaxStudioTextBytes+1)
	if _, ok := BoundedText(&long, metadatapolicy.StudioLimits()); ok {
		t.Fatal("over-limit text accepted")
	}
}

func TestValidSearchQuery(t *testing.T) {
	for value, want := range map[string]bool{
		"作品":        true,
		"":          false,
		"bad\x00":   false,
		"line\ttab": true,
		strings.Repeat("x", metadatapolicy.MaxTitleTextBytes+1): false,
		strings.Repeat("字", metadatapolicy.MaxTitleTextRunes+1): false,
		string([]byte{0xff}): false,
	} {
		if got := ValidSearchQuery(value); got != want {
			t.Fatalf("ValidSearchQuery(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestParseID(t *testing.T) {
	for value, want := range map[string]bool{
		"1": true, "2147483647": true, "2147483648": false, "0": false, "012": false,
		"": false, "12a": false, "-1": false, "12345678901": false,
	} {
		id, numeric, ok := ParseID(value)
		if ok != want || ok && (id != value || numeric <= 0) {
			t.Fatalf("ParseID(%q) = %q, %d, %v", value, id, numeric, ok)
		}
	}
	if _, _, ok := ParseJSONID(json.RawMessage(nil)); ok {
		t.Fatal("empty JSON ID accepted")
	}
	if id, numeric, ok := ParseJSONID(json.RawMessage(`42`)); !ok || id != "42" || numeric != 42 {
		t.Fatalf("ParseJSONID = %q, %d, %v", id, numeric, ok)
	}
	if _, _, ok := ParseJSONID(json.RawMessage(`"42"`)); ok {
		t.Fatal("quoted JSON ID accepted")
	}
}

func TestValidCoverURL(t *testing.T) {
	host := metadatapolicy.AniListCoverHost
	for value, want := range map[string]bool{
		"https://s4.anilist.co/file/cover.jpg": true,
		"https://S4.ANILIST.CO/file/cover.jpg": true,
		"https://lain.bgm.tv/pic/cover.jpg":    false,
		"http://s4.anilist.co/file/cover.jpg":  false,
		"https://s4.anilist.co:443/cover.jpg":  false,
		"":                                     false,
	} {
		if got := ValidCoverURL(value, host); got != want {
			t.Fatalf("ValidCoverURL(%q) = %v, want %v", value, got, want)
		}
	}
}
