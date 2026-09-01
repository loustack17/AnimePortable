package internal

import (
	"reflect"
	"strings"
	"testing"
)

func TestDecodeObject(t *testing.T) {
	tests := []struct {
		name string
		body string
		ok   bool
	}{
		{name: "valid nested object", body: `{"data":{"items":[1,{"name":"ok"}]}}`, ok: true},
		{name: "duplicate root key", body: `{"data":1,"data":2}`, ok: false},
		{name: "duplicate nested key", body: `{"data":{"name":1,"name":2}}`, ok: false},
		{name: "top level array", body: `[]`, ok: false},
		{name: "trailing value", body: `{} {}`, ok: false},
		{name: "malformed", body: `{"data":`, ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var value map[string]any
			ok := DecodeObject([]byte(test.body), &value, JSONLimits{MaxBytes: 1024, MaxNestingDepth: 8})
			if ok != test.ok {
				t.Fatalf("ok = %v, want %v", ok, test.ok)
			}
		})
	}
}

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

func TestDecodeObjectRejectsInvalidTargets(t *testing.T) {
	var nilPointer *map[string]any
	tests := []any{nil, map[string]any{}, nilPointer}
	for _, target := range tests {
		if DecodeObject([]byte(`{"value":1}`), target, JSONLimits{MaxBytes: 1024, MaxNestingDepth: 8}) {
			t.Fatalf("invalid target accepted: %#v", target)
		}
	}
}

func TestNormalizePlainText(t *testing.T) {
	got, ok := NormalizePlainText("# **A synopsis** <br><p>[with a link](https://example.test)</p>", PlainTextLimits{MaxInputBytes: 1024, MaxOutputBytes: 1024, MaxOutputRunes: 1024})
	if !ok || got != "A synopsis with a link" {
		t.Fatalf("result = %q, %v", got, ok)
	}
	got, ok = NormalizePlainText("&amp;lt;script&amp;gt;alert(1)&amp;lt;/script&amp;gt;", PlainTextLimits{MaxInputBytes: 1024, MaxOutputBytes: 1024, MaxOutputRunes: 1024})
	if !ok || strings.Contains(got, "alert(1)") || strings.Contains(got, "<") {
		t.Fatalf("unsafe result = %q, %v", got, ok)
	}
}

func TestNormalizePlainTextEnforcesLimitsAndControls(t *testing.T) {
	limits := PlainTextLimits{MaxInputBytes: 8, MaxOutputBytes: 8, MaxOutputRunes: 8}
	if _, ok := NormalizePlainText("123456789", limits); ok {
		t.Fatal("oversized input accepted")
	}
	if _, ok := NormalizePlainText("123456789", PlainTextLimits{MaxInputBytes: 32, MaxOutputBytes: 8, MaxOutputRunes: 32}); ok {
		t.Fatal("oversized output accepted")
	}
	if _, ok := NormalizePlainText("123456789", PlainTextLimits{MaxInputBytes: 32, MaxOutputBytes: 32, MaxOutputRunes: 8}); ok {
		t.Fatal("oversized rune output accepted")
	}
	if _, ok := NormalizePlainText("a\x00b", PlainTextLimits{MaxInputBytes: 32, MaxOutputBytes: 32, MaxOutputRunes: 32}); ok {
		t.Fatal("control character accepted")
	}
}

func TestNormalizePlainTextAdversarialOpenDelimiters(t *testing.T) {
	value := strings.Repeat("<", 4096) + strings.Repeat("[", 4096)
	got, ok := NormalizePlainText(value, PlainTextLimits{MaxInputBytes: 16 * 1024, MaxOutputBytes: 16 * 1024, MaxOutputRunes: 16 * 1024})
	if !ok || got == "" {
		t.Fatalf("result = %q, %v", got, ok)
	}
}
