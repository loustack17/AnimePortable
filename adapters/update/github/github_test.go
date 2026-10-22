// SPDX-License-Identifier: MPL-2.0

package github

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"animeportable/adapters/network/securehttp"
)

type fakeDo struct {
	status  int
	body    string
	err     error
	request *http.Request
}

func (f *fakeDo) Do(request *http.Request) (*securehttp.Response, error) {
	f.request = request
	if f.err != nil {
		return nil, f.err
	}
	return &securehttp.Response{StatusCode: f.status, Body: []byte(f.body)}, nil
}

const releaseJSON = `[
 {"tag_name":"v0.2.0-alpha","draft":false,"prerelease":true,"html_url":"https://github.com/loustack17/AnimePortable/releases/tag/v0.2.0-alpha","body":"notes"},
 {"tag_name":"v0.9.0-alpha","draft":true,"prerelease":true,"html_url":"https://github.com/loustack17/AnimePortable/releases/tag/v0.9.0-alpha"},
 {"tag_name":"v0.3.0-alpha","draft":false,"prerelease":true,"html_url":"https://github.com/loustack17/AnimePortable/releases/tag/v0.3.0-alpha"},
 {"tag_name":"nightly","draft":false,"prerelease":false,"html_url":"https://github.com/loustack17/AnimePortable/releases/tag/nightly"},
 {"tag_name":"v0.1.0-alpha.2","draft":false,"prerelease":true,"html_url":"https://github.com/loustack17/AnimePortable/releases/tag/v0.1.0-alpha.2"}
]`

func TestNewestPicksHighestVersionAndIgnoresDraftsAndOddTags(t *testing.T) {
	do := &fakeDo{status: http.StatusOK, body: releaseJSON}
	newest, found, err := newWithDo(do, "0.2.0-alpha").Newest(context.Background())
	if err != nil || !found || newest.Tag != "v0.3.0-alpha" || newest.URL != "https://github.com/loustack17/AnimePortable/releases/tag/v0.3.0-alpha" {
		t.Fatal(newest, found, err)
	}
	if do.request.URL.String() != "https://api.github.com/repos/loustack17/AnimePortable/releases?per_page=20" || do.request.Method != http.MethodGet {
		t.Fatal(do.request.URL)
	}
	if do.request.Header.Get("User-Agent") != "AnimePortable/0.2.0-alpha" || do.request.Header.Get("Accept") != "application/vnd.github+json" {
		t.Fatal(do.request.Header)
	}
}

func TestNewestOrdersByVersionNotPosition(t *testing.T) {
	cases := map[string]string{
		`[{"tag_name":"v0.2.0-alpha"},{"tag_name":"v0.2.1-alpha"}]`:     "v0.2.1-alpha",
		`[{"tag_name":"v0.2.1-alpha"},{"tag_name":"v0.2.0-alpha"}]`:     "v0.2.1-alpha",
		`[{"tag_name":"v0.1.0-alpha.1"},{"tag_name":"v0.1.0-alpha.2"}]`: "v0.1.0-alpha.2",
		`[{"tag_name":"v0.2.0-alpha"},{"tag_name":"v0.2.0-beta"}]`:      "v0.2.0-beta",
	}
	for body, want := range cases {
		newest, found, err := newWithDo(&fakeDo{status: http.StatusOK, body: body}, "0.2.0-alpha").Newest(context.Background())
		if err != nil || !found || newest.Tag != want {
			t.Errorf("%s: got %v %v %v, want %s", body, newest, found, err, want)
		}
	}
}

func TestNewestKeepsOnlyReleasePagesOfThisRepository(t *testing.T) {
	for _, link := range []string{
		"http://github.com/loustack17/AnimePortable/releases/tag/v0.3.0-alpha",
		"https://github.com/someone/else/releases/tag/v0.3.0-alpha",
		"https://github.com.evil.example/loustack17/AnimePortable/releases/tag/v0.3.0-alpha",
		"https://evil.example/loustack17/AnimePortable/releases/tag/v0.3.0-alpha",
		"https://user@github.com/loustack17/AnimePortable/releases/tag/v0.3.0-alpha",
		"https://github.com/loustack17/AnimePortable/releases/../../evil",
		"https://github.com/loustack17/AnimePortable/issues/1",
		"javascript:alert(1)",
		"file:///C:/Windows/System32/calc.exe",
		"",
	} {
		body := `[{"tag_name":"v0.3.0-alpha","html_url":"` + link + `"}]`
		newest, found, err := newWithDo(&fakeDo{status: http.StatusOK, body: body}, "0.2.0-alpha").Newest(context.Background())
		if err != nil || !found || newest.URL != "" {
			t.Errorf("%q: url %q found %v err %v", link, newest.URL, found, err)
		}
	}
}

func TestNewestNothingFoundAndFailures(t *testing.T) {
	newest, found, err := newWithDo(&fakeDo{status: http.StatusOK, body: `[{"tag_name":"v0.9.0-alpha","draft":true}]`}, "0").Newest(context.Background())
	if err != nil || found || newest != (Release{}) {
		t.Fatal(newest, found, err)
	}
	failure := errors.New("network down")
	if _, _, err := newWithDo(&fakeDo{err: failure}, "0").Newest(context.Background()); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	for name, do := range map[string]*fakeDo{
		"status":    {status: http.StatusForbidden, body: `{"message":"rate limited"}`},
		"not json":  {status: http.StatusOK, body: `<html>`},
		"not array": {status: http.StatusOK, body: `{"tag_name":"v1.0.0"}`},
		"truncated": {status: http.StatusOK, body: strings.Repeat("[", 10)},
	} {
		if _, _, err := newWithDo(do, "0").Newest(context.Background()); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestAllowedOriginsAreOnlyTheGitHubAPI(t *testing.T) {
	if origins := AllowedOrigins(); len(origins) != 1 || origins[0] != "https://api.github.com" {
		t.Fatal(origins)
	}
	if _, err := securehttp.New(securehttp.Config{AllowedOrigins: AllowedOrigins(), MaxResponseBytes: MaxResponseBytes}); err != nil {
		t.Fatal(err)
	}
}
