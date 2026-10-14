//go:build windows && amd64

// SPDX-License-Identifier: MPL-2.0

package mpvwin

import (
	"testing"
	"unsafe"
)

func cString(value string) *byte {
	return &append([]byte(value), 0)[0]
}

func TestParsePropertyChange(t *testing.T) {
	name := cString("time-pos")
	text := cString("d3d11va")
	textPointer := unsafe.Pointer(text)
	flag := int32(1)
	number := 12.5
	for _, test := range []struct {
		label string
		raw   rawPropertyChange
		want  PropertyChange
	}{
		{"string", rawPropertyChange{unsafe.Pointer(name), int32(FormatString), unsafe.Pointer(&textPointer)}, PropertyChange{Name: "time-pos", Format: FormatString, String: "d3d11va"}},
		{"double", rawPropertyChange{unsafe.Pointer(name), int32(FormatDouble), unsafe.Pointer(&number)}, PropertyChange{Name: "time-pos", Format: FormatDouble, Double: 12.5}},
		{"flag", rawPropertyChange{unsafe.Pointer(name), int32(FormatFlag), unsafe.Pointer(&flag)}, PropertyChange{Name: "time-pos", Format: FormatFlag, Flag: true}},
		{"unavailable", rawPropertyChange{unsafe.Pointer(name), int32(FormatNone), nil}, PropertyChange{Name: "time-pos", Format: FormatNone}},
		{"missing payload", rawPropertyChange{unsafe.Pointer(name), int32(FormatDouble), nil}, PropertyChange{Name: "time-pos", Format: FormatNone}},
		{"unsupported format", rawPropertyChange{unsafe.Pointer(name), int32(FormatInt64), unsafe.Pointer(&number)}, PropertyChange{Name: "time-pos", Format: FormatNone}},
	} {
		got := parsePropertyChange(unsafe.Pointer(&test.raw))
		if got == nil || *got != test.want {
			t.Fatalf("%s: got %+v, want %+v", test.label, got, test.want)
		}
	}
	if parsePropertyChange(nil) != nil {
		t.Fatal("nil payload must produce no change")
	}
}
