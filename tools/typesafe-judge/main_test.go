// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func f(v float64) *float64 { return &v }

func TestNeedsReview(t *testing.T) {
	cases := []struct {
		name string
		a    answer
		want bool
	}{
		{"confident noul yes", answer{Noul: f(0.95)}, false},
		{"confident noul no", answer{Noul: f(0.05)}, false},
		{"uncertain noul", answer{Noul: f(0.5)}, true},
		{"confident score", answer{Score: f(1.9), Confidence: f(0.9)}, false},
		{"spread choice", answer{Choice: "a", Confidence: f(0.3)}, true},
	}
	for _, c := range cases {
		if got := needsReview(c.a); got != c.want {
			t.Errorf("%s: needsReview = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestBatchValidateRejectsDuplicateAndEmpty(t *testing.T) {
	q := map[string]json.RawMessage{"q": json.RawMessage(`{"type":"noul","instructions":"x"}`)}
	dup := batch{Questions: q, Items: []item{{ID: "a", State: json.RawMessage(`"s"`)}, {ID: "a", State: json.RawMessage(`"s"`)}}}
	if err := dup.validate(); err == nil {
		t.Fatal("duplicate ids accepted")
	}
	noState := batch{Questions: q, Items: []item{{ID: "a"}}}
	if err := noState.validate(); err == nil {
		t.Fatal("missing state accepted")
	}
	ok := batch{Questions: q, Items: []item{{ID: "a", State: json.RawMessage(`"s"`)}}}
	if err := ok.validate(); err != nil || ok.Model != "jev-latest" {
		t.Fatalf("valid batch: err=%v model=%q", err, ok.Model)
	}
}

func TestJudgeRetriesOverloadAndReportsValidationErrors(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("missing bearer key")
		}
		var body map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		if string(body["state"]) == `"bad"` {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"detail":"criteria"}`))
			return
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(529)
			return
		}
		_, _ = w.Write([]byte(`{"model":"jev-x","answers":{"q":{"type":"noul","noul":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer srv.Close()
	c := &client{endpoint: srv.URL, key: "k", http: srv.Client(), backoff: func(int) time.Duration { return 0 }}
	q := map[string]json.RawMessage{"q": json.RawMessage(`{"type":"noul","instructions":"x"}`)}

	ok := c.judge("m", q, item{ID: "good", State: json.RawMessage(`"fine"`)})
	if ok.Error != "" || ok.Answers["q"].Noul == nil || *ok.Answers["q"].Noul != 0.9 {
		t.Fatalf("expected answer after one 529 retry, got %+v", ok)
	}
	bad := c.judge("m", q, item{ID: "bad", State: json.RawMessage(`"bad"`)})
	if !strings.Contains(bad.Error, "HTTP 422") {
		t.Fatalf("expected surfaced 422, got %+v", bad)
	}
}

func TestJudgeRejectsIncompleteAnswersAndDoesNotResendAfterTransportError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"model":"jev-x","answers":{"a":{"type":"choice","choice":"x"}}}`))
	}))
	c := &client{endpoint: srv.URL, key: "k", http: srv.Client(), backoff: func(int) time.Duration { return 0 }}
	q := map[string]json.RawMessage{"a": json.RawMessage(`{}`), "b": json.RawMessage(`{}`)}
	r := c.judge("m", q, item{ID: "x", State: json.RawMessage(`"s"`)})
	if r.Error == "" {
		t.Fatal("missing answer or confidence accepted")
	}
	srv.Close()
	calls.Store(0)
	r = c.judge("m", q, item{ID: "y", State: json.RawMessage(`"s"`)})
	if !strings.Contains(r.Error, "request failed") || calls.Load() != 0 {
		t.Fatalf("transport error not surfaced once: %+v calls=%d", r, calls.Load())
	}
}

func TestCheckAnswers(t *testing.T) {
	q := map[string]json.RawMessage{"n": nil, "s": nil}
	ok := map[string]answer{"n": {Type: "noul", Noul: f(0.9)}, "s": {Type: "score", Score: f(1), Confidence: f(0.8)}}
	if err := checkAnswers(q, ok); err != nil {
		t.Fatalf("complete answers rejected: %v", err)
	}
	for name, bad := range map[string]map[string]answer{
		"missing question": {"n": ok["n"]},
		"null noul":        {"n": {Type: "noul"}, "s": ok["s"]},
		"no confidence":    {"n": ok["n"], "s": {Type: "score", Score: f(1)}},
		"no score value":   {"n": ok["n"], "s": {Type: "score", Confidence: f(0.9)}},
		"empty choice":     {"n": ok["n"], "s": {Type: "choice", Confidence: f(0.9)}},
		"unknown type":     {"n": ok["n"], "s": {Type: "rank", Confidence: f(0.9)}},
	} {
		if checkAnswers(q, bad) == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if !needsReview(answer{Type: "choice", Choice: "a"}) {
		t.Error("answer without confidence not flagged")
	}
}

func TestRunRequiresKeyAndWritesFailures(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.json")
	out := filepath.Join(dir, "out.json")
	_ = os.WriteFile(in, []byte(`{"questions":{"q":{"type":"noul","instructions":"x"}},"items":[{"id":"a","state":"s"}]}`), 0o600)

	t.Setenv(apiKeyEnv, "")
	if err := run(in, "", out, "http://127.0.0.1:0", &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), apiKeyEnv) {
		t.Fatalf("missing key not reported: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	t.Setenv(apiKeyEnv, "k")
	var stdout bytes.Buffer
	if err := run(in, "", out, srv.URL, &stdout); err == nil {
		t.Fatal("API failure was hidden")
	}
	if !strings.Contains(stdout.String(), "a ERROR HTTP 401") {
		t.Fatalf("summary missing error: %q", stdout.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("raw results not written: %v", err)
	}
}
