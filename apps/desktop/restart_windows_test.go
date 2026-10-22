// SPDX-License-Identifier: MPL-2.0

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const restartProbeEnv = "ANIMEPORTABLE_RESTART_PROBE"

// When the test binary is started by TestStartAgain it only records how it was started.
func init() {
	output := os.Getenv(restartProbeEnv)
	if output == "" {
		return
	}
	directory, _ := os.Getwd()
	record := strings.Join(append([]string{directory}, os.Args[1:]...), "\n")
	temporary := output + ".tmp"
	if os.WriteFile(temporary, []byte(record), 0o600) != nil || os.Rename(temporary, output) != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestStartAgainStartsTheProgramWithNoArgumentsFromItsFolder(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "probe.txt")
	t.Setenv(restartProbeEnv, output)
	if err := startAgain(executable); err != nil {
		t.Fatalf("startAgain = %v", err)
	}
	var record []byte
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if record, err = os.ReadFile(output); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("the started program never reported: %v", err)
	}
	lines := strings.Split(string(record), "\n")
	if len(lines) != 1 {
		t.Fatalf("the program was given arguments %q, want none", lines[1:])
	}
	if !strings.EqualFold(filepath.Clean(lines[0]), filepath.Dir(executable)) {
		t.Fatalf("the program ran in %q, want its own folder %q", lines[0], filepath.Dir(executable))
	}
	if err := startAgain(filepath.Join(t.TempDir(), "missing.exe")); err == nil {
		t.Fatal("startAgain succeeded for a program that does not exist")
	}
}

func TestRestartAfterShutdownWarnsWhenTheProgramCannotBeStarted(t *testing.T) {
	failure := errors.New("cannot start")
	tests := []struct {
		name          string
		executableErr error
		startErr      error
		wantStarts    int
		wantWarnings  int
	}{
		{"starts", nil, nil, 1, 0},
		{"start fails", nil, failure, 1, 1},
		{"unknown executable", failure, nil, 0, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var starts int
			var warnings []string
			restartAfterShutdown("app.exe", test.executableErr,
				func(string) error { starts++; return test.startErr },
				func(message string) { warnings = append(warnings, message) })
			if starts != test.wantStarts || len(warnings) != test.wantWarnings {
				t.Fatalf("starts %d, warnings %q; want %d, %d", starts, warnings, test.wantStarts, test.wantWarnings)
			}
			if len(warnings) == 1 && warnings[0] != "無法自動重新開啟，請手動開啟程式。" {
				t.Fatalf("warning = %q", warnings[0])
			}
		})
	}
}
