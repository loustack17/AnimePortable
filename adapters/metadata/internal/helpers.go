// SPDX-License-Identifier: MPL-2.0

package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	stdhtml "html"
	"io"
	"reflect"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
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

type PlainTextLimits struct {
	MaxInputBytes  int
	MaxOutputBytes int
	MaxOutputRunes int
}

func NormalizePlainText(value string, limits PlainTextLimits) (string, bool) {
	if limits.MaxInputBytes <= 0 || limits.MaxOutputBytes <= 0 || limits.MaxOutputRunes <= 0 || len(value) > limits.MaxInputBytes || !utf8.ValidString(value) {
		return "", false
	}
	plain := stripHTML(stdhtml.UnescapeString(value))
	plain = stripHTML(plain)
	plain = strings.NewReplacer("<", " ", ">", " ").Replace(plain)
	plain = strings.TrimSpace(stripMarkdown(plain))
	if len(plain) > limits.MaxOutputBytes || utf8.RuneCountInString(plain) > limits.MaxOutputRunes || containsDisallowedControl(plain) {
		return "", false
	}
	return strings.Join(strings.Fields(plain), " "), true
}

var markdownLinkPattern = regexp.MustCompile(`\[([^\[\]\r\n]*)\]\([^()\r\n]*\)`)

func stripHTML(value string) string {
	var result strings.Builder
	tokenizer := html.NewTokenizer(strings.NewReader(value))
	skippedElement := ""
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return result.String()
		case html.StartTagToken:
			name, _ := tokenizer.TagName()
			if skippedElement == "" && (string(name) == "script" || string(name) == "style") {
				skippedElement = string(name)
			}
			result.WriteByte(' ')
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == skippedElement {
				skippedElement = ""
			}
			result.WriteByte(' ')
		case html.TextToken:
			if skippedElement == "" {
				result.Write(tokenizer.Text())
			}
		}
	}
}

func stripMarkdown(value string) string {
	value = markdownLinkPattern.ReplaceAllString(value, "$1")
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		for len(trimmed) > 0 && trimmed[0] == '#' {
			trimmed = trimmed[1:]
		}
		if len(trimmed) > 0 && (trimmed[0] == '>' || trimmed[0] == '-' || trimmed[0] == '+' || trimmed[0] == '*') {
			trimmed = trimmed[1:]
		}
		lines[index] = trimmed
	}
	value = strings.Join(lines, " ")
	var result strings.Builder
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '*', '_', '~', '`':
		case '\\':
			if index+1 < len(value) && strings.ContainsRune("\\`*_{}[]()#+-.!~", rune(value[index+1])) {
				continue
			}
			result.WriteByte(value[index])
		default:
			result.WriteByte(value[index])
		}
	}
	return result.String()
}

func containsDisallowedControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return true
		}
	}
	return false
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
