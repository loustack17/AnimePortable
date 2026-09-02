// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"errors"

	"animeportable/core"
)

var (
	ErrInvalidInput     = errors.New("invalid input")
	ErrIdentityConflict = errors.New("identity conflict")
	ErrMigration        = errors.New("migration failed")
	ErrStorage          = errors.New("storage failure")
	ErrClosed           = errors.New("store closed")
)

func sanitizeError(ctx context.Context, err, fallback error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if ctx != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return contextErr
		}
	}
	if errors.Is(err, core.ErrNotFound) {
		return core.ErrNotFound
	}
	for _, safe := range []error{
		ErrInvalidInput,
		ErrIdentityConflict,
		ErrMigration,
		ErrStorage,
		ErrClosed,
	} {
		if errors.Is(err, safe) {
			return safe
		}
	}
	return fallback
}
