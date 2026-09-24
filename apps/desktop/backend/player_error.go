// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"errors"

	"animeportable/adapters/player/mpv"
)

type PlayerErrorKind uint8

const (
	PlayerErrorOther PlayerErrorKind = iota
	PlayerErrorMissing
	PlayerErrorInvalidPath
)

func ClassifyPlayerError(err error) PlayerErrorKind {
	if errors.Is(err, mpv.ErrNotFound) {
		return PlayerErrorMissing
	}
	if errors.Is(err, mpv.ErrInvalidPath) {
		return PlayerErrorInvalidPath
	}
	return PlayerErrorOther
}
