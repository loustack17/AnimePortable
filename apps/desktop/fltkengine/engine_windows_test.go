//go:build windows && amd64 && cgo

package fltkengine

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"animeportable/apps/desktop/fltkengine/internal/mpvwin"
	"animeportable/internal/runtimepin"
)

func TestSnapshotRejectsNilContext(t *testing.T) {
	engine := &Engine{}
	if _, err := engine.Snapshot(nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Snapshot(nil) error = %v, want context.Canceled", err)
	}
}

func TestSnapshotDurationPreservesMissingAndRejectsInvalidProperties(t *testing.T) {
	for _, test := range []struct {
		value   string
		want    time.Duration
		invalid bool
	}{
		{"", time.Second, false},
		{"42.75", 42750 * time.Millisecond, false},
		{"0", 0, false},
		{"-1", 0, true},
		{"NaN", 0, true},
		{"Inf", 0, true},
		{"1e30", 0, true},
		{"bad", 0, true},
	} {
		got, err := snapshotDuration(test.value, time.Second)
		if (err != nil) != test.invalid || got != test.want {
			t.Fatalf("property %q: duration=%v, error=%v", test.value, got, err)
		}
	}
}

func TestLibraryLocation(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "animeportable.exe")
	getenv := func(values map[string]string) func(string) string {
		return func(name string) string { return values[name] }
	}
	path, hash, err := libraryLocation(executable, getenv(nil))
	if err != nil || path != filepath.Join(filepath.Dir(executable), "libmpv-2.dll") || hash != runtimepin.LibMPVSHA256 {
		t.Fatalf("default runtime: path=%q hash=%q err=%v", path, hash, err)
	}
	override := filepath.Join(t.TempDir(), "custom-libmpv.dll")
	customHash := strings.Repeat("a", 64)
	path, hash, err = libraryLocation(executable, getenv(map[string]string{
		"ANIMEPORTABLE_LIBMPV_OVERRIDE":        override,
		"ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256": customHash,
	}))
	if err != nil || path != override || hash != customHash {
		t.Fatalf("custom runtime: path=%q hash=%q err=%v", path, hash, err)
	}
	for name, values := range map[string]map[string]string{
		"missing hash":  {"ANIMEPORTABLE_LIBMPV_OVERRIDE": override},
		"missing path":  {"ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256": customHash},
		"relative path": {"ANIMEPORTABLE_LIBMPV_OVERRIDE": "libmpv-2.dll", "ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256": customHash},
		"short hash":    {"ANIMEPORTABLE_LIBMPV_OVERRIDE": override, "ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256": "abc"},
		"nonhex hash":   {"ANIMEPORTABLE_LIBMPV_OVERRIDE": override, "ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256": strings.Repeat("g", 64)},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := libraryLocation(executable, getenv(values))
			if !errors.Is(err, mpvwin.ErrInvalidLibrary) {
				t.Fatalf("expected invalid runtime configuration, got %v", err)
			}
		})
	}
}
