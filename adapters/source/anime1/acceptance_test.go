package anime1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
)

const acceptanceSecret = "phase8-provider-secret"

type malformedAcceptanceCall struct {
	name string
	want error
	call func(*Client) (bool, error)
}

type malformedAcceptanceCase struct {
	name   string
	client func() *Client
	calls  []malformedAcceptanceCall
}

func TestAnime1AdapterAcceptanceMalformedResponses(t *testing.T) {
	cases := []malformedAcceptanceCase{
		{
			name: "catalog and search",
			client: func() *Client {
				return newWithDo(&fakeClient{response: jsonResponse([]byte(`[{"secret":"` + acceptanceSecret + `"}]`))})
			},
			calls: []malformedAcceptanceCall{
				{
					name: "catalog",
					want: errCatalogMalformed,
					call: func(client *Client) (bool, error) {
						items, err := client.Catalog(context.Background())
						return items == nil, err
					},
				},
				{
					name: "search",
					want: errCatalogMalformed,
					call: func(client *Client) (bool, error) {
						items, err := client.Search(context.Background(), "secret")
						return items == nil, err
					},
				},
			},
		},
		{
			name: "episodes",
			client: func() *Client {
				body := `<html><body><main id="main"><p>` + acceptanceSecret + `</p></main></body></html>`
				return newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(body)}})
			},
			calls: []malformedAcceptanceCall{
				{
					name: "episodes",
					want: errEpisodesMalformed,
					call: func(client *Client) (bool, error) {
						items, err := client.Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
						return items == nil, err
					},
				},
			},
		},
		{
			name: "resolve",
			client: func() *Client {
				body := `<html><body><main id="main"><article id="post-101"><video data-apireq="">` + acceptanceSecret + `</video></article></main></body></html>`
				return newWithDo(&fakeClient{resolverResponses: []*securehttp.Response{{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": {"text/html; charset=utf-8"}},
					Body:       []byte(body),
				}}})
			},
			calls: []malformedAcceptanceCall{
				{
					name: "resolve",
					want: errResolverMalformed,
					call: func(client *Client) (bool, error) {
						source, err := client.Resolve(context.Background(), resolverRef())
						return source.URL() == "" && len(source.Headers()) == 0, err
					},
				},
			},
		},
		{
			name: "schedule",
			client: func() *Client {
				body := `<html><body><div>` + acceptanceSecret + `</div></body></html>`
				return newWithDo(&fakeClient{scheduleResponses: []*securehttp.Response{scheduleResponse(body)}})
			},
			calls: []malformedAcceptanceCall{
				{
					name: "schedule",
					want: errScheduleMalformed,
					call: func(client *Client) (bool, error) {
						items, err := client.Schedule(context.Background(), core.ScheduleQuery{
							From: taipeiDate(2026, 1, 4),
							To:   taipeiDate(2026, 1, 11),
						})
						return items == nil, err
					},
				},
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			for _, call := range test.calls {
				t.Run(call.name, func(t *testing.T) {
					zero, err := call.call(test.client())
					if err == nil || !errors.Is(err, call.want) {
						t.Fatal("malformed response returned an unexpected error type")
					}
					if !zero {
						t.Fatal("malformed response returned a non-zero result")
					}
					assertAcceptanceErrorRedacted(t, err)
				})
			}
		})
	}
}

func assertAcceptanceErrorRedacted(t *testing.T, err error) {
	t.Helper()
	for _, format := range []string{"%v", "%+v", "%#v"} {
		formatted := fmt.Sprintf(format, err)
		if strings.Contains(formatted, acceptanceSecret) {
			t.Fatalf("error format %s leaked provider secret", format)
		}
	}
}

func TestAnime1AdapterAcceptancePublicComposition(t *testing.T) {
	client, err := securehttp.New(securehttp.Config{AllowedOrigins: AllowedOrigins()})
	if err != nil {
		t.Fatal(err)
	}
	clientSource := New(client)
	if clientSource == nil {
		t.Fatal("Anime1 source composition returned nil")
	}
}
