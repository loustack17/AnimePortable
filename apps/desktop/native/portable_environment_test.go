// SPDX-License-Identifier: MPL-2.0

package native

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFyneEnvironmentPathsStayInsidePortableData(t *testing.T) {
	data := filepath.Join(t.TempDir(), "portable data")
	for _, goos := range []string{"windows", "linux", "darwin"} {
		paths := fyneEnvironmentPaths(goos, data)
		if len(paths) == 0 {
			t.Fatalf("no environment paths for %s", goos)
		}
		for key, value := range paths {
			if !filepath.IsAbs(value) || !strings.HasPrefix(filepath.Clean(value), data+string(filepath.Separator)) {
				t.Fatalf("%s %s resolves outside portable data: %q", goos, key, value)
			}
		}
	}
}

func TestFyneEnvironmentRedirectPreservesOriginalForMPVAndRestores(t *testing.T) {
	data := filepath.Join(t.TempDir(), "portable data")
	paths := fyneEnvironmentPaths(runtime.GOOS, data)
	if len(paths) == 0 {
		t.Skip("unsupported desktop OS")
	}
	prior := make(map[string]environmentValue, len(paths))
	for key := range paths {
		value, present := os.LookupEnv(key)
		prior[key] = environmentValue{value: value, present: present}
	}
	original, restore, err := RedirectFyneEnvironment(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	for key, want := range paths {
		if got := os.Getenv(key); got != want {
			t.Fatalf("redirected %s=%q, want %q", key, got, want)
		}
		foundOriginal := false
		for _, pair := range original {
			if strings.HasPrefix(strings.ToUpper(pair), strings.ToUpper(key)+"=") {
				foundOriginal = true
				if !prior[key].present || !strings.EqualFold(strings.SplitN(pair, "=", 2)[1], prior[key].value) {
					t.Fatalf("original %s was not preserved", key)
				}
			}
		}
		if foundOriginal != prior[key].present {
			t.Fatalf("original presence of %s changed", key)
		}
	}
	if info, err := os.Stat(data); err != nil || !info.IsDir() {
		t.Fatalf("portable data directory was not created: info=%v err=%v", info, err)
	}
	restore()
	for key, previous := range prior {
		value, present := os.LookupEnv(key)
		if present != previous.present || value != previous.value {
			t.Fatalf("%s was not restored", key)
		}
	}
}
