// SPDX-License-Identifier: MPL-2.0

package uitext

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var languages = map[string]*Strings{"zh-TW": &traditionalChinese, "en": &english, "ja": &japanese}

// texts lists every text of table by field name; array elements get their index.
func texts(table *Strings) map[string]string {
	all := map[string]string{}
	value := reflect.ValueOf(table).Elem()
	for index := range value.NumField() {
		field, name := value.Field(index), value.Type().Field(index).Name
		switch field.Kind() {
		case reflect.String:
			all[name] = field.String()
		case reflect.Array:
			for element := range field.Len() {
				all[fmt.Sprintf("%s[%d]", name, element)] = field.Index(element).String()
			}
		default:
			panic("unexpected field kind " + field.Kind().String())
		}
	}
	return all
}

func TestEveryTextHasEveryLanguage(t *testing.T) {
	for code, table := range languages {
		for name, text := range texts(table) {
			if text == "" {
				t.Errorf("%s: %s is empty", code, name)
			}
		}
	}
	if len(LanguageNames) != len(LanguageCodes) {
		t.Fatal("LanguageNames and LanguageCodes differ in length")
	}
	for _, code := range LanguageCodes {
		if languages[code] == nil || For(code) != languages[code] {
			t.Errorf("For(%q) is not that language", code)
		}
	}
}

var verbPattern = regexp.MustCompile(`%(\[\d+\])?[a-z]`)

// TestFormatsTakeTheSameArguments formats every language with the arguments the Traditional
// Chinese text takes; each must use all of them and no other.
func TestFormatsTakeTheSameArguments(t *testing.T) {
	for name, format := range texts(&traditionalChinese) {
		var args []any
		var values []string
		for index, verb := range verbPattern.FindAllString(format, -1) {
			switch verb {
			case "%d":
				args = append(args, 7001+index)
				values = append(values, fmt.Sprint(7001+index))
			case "%s":
				args = append(args, fmt.Sprintf("arg%d", index))
				values = append(values, fmt.Sprintf("arg%d", index))
			default:
				t.Fatalf("%s uses %s; the Chinese text must use plain %%d and %%s", name, verb)
			}
		}
		for code, table := range languages {
			text := texts(table)[name]
			if len(args) == 0 {
				if verbPattern.MatchString(text) {
					t.Errorf("%s: %s has a %% verb but is not a format", code, name)
				}
				continue
			}
			out := fmt.Sprintf(text, args...)
			if verbPattern.MatchString(out) || strings.Contains(out, "%!") {
				t.Errorf("%s: %s formats badly: %q", code, name, out)
			}
			for _, value := range values {
				if !strings.Contains(out, value) {
					t.Errorf("%s: %s drops an argument: %q", code, name, out)
				}
			}
		}
	}
}

// TestChineseHasNoSimplifiedCharacters checks every Traditional Chinese text against OpenCC's
// Simplified-to-Traditional character table, the one the Bangumi adapter converts with.
func TestChineseHasNoSimplifiedCharacters(t *testing.T) {
	file, err := os.Open("../../../adapters/metadata/bangumi/opencc/STCharacters.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	simplified := map[rune]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, values, ok := strings.Cut(scanner.Text(), "\t")
		runes := []rune(key)
		if !ok || len(runes) != 1 {
			continue
		}
		// A character that also maps to itself is valid Traditional Chinese.
		if !slices.Contains(strings.Fields(values), key) {
			simplified[runes[0]] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(simplified) < 1000 {
		t.Fatalf("only %d Simplified characters read; the table is wrong", len(simplified))
	}
	all := texts(&traditionalChinese)
	all["LanguageNames[0]"] = LanguageNames[0]
	for name, text := range all {
		for _, character := range text {
			if simplified[character] {
				t.Errorf("%s has the Simplified character %q: %q", name, character, text)
			}
		}
	}
}
