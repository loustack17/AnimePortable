package core

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrUnsupported      = errors.New("unsupported")
	ErrInvalidPlayback  = errors.New("invalid playback request")
	ErrPlaybackTracking = errors.New("playback tracking unavailable")
)
