// SPDX-License-Identifier: MPL-2.0

package core

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrInvalidPlayback  = errors.New("invalid playback request")
	ErrPlaybackTracking = errors.New("playback tracking unavailable")
)
