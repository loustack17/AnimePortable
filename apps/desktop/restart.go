// SPDX-License-Identifier: MPL-2.0

package main

import (
	"os/exec"
	"path/filepath"

	"animeportable/apps/desktop/uitext"
)

// startAgain starts executable again with no arguments, from its own folder, and does not
// wait for it. The caller has finished its shutdown, so the new process can open the same
// files. exec connects the standard handles to the null device; a bare os.StartProcess
// without them is rejected on Windows.
func startAgain(executable string) error {
	command := exec.Command(executable)
	command.Dir = filepath.Dir(executable)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

// restartAfterShutdown starts the program again once the app has shut down. Nothing is left
// to show a message with at that point, so a failure goes to warn, which must not need the UI.
func restartAfterShutdown(executable string, executableErr error, start func(string) error, warn func(string)) {
	if executableErr != nil || start(executable) != nil {
		warn(uitext.T().RestartFailed)
	}
}
