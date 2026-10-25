// SPDX-License-Identifier: MPL-2.0

package securehttp

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestSafeErrorKeepsOnlySafeErrors(t *testing.T) {
	fallback := errors.New("adapter unavailable")
	secure := &Error{Kind: KindStatus, Status: 503}
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "canceled", err: fmt.Errorf("wrapped: %w", context.Canceled), want: context.Canceled},
		{name: "deadline", err: fmt.Errorf("wrapped: %w", context.DeadlineExceeded), want: context.DeadlineExceeded},
		{name: "secure error", err: fmt.Errorf("wrapped: %w", secure), want: secure},
		{name: "other error", err: errors.New("dial tcp 203.0.113.7:443: secret detail"), want: fallback},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SafeError(test.err, fallback); got != test.want {
				t.Fatalf("SafeError(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}

func TestHasMediaType(t *testing.T) {
	tests := []struct {
		contentType string
		mediaTypes  []string
		want        bool
	}{
		{contentType: "application/json", mediaTypes: []string{"application/json"}, want: true},
		{contentType: "Application/JSON; charset=utf-8", mediaTypes: []string{"application/json"}, want: true},
		{contentType: "application/xhtml+xml", mediaTypes: []string{"text/html", "application/xhtml+xml"}, want: true},
		{contentType: "text/plain", mediaTypes: []string{"application/json"}, want: false},
		{contentType: "", mediaTypes: []string{"application/json"}, want: false},
		{contentType: "application/json; =broken", mediaTypes: []string{"application/json"}, want: false},
		{contentType: "application/json", mediaTypes: nil, want: false},
	}
	for _, test := range tests {
		if got := HasMediaType(test.contentType, test.mediaTypes...); got != test.want {
			t.Fatalf("HasMediaType(%q, %q) = %v, want %v", test.contentType, test.mediaTypes, got, test.want)
		}
	}
}
