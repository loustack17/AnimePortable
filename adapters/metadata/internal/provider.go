// SPDX-License-Identifier: MPL-2.0

package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	metadatapolicy "animeportable/internal/metadata"
)

const maxMetadataIDDigits = 10

// Rules holds what a metadata provider passes to the shared answer checks: its limits and the
// two errors its callers see, so each provider keeps its own error values.
type Rules struct {
	MaxBodyBytes    int
	MaxNestingDepth int
	Unavailable     error
	Malformed       error
}

// CheckResponse turns the result of a request into an answer the provider may decode: request
// errors are reduced to safe ones, a 404 becomes core.ErrNotFound when notFoundIsMissing, and
// anything but a bounded 2xx JSON answer is refused.
func (rules Rules) CheckResponse(ctx context.Context, response *securehttp.Response, err error, notFoundIsMissing bool) (*securehttp.Response, error) {
	if err != nil {
		return nil, securehttp.SafeError(err, rules.Unavailable)
	}
	if response == nil || ctx == nil {
		return nil, rules.Unavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusNotFound && notFoundIsMissing {
		return nil, core.ErrNotFound
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, rules.Unavailable
	}
	if len(response.Body) > rules.MaxBodyBytes || !securehttp.HasMediaType(response.Header.Get("Content-Type"), "application/json") {
		return nil, rules.Malformed
	}
	return response, nil
}

// Decode decodes one JSON object answer into value, or returns the Malformed error.
func (rules Rules) Decode(body []byte, value any) error {
	if !DecodeObject(body, value, JSONLimits{MaxBytes: rules.MaxBodyBytes, MaxNestingDepth: rules.MaxNestingDepth}) {
		return rules.Malformed
	}
	return nil
}

// BoundedText normalizes an optional provider text field; a missing field is an empty text.
func BoundedText(value *string, limits metadatapolicy.PlainTextLimits) (string, bool) {
	if value == nil {
		return "", true
	}
	return metadatapolicy.NormalizePlainText(*value, limits)
}

// ValidSearchQuery reports whether value may be sent to a provider as a title search.
func ValidSearchQuery(value string) bool {
	return value != "" && len(value) <= metadatapolicy.MaxTitleTextBytes && utf8.RuneCountInString(value) <= metadatapolicy.MaxTitleTextRunes && utf8.ValidString(value) && !metadatapolicy.ContainsDisallowedControl(value)
}

// ParseID accepts a provider ID written as a positive decimal int32 without leading zeros and
// returns it with its numeric value.
func ParseID(value string) (string, int, bool) {
	if len(value) == 0 || len(value) > maxMetadataIDDigits || value[0] == '0' {
		return "", 0, false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return "", 0, false
		}
	}
	numeric, err := strconv.ParseInt(value, 10, 32)
	if err != nil || numeric <= 0 {
		return "", 0, false
	}
	return value, int(numeric), true
}

// ParseJSONID is ParseID for an ID given as a raw JSON number.
func ParseJSONID(value json.RawMessage) (string, int, bool) {
	if len(value) == 0 {
		return "", 0, false
	}
	return ParseID(string(value))
}

// ValidCoverURL reports whether value is a safe cover URL on host.
func ValidCoverURL(value, host string) bool {
	if value == "" || !metadatapolicy.IsSafeCoverURL(value) {
		return false
	}
	target, err := url.Parse(value)
	if err != nil {
		return false
	}
	return strings.EqualFold(target.Hostname(), host)
}
