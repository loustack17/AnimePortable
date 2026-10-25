// SPDX-License-Identifier: MPL-2.0

package internal

import (
	"bytes"
	"encoding/json"
	"reflect"

	"animeportable/adapters/internal/jsonscan"
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
	if !jsonscan.Object(json.NewDecoder(bytes.NewReader(body)), limits.MaxNestingDepth) {
		return false
	}
	decoded := reflect.New(targetValue.Elem().Type())
	if err := json.Unmarshal(body, decoded.Interface()); err != nil {
		return false
	}
	targetValue.Elem().Set(decoded.Elem())
	return true
}
