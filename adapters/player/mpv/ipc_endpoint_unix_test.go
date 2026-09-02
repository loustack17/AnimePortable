//go:build darwin || linux

// SPDX-License-Identifier: MPL-2.0

package mpv

import (
	"os"
	"testing"
)

func TestTrustedRuntimeBasePermissions(t *testing.T) {
	directory := t.TempDir()
	for _, test := range []struct {
		mode os.FileMode
		want bool
	}{
		{mode: 0o700, want: true},
		{mode: 0o777, want: false},
		{mode: os.ModeSticky | 0o777, want: true},
	} {
		if err := os.Chmod(directory, test.mode); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatal(err)
		}
		if got := trustedRuntimeBase(info); got != test.want {
			t.Fatalf("mode %v trusted = %v", test.mode, got)
		}
	}
}

func TestUnixEndpointsAreUnique(t *testing.T) {
	first, err := newEndpoint()
	if err != nil {
		t.Fatal(err)
	}
	defer first.cleanup()
	second, err := newEndpoint()
	if err != nil {
		t.Fatal(err)
	}
	defer second.cleanup()
	if first.name == second.name {
		t.Fatal("IPC endpoints are not unique")
	}
}
