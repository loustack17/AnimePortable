// SPDX-License-Identifier: MPL-2.0

// Package jsonscan checks the token structure of untrusted JSON before it is decoded, because
// encoding/json accepts duplicate object keys (the last one wins) and has no nesting limit.
package jsonscan

import (
	"encoding/json"
	"errors"
	"io"
)

var errRejected = errors.New("jsonscan: rejected")

// Object reports whether the decoder's whole input is one JSON object with no duplicate key in
// any object and no value nested deeper than maxDepth. The caller configures the decoder, for
// example with UseNumber so that an out-of-range number is not rejected.
func Object(decoder *json.Decoder, maxDepth int) bool {
	if err := scanValue(decoder, true, 0, maxDepth); err != nil {
		return false
	}
	_, err := decoder.Token()
	return errors.Is(err, io.EOF)
}

func scanValue(decoder *json.Decoder, requireObject bool, depth, maxDepth int) error {
	if depth > maxDepth {
		return errRejected
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if requireObject && (!isDelimiter || delimiter != '{') {
		return errRejected
	}
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errRejected
			}
			if _, exists := seen[key]; exists {
				return errRejected
			}
			seen[key] = struct{}{}
			if err := scanValue(decoder, false, depth+1, maxDepth); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errRejected
		}
	case '[':
		for decoder.More() {
			if err := scanValue(decoder, false, depth+1, maxDepth); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errRejected
		}
	default:
		return errRejected
	}
	return nil
}
