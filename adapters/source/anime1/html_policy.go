package anime1

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	"golang.org/x/net/html"
)

func rejectDuplicateRawAttributes(ctx context.Context, body []byte, attributes map[string][]string, malformed error) error {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		switch tokenizer.Next() {
		case html.ErrorToken:
			if errors.Is(tokenizer.Err(), io.EOF) {
				return nil
			}
			return malformed
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := tokenizer.TagName()
			for _, attribute := range attributes[strings.ToLower(string(name))] {
				if duplicateRawAttribute(tokenizer.Raw(), attribute) {
					return malformed
				}
			}
		}
	}
}

func duplicateRawAttribute(raw []byte, target string) bool {
	index := bytes.IndexAny(raw, " \t\r\n")
	seen := false
	for index >= 0 && index < len(raw) {
		for index < len(raw) && strings.ContainsRune(" \t\r\n/", rune(raw[index])) {
			index++
		}
		if index >= len(raw) || raw[index] == '>' {
			return false
		}
		start := index
		for index < len(raw) && !strings.ContainsRune(" \t\r\n=/>", rune(raw[index])) {
			index++
		}
		if strings.EqualFold(string(raw[start:index]), target) {
			if seen {
				return true
			}
			seen = true
		}
		for index < len(raw) && strings.ContainsRune(" \t\r\n", rune(raw[index])) {
			index++
		}
		if index >= len(raw) || raw[index] != '=' {
			continue
		}
		index++
		for index < len(raw) && strings.ContainsRune(" \t\r\n", rune(raw[index])) {
			index++
		}
		if index < len(raw) && (raw[index] == '\'' || raw[index] == '"') {
			quote := raw[index]
			index++
			for index < len(raw) && raw[index] != quote {
				index++
			}
			if index < len(raw) {
				index++
			}
			continue
		}
		for index < len(raw) && !strings.ContainsRune(" \t\r\n>", rune(raw[index])) {
			index++
		}
	}
	return false
}
