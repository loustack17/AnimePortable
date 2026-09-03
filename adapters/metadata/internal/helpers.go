// SPDX-License-Identifier: MPL-2.0

package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
)

type JSONLimits struct {
	MaxBytes        int
	MaxNestingDepth int
}

func DecodeObject(body []byte, target any, limits JSONLimits) bool {
	if len(body) == 0 || len(body) > limits.MaxBytes || limits.MaxBytes <= 0 || limits.MaxNestingDepth < 0 {
		return false
	}
	targetValue := reflect.ValueOf(target)
	if !targetValue.IsValid() || targetValue.Kind() != reflect.Pointer || targetValue.IsNil() || !targetValue.Elem().CanSet() {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := scanJSONValue(decoder, true, 0, limits.MaxNestingDepth); err != nil {
		return false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return false
	}
	decoded := reflect.New(targetValue.Elem().Type())
	if err := json.Unmarshal(body, decoded.Interface()); err != nil {
		return false
	}
	targetValue.Elem().Set(decoded.Elem())
	return true
}

func scanJSONValue(decoder *json.Decoder, requireObject bool, depth, maxDepth int) error {
	if depth > maxDepth {
		return errors.New("json nesting limit exceeded")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if requireObject && (!isDelimiter || delimiter != '{') {
		return errors.New("json object required")
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
				return errors.New("json object key required")
			}
			if _, exists := seen[key]; exists {
				return errors.New("duplicate json object key")
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder, false, depth+1, maxDepth); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("json object not closed")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder, false, depth+1, maxDepth); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("json array not closed")
		}
	default:
		return errors.New("unexpected json delimiter")
	}
	return nil
}
