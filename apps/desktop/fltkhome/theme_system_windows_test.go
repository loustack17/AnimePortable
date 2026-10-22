//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// fakeAppsUseLightTheme replaces the registry read for the test.
func fakeAppsUseLightTheme(t *testing.T, value uint64, err error) {
	t.Helper()
	original := readAppsUseLightTheme
	readAppsUseLightTheme = func() (uint64, error) { return value, err }
	t.Cleanup(func() { readAppsUseLightTheme = original })
}

func TestSystemThemeFollowsAppsUseLightTheme(t *testing.T) {
	for _, test := range []struct {
		name     string
		value    uint64
		err      error
		wantDark bool
	}{
		{name: "0 is dark", value: 0, wantDark: true},
		{name: "1 is light", value: 1, wantDark: false},
		{name: "missing is light", err: registry.ErrNotExist, wantDark: false},
		{name: "unreadable is light", err: errors.New("access denied"), wantDark: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			fakeAppsUseLightTheme(t, test.value, test.err)
			if got := systemUsesDarkTheme(); got != test.wantDark {
				t.Fatalf("systemUsesDarkTheme() = %v, want %v", got, test.wantDark)
			}
		})
	}
}

func TestResolveDarkReadsTheSystemOnlyForTheResolvedSystemAppearance(t *testing.T) {
	fakeAppsUseLightTheme(t, 0, nil)
	for appearance, want := range map[string]bool{"dark": true, "light": false, "system": true} {
		if got := resolveDark(appearance); got != want {
			t.Errorf("resolveDark(%q) = %v, want %v with a dark system", appearance, got, want)
		}
	}
	fakeAppsUseLightTheme(t, 1, nil)
	for appearance, want := range map[string]bool{"dark": true, "light": false, "system": false} {
		if got := resolveDark(appearance); got != want {
			t.Errorf("resolveDark(%q) = %v, want %v with a light system", appearance, got, want)
		}
	}
}
