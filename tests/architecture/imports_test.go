// SPDX-License-Identifier: MPL-2.0

package architecture_test

import (
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
)

type packageInfo struct {
	ImportPath string
	Imports    []string
}

func TestCoreImportsOnlyPermittedPackages(t *testing.T) {
	command := exec.Command("go", "list", "-json", "animeportable/core/...")
	output, err := command.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			t.Fatalf("discovering core packages: %v\n%s", err, exitError.Stderr)
		}
		t.Fatalf("discovering core packages: %v", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(output)))
	found := false
	for {
		var packageData packageInfo
		err := decoder.Decode(&packageData)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(packageData.ImportPath, "animeportable/core") {
			continue
		}
		found = true
		for _, imported := range packageData.Imports {
			if forbiddenCoreImport(imported) {
				t.Fatalf("%s imports forbidden dependency %s", packageData.ImportPath, imported)
			}
		}
	}
	if !found {
		t.Fatal("core package was not discovered")
	}
}

func forbiddenCoreImport(importPath string) bool {
	for _, prefix := range []string{
		"github.com/wailsapp/wails",
		"github.com/mattn/go-sqlite3",
		"modernc.org/sqlite",
		"animeportable/adapters",
		"animeportable/apps/desktop",
	} {
		if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
			return true
		}
	}
	return false
}
