// SPDX-License-Identifier: MPL-2.0

// Command typesafe-judge sends development-time judgment questions to the
// TypeSafe System One API and saves the raw answers as evidence. It is a
// developer tool only; the product never calls TypeSafe.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

const (
	defaultEndpoint = "https://api.typesafe.ai/v1/systemone"
	apiKeyEnv       = "TYPESAFE_API_KEY"
	// Answers below this confidence, or Noul values in the uncertain band,
	// are flagged for independent adjudication instead of being acted on.
	lowConfidence = 0.5
	noulLow       = 0.3
	noulHigh      = 0.7
	maxAttempts   = 5
	workers       = 4
)

// batch is the input file: one question set applied to every item's state.
type batch struct {
	Model     string                     `json:"model"`
	Questions map[string]json.RawMessage `json:"questions"`
	Items     []item                     `json:"items"`
}

type item struct {
	ID    string          `json:"id"`
	State json.RawMessage `json:"state"`
}

type answer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
}

type result struct {
	ID      string            `json:"id"`
	Model   string            `json:"model,omitempty"`
	Answers map[string]answer `json:"answers,omitempty"`
	Usage   json.RawMessage   `json:"usage,omitempty"`
	Error   string            `json:"error,omitempty"`
}

type client struct {
	endpoint string
	key      string
	http     *http.Client
	backoff  func(attempt int) time.Duration
}

func main() {
	in := flag.String("in", "", "batch request JSON file")
	questions := flag.String("questions", "", "optional question template JSON file; replaces the batch's questions")
	out := flag.String("out", "", "file to write raw results JSON")
	endpoint := flag.String("endpoint", defaultEndpoint, "System One endpoint")
	flag.Parse()
	if err := run(*in, *questions, *out, *endpoint, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "typesafe-judge:", err)
		os.Exit(1)
	}
}

func run(inPath, questionsPath, outPath, endpoint string, stdout io.Writer) error {
	if inPath == "" || outPath == "" {
		return errors.New("-in and -out are required")
	}
	key := os.Getenv(apiKeyEnv)
	if key == "" {
		return fmt.Errorf("%s is not set; inject it from the secret manager for this process only", apiKeyEnv)
	}
	raw, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}
	var b batch
	if err := json.Unmarshal(raw, &b); err != nil {
		return fmt.Errorf("parse %s: %w", inPath, err)
	}
	if questionsPath != "" {
		raw, err := os.ReadFile(questionsPath)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &b.Questions); err != nil {
			return fmt.Errorf("parse %s: %w", questionsPath, err)
		}
	}
	if err := b.validate(); err != nil {
		return err
	}
	c := &client{endpoint: endpoint, key: key, http: &http.Client{Timeout: 2 * time.Minute},
		backoff: func(attempt int) time.Duration { return time.Duration(2<<attempt) * time.Second }}
	results := c.judgeAll(b)
	encoded, err := json.MarshalIndent(results, "", " ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, encoded, 0o644); err != nil {
		return err
	}
	failed := printSummary(stdout, results)
	if failed > 0 {
		return fmt.Errorf("%d of %d items failed; see %s", failed, len(results), outPath)
	}
	return nil
}

func (b *batch) validate() error {
	if b.Model == "" {
		b.Model = "jev-latest"
	}
	if len(b.Questions) == 0 {
		return errors.New("batch has no questions")
	}
	if len(b.Items) == 0 {
		return errors.New("batch has no items")
	}
	seen := map[string]bool{}
	for _, it := range b.Items {
		if it.ID == "" || seen[it.ID] {
			return fmt.Errorf("item id %q is empty or duplicated", it.ID)
		}
		if len(it.State) == 0 {
			return fmt.Errorf("item %q has no state", it.ID)
		}
		seen[it.ID] = true
	}
	return nil
}

func (c *client) judgeAll(b batch) []result {
	results := make([]result, len(b.Items))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] = c.judge(b.Model, b.Questions, b.Items[i])
			}
		}()
	}
	for i := range b.Items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}

