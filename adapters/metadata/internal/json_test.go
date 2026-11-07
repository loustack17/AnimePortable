// SPDX-License-Identifier: MPL-2.0

package internal

import (
	"reflect"
	"testing"
)

func TestDecodeObjectEnforcesLimits(t *testing.T) {
	var value map[string]any
	if DecodeObject([]byte(`{"value":1}`), &value, JSONLimits{MaxBytes: 1, MaxNestingDepth: 8}) {
		t.Fatal("oversized body accepted")
	}
	if DecodeObject([]byte(`{"a":{"b":1}}`), &value, JSONLimits{MaxBytes: 1024, MaxNestingDepth: 1}) {
		t.Fatal("excess nesting accepted")
	}
	if !DecodeObject([]byte(`{"a":{"b":1}}`), &value, JSONLimits{MaxBytes: 1024, MaxNestingDepth: 2}) {
		t.Fatal("boundary nesting rejected")
	}
	if DecodeObject([]byte(`{"value":1}`), &value, JSONLimits{MaxBytes: 1024, MaxNestingDepth: -1}) {
		t.Fatal("invalid nesting limit accepted")
	}
}

func TestDecodeObjectDoesNotPartiallyDecode(t *testing.T) {
	value := map[string]any{"existing": "value"}
	if DecodeObject([]byte(`{"new":1,"new":2}`), &value, JSONLimits{MaxBytes: 1024, MaxNestingDepth: 8}) {
		t.Fatal("duplicate object accepted")
	}
	if !reflect.DeepEqual(value, map[string]any{"existing": "value"}) {
		t.Fatalf("target mutated on rejected input: %#v", value)
	}
}

func TestDecodeObjectDoesNotPartiallyDecodeTypeMismatch(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	target := payload{Name: "existing", Count: 7}
	if DecodeObject([]byte(`{"name":"updated","count":"invalid"}`), &target, JSONLimits{MaxBytes: 1024, MaxNestingDepth: 8}) {
		t.Fatal("type mismatch accepted")
	}
	if target != (payload{Name: "existing", Count: 7}) {
		t.Fatalf("target mutated on rejected type mismatch: %#v", target)
	}
}
