// SPDX-License-Identifier: MPL-2.0

package securehttp

import (
	"context"
	"errors"
	"mime"
	"strings"
)

// SafeError is the error an adapter may pass up after a failed request: cancellation and
// deadline errors, and this package's *Error (which carries no URL, header or body), go through
// unchanged; anything else is replaced by fallback, so no other error text leaves the adapter.
func SafeError(err, fallback error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	var secureError *Error
	if errors.As(err, &secureError) {
		return secureError
	}
	return fallback
}

// HasMediaType reports whether a Content-Type header value parses and names one of mediaTypes,
// compared case-insensitively; parameters such as charset are ignored.
func HasMediaType(contentType string, mediaTypes ...string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	for _, want := range mediaTypes {
		if strings.EqualFold(mediaType, want) {
			return true
		}
	}
	return false
}
