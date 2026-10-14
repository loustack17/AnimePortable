// SPDX-License-Identifier: MPL-2.0

package bangumi

import (
	"bufio"
	"embed"
	"strings"
	"sync"
	"unicode/utf8"
)

// The dictionaries are OpenCC's (Apache-2.0, see opencc/LICENSE). The chain follows OpenCC's
// s2twp configuration: Simplified to Traditional, then Taiwan phrases, then Taiwan variants.
// Unlike OpenCC, later steps match across the whole text rather than within first-step segments.
//
//go:embed opencc/*.txt
var openccFiles embed.FS

type conversionDictionary struct {
	entries   map[string]string
	maxLength int // longest key, in runes
}

var traditionalChain = sync.OnceValues(func() ([]conversionDictionary, error) {
	var chain []conversionDictionary
	for _, files := range [][]string{{"STPhrases.txt", "STCharacters.txt"}, {"TWPhrases.txt"}, {"TWVariants.txt"}} {
		dictionary := conversionDictionary{entries: make(map[string]string)}
		for _, name := range files {
			data, err := openccFiles.ReadFile("opencc/" + name)
			if err != nil {
				return nil, err
			}
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				key, values, ok := strings.Cut(scanner.Text(), "\t")
				if !ok || key == "" {
					continue
				}
				// Earlier files win, so phrases take priority over single characters.
				if _, exists := dictionary.entries[key]; !exists {
					dictionary.entries[key], _, _ = strings.Cut(values, " ")
					dictionary.maxLength = max(dictionary.maxLength, utf8.RuneCountInString(key))
				}
			}
		}
		chain = append(chain, dictionary)
	}
	return chain, nil
})

// toTraditional converts Simplified Chinese to Traditional Chinese with Taiwan wording.
// Bangumi's Chinese text is Simplified and the app shows only Traditional Chinese.
func toTraditional(value string) (string, bool) {
	chain, err := traditionalChain()
	if err != nil {
		return "", false
	}
	for _, dictionary := range chain {
		value = dictionary.convert(value)
	}
	return value, true
}

// convert replaces the longest dictionary key at each position, as OpenCC does.
func (dictionary conversionDictionary) convert(value string) string {
	runes := []rune(value)
	var result strings.Builder
	result.Grow(len(value))
	for start := 0; start < len(runes); {
		matched := false
		for length := min(dictionary.maxLength, len(runes)-start); length > 0; length-- {
			if replacement, ok := dictionary.entries[string(runes[start:start+length])]; ok {
				result.WriteString(replacement)
				start += length
				matched = true
				break
			}
		}
		if !matched {
			result.WriteRune(runes[start])
			start++
		}
	}
	return result.String()
}
