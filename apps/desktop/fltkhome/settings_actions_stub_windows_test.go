//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"

	"animeportable/apps/desktop/backend"
)

// noSettingsActions is embedded by test services that do not exercise the rows that act.
type noSettingsActions struct{}

func (noSettingsActions) TestProxy(context.Context, backend.Settings) (backend.ProxyTestResult, error) {
	return backend.ProxyTestResult{}, backend.ErrUnavailable
}

func (noSettingsActions) CheckForUpdate(context.Context) (backend.UpdateInfo, error) {
	return backend.UpdateInfo{}, backend.ErrUnavailable
}

func (noSettingsActions) DataLocation(context.Context) (backend.DataLocation, error) {
	return backend.DataLocation{}, backend.ErrUnavailable
}

func (noSettingsActions) MoveDataLocation(context.Context, backend.MoveDataLocationRequest) (backend.MoveDataLocationResult, error) {
	return backend.MoveDataLocationResult{}, backend.ErrUnavailable
}
