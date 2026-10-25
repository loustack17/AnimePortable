// SPDX-License-Identifier: MPL-2.0

package jsonscan

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestObject(t *testing.T) {
	tests := []struct {
		name string
		body string
		ok   bool
	}{
		{name: "nested object", body: `{"data":{"items":[1,{"name":"ok"}]}}`, ok: true},
		{name: "empty object", body: `{}`, ok: true},
		{name: "duplicate root key", body: `{"a":1,"a":2}`, ok: false},
		{name: "duplicate nested key", body: `{"a":{"b":1,"b":2}}`, ok: false},
		{name: "duplicate key inside array", body: `{"a":[{"b":1,"b":2}]}`, ok: false},
		{name: "top level array", body: `[]`, ok: false},
		{name: "top level scalar", body: `1`, ok: false},
		{name: "trailing value", body: `{} {}`, ok: false},
		{name: "unclosed", body: `{"a":`, ok: false},
		{name: "empty", body: ``, ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Object(json.NewDecoder(strings.NewReader(test.body)), 8); got != test.ok {
				t.Fatalf("Object(%q) = %v, want %v", test.body, got, test.ok)
			}
		})
	}
}

func TestObjectNestingLimitIsInclusive(t *testing.T) {
	nested := func(depth int) string {
		return `{"a":` + strings.Repeat("[", depth) + strings.Repeat("]", depth) + `}`
	}
	if !Object(json.NewDecoder(strings.NewReader(nested(3))), 3) {
		t.Fatal("value at the depth limit rejected")
	}
	if Object(json.NewDecoder(strings.NewReader(nested(4))), 3) {
		t.Fatal("value past the depth limit accepted")
	}
}

func TestObjectLeavesNumberHandlingToTheDecoder(t *testing.T) {
	body := []byte(`{"n":1e400}`)
	if Object(json.NewDecoder(bytes.NewReader(body)), 8) {
		t.Fatal("default decoder accepted an out-of-range number")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if !Object(decoder, 8) {
		t.Fatal("UseNumber decoder rejected an out-of-range number")
	}
}
