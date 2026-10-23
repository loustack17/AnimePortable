// SPDX-License-Identifier: MPL-2.0

// Package release holds the app's own version and compares release tags.
package release

import (
	"strconv"
	"strings"
)

// Version is this build's version without the leading "v"; it matches the release tag
// (docs/05_WORKFLOW.md section 11 bumps it with each release).
const Version = "0.4.1-beta"

// Parsed is a version of the form [v]MAJOR.MINOR.PATCH[-STAGE], where STAGE is dot-separated
// identifiers such as "alpha" or "alpha.1".
type Parsed struct {
	numbers [3]uint64
	stage   []string
}

// Parse reads a release tag or version. ok is false for anything else.
func Parse(value string) (Parsed, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	core, stage, hasStage := strings.Cut(value, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Parsed{}, false
	}
	var parsed Parsed
	for index, part := range parts {
		number, ok := parseNumber(part)
		if !ok {
			return Parsed{}, false
		}
		parsed.numbers[index] = number
	}
	if hasStage {
		parsed.stage = strings.Split(stage, ".")
		for _, identifier := range parsed.stage {
			if !validIdentifier(identifier) {
				return Parsed{}, false
			}
		}
	}
	return parsed, true
}

// String is the version without a leading "v", rebuilt from its parts, so what is shown is
// what was parsed and never the raw tag.
func (left Parsed) String() string {
	text := strconv.FormatUint(left.numbers[0], 10) + "." + strconv.FormatUint(left.numbers[1], 10) + "." + strconv.FormatUint(left.numbers[2], 10)
	if len(left.stage) > 0 {
		text += "-" + strings.Join(left.stage, ".")
	}
	return text
}

// Compare orders versions by number, then by stage as semantic versioning does: a version
// with a stage is older than the same numbers without one, and stages compare identifier by
// identifier (numbers numerically and below text, text alphabetically, a shorter list first).
// It returns -1, 0 or 1.
func (left Parsed) Compare(right Parsed) int {
	for index := range left.numbers {
		if left.numbers[index] != right.numbers[index] {
			return sign(left.numbers[index] > right.numbers[index])
		}
	}
	switch {
	case len(left.stage) == 0 && len(right.stage) == 0:
		return 0
	case len(left.stage) == 0:
		return 1
	case len(right.stage) == 0:
		return -1
	}
	for index := 0; index < len(left.stage) && index < len(right.stage); index++ {
		if result := compareIdentifier(left.stage[index], right.stage[index]); result != 0 {
			return result
		}
	}
	return compareInt(len(left.stage), len(right.stage))
}

func compareIdentifier(left, right string) int {
	leftNumber, leftIsNumber := parseNumber(left)
	rightNumber, rightIsNumber := parseNumber(right)
	switch {
	case leftIsNumber && rightIsNumber:
		return compareUint64(leftNumber, rightNumber)
	case leftIsNumber:
		return -1
	case rightIsNumber:
		return 1
	}
	return strings.Compare(left, right)
}

func parseNumber(value string) (uint64, bool) {
	if value == "" || len(value) > 9 {
		return 0, false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, false
		}
	}
	number, err := strconv.ParseUint(value, 10, 64)
	return number, err == nil
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character == '-') {
			return false
		}
	}
	return true
}

func sign(greater bool) int {
	if greater {
		return 1
	}
	return -1
}

func compareInt(left, right int) int {
	return compareUint64(uint64(left), uint64(right))
}

func compareUint64(left, right uint64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}
