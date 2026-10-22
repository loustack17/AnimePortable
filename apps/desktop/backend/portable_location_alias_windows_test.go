//go:build windows

// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// The 8.3 short name is a second spelling of the live data folder; it must not be accepted as
// another folder to move into.
func TestOpensFolderRecognisesAShortNameOfTheLiveFolder(t *testing.T) {
	fixture := newLocationFixture(t)
	store := openSeededStore(t, fixture.plan)
	defer store.Close()

	long, err := windows.UTF16PtrFromString(fixture.plan.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, windows.MAX_PATH)
	length, err := windows.GetShortPathName(long, &buffer[0], uint32(len(buffer)))
	if err != nil || length == 0 || int(length) > len(buffer) {
		t.Skipf("no short name for the data folder: %v", err)
	}
	short := windows.UTF16ToString(buffer[:length])
	if strings.EqualFold(short, fixture.plan.DataDir) {
		t.Skip("the data folder has no distinct short name")
	}
	if !fixture.plan.opensFolder(short) {
		t.Fatalf("opensFolder(%q) = false for the live folder %q", short, fixture.plan.DataDir)
	}
}
