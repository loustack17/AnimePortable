package core

import "errors"

var (
	ErrNotFound    = errors.New("not found")
	ErrUnsupported = errors.New("unsupported")
)
