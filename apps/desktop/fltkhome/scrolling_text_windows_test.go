//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"reflect"
	"testing"
	"unicode/utf8"
)

func TestWrapLinesBreaksTextWithoutSpacesAndAtWordBoundaries(t *testing.T) {
	// Each character is one unit wide.
	measure := func(text string) int { return utf8.RuneCountInString(text) }
	for _, test := range []struct {
		text string
		want []string
	}{
		{"尤爾是一名住在山中", []string{"尤爾是一", "名住在山", "中"}},
		{"one two three", []string{"one", "two", "thre", "e"}},
		{"ab\n\ncd", []string{"ab", "", "cd"}},
		{"abcd efg", []string{"abcd", "efg"}},
	} {
		if got := wrapLines(test.text, 4, measure); !reflect.DeepEqual(got, test.want) {
			t.Errorf("wrapLines(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}