func (c *client) judge(model string, questions map[string]json.RawMessage, it item) result {
	body, err := json.Marshal(map[string]any{"model": model, "questions": questions, "state": it.State})
	if err != nil {
		return result{ID: it.ID, Error: err.Error()}
	}
	var lastErr string
	for attempt := 0; attempt < maxAttempts; attempt++ {
		status, payload, err := c.post(body)
		switch {
		case err != nil:
			// A timed-out request may still have been processed and billed; do not resend it.
			return result{ID: it.ID, Error: "request failed: " + err.Error()}
		case status == http.StatusOK:
			var r result
			if err := json.Unmarshal(payload, &r); err != nil {
				return result{ID: it.ID, Error: "decode response: " + err.Error()}
			}
			if err := checkAnswers(questions, r.Answers); err != nil {
				return result{ID: it.ID, Model: r.Model, Answers: r.Answers, Error: err.Error()}
			}
			r.ID = it.ID
			return r
		case status == http.StatusTooManyRequests || status == 529:
			lastErr = fmt.Sprintf("HTTP %d", status)
		default:
			// Validation and auth errors do not improve on retry.
			return result{ID: it.ID, Error: fmt.Sprintf("HTTP %d: %s", status, truncate(payload, 300))}
		}
		if attempt < maxAttempts-1 {
			time.Sleep(c.backoff(attempt))
		}
	}
	return result{ID: it.ID, Error: "gave up after retries: " + lastErr}
}

func (c *client) post(body []byte) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return resp.StatusCode, payload, err
}

// checkAnswers rejects responses that omit a question or its decision value,
// so a partial answer is never mistaken for a confident one.
func checkAnswers(questions map[string]json.RawMessage, answers map[string]answer) error {
	for id := range questions {
		a, ok := answers[id]
		switch {
		case !ok:
			return fmt.Errorf("response has no answer for question %q", id)
		case a.Type == "noul" && a.Noul == nil:
			return fmt.Errorf("noul answer %q has no value", id)
		case (a.Type == "choice" || a.Type == "score") && a.Confidence == nil:
			return fmt.Errorf("%s answer %q has no confidence", a.Type, id)
		case a.Type == "choice" && a.Choice == "":
			return fmt.Errorf("choice answer %q has no choice", id)
		case a.Type == "score" && a.Score == nil:
			return fmt.Errorf("score answer %q has no score", id)
		case a.Type != "noul" && a.Type != "choice" && a.Type != "score":
			return fmt.Errorf("answer %q has unknown type %q", id, a.Type)
		}
	}
	return nil
}

// needsReview reports whether an answer is too uncertain to act on alone.
func needsReview(a answer) bool {
	if a.Noul != nil {
		return *a.Noul > noulLow && *a.Noul < noulHigh
	}
	return a.Confidence == nil || *a.Confidence < lowConfidence
}

func formatAnswer(a answer) string {
	switch {
	case a.Noul != nil:
		return fmt.Sprintf("noul=%.2f", *a.Noul)
	case a.Score != nil && a.Confidence != nil:
		return fmt.Sprintf("score=%.2f/%d conf=%.2f", *a.Score, len(a.Legend)-1, *a.Confidence)
	case a.Confidence != nil:
		return fmt.Sprintf("choice=%s conf=%.2f", a.Choice, *a.Confidence)
	}
	return a.Type
}

func printSummary(w io.Writer, results []result) (failed int) {
	for _, r := range results {
		if r.Error != "" {
			failed++
			fmt.Fprintf(w, "%s ERROR %s\n", r.ID, r.Error)
			continue
		}
		ids := make([]string, 0, len(r.Answers))
		for id := range r.Answers {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			flag := ""
			if needsReview(r.Answers[id]) {
				flag = " REVIEW"
			}
			fmt.Fprintf(w, "%s %s %s%s\n", r.ID, id, formatAnswer(r.Answers[id]), flag)
		}
	}
	return failed
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		b = b[:n]
	}
	return string(b)
}
