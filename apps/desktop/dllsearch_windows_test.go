// SPDX-License-Identifier: MPL-2.0

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

const dllSearchProbeEnv = "ANIMEPORTABLE_DLL_SEARCH_PROBE"

// The search order is per process and cannot be undone, so a child copy of the test binary
// checks it: it loads a DLL from its current folder once before and once after the restriction
// and records each outcome.
func init() {
	output := os.Getenv(dllSearchProbeEnv)
	if output == "" {
		return
	}
	outcome := func(name string) byte {
		module, err := windows.LoadLibrary(name)
		if err != nil {
			return '0'
		}
		_ = windows.FreeLibrary(module)
		return '1'
	}
	before := outcome("probe-before.dll")
	after := byte('e')
	if restrictDLLSearch() == nil {
		after = outcome("probe-after.dll")
	}
	if os.WriteFile(output, []byte{before, after}, 0o600) != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestRestrictDLLSearchStopsLoadsFromTheCurrentFolder(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatal(err)
	}
	library, err := os.ReadFile(filepath.Join(system, "version.dll"))
	if err != nil {
		t.Fatal(err)
	}
	folder := t.TempDir()
	for _, name := range []string{"probe-before.dll", "probe-after.dll"} {
		if err := os.WriteFile(filepath.Join(folder, name), library, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(t.TempDir(), "probe.txt")
	command := exec.Command(executable)
	command.Dir = folder
	command.Env = append(os.Environ(), dllSearchProbeEnv+"="+output)
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatalf("probe failed: %v\n%s", err, raw)
	}
	record, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(record) != "10" {
		t.Fatalf("loads from the current folder before/after the restriction = %q, want \"10\" (control loads, restricted does not)", record)
	}
}
