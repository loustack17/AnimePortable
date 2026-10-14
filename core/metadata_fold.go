// SPDX-License-Identifier: MPL-2.0

package core

import (
	_ "embed"
	"strings"
	"sync"
	"unicode/utf8"
)

// tsCharacters is OpenCC's Traditional-to-Simplified character table (Apache-2.0, see
// opencc/LICENSE). Titles are compared in one script so that Taiwan names, mainland names and
// variant characters (迴/回, 藥/药) fold together.
//
//go:embed opencc/TSCharacters.txt
var tsCharacters string

var simplifiedCharacters = sync.OnceValue(func() map[rune]rune {
	table := make(map[rune]rune)
	for _, line := range strings.Split(tsCharacters, "\n") {
		traditional, simplified, ok := strings.Cut(strings.TrimRight(line, "\r"), "\t")
		simplified, _, _ = strings.Cut(simplified, " ")
		if !ok || utf8.RuneCountInString(traditional) != 1 || utf8.RuneCountInString(simplified) != 1 {
			continue
		}
		from, _ := utf8.DecodeRuneInString(traditional)
		to, _ := utf8.DecodeRuneInString(simplified)
		table[from] = to
	}
	return table
})

// foldToSimplified maps every Traditional character to its Simplified form for comparison only.
func foldToSimplified(value string) string {
	table := simplifiedCharacters()
	return strings.Map(func(character rune) rune {
		if simplified, ok := table[character]; ok {
			return simplified
		}
		return character
	}, value)
}
