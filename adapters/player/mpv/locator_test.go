// SPDX-License-Identifier: MPL-2.0

package mpv

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFindConfiguredPathDoesNotFallback(t *testing.T) {
	lookedUp := false
	locator := newTestLocator(t, "linux", map[string]fs.FileMode{
		"/configured/mpv": 0o755,
		"/usr/bin/mpv":    0o755,
	})
	locator.deps.lookPath = func(name string) (string, error) {
		lookedUp = true
		return "/usr/bin/mpv", nil
	}

	executable, err := locator.find("/configured/mpv")
	if err != nil {
		t.Fatal(err)
	}
	if executable.path != "/configured/mpv" {
		t.Fatalf("configured path = %q", executable.path)
	}
	if lookedUp {
		t.Fatal("PATH was checked after a valid configured path")
	}

	for _, configured := range []string{"/missing/mpv", "/configured/directory"} {
		t.Run(configured, func(t *testing.T) {
			lookedUp = false
			_, err := locator.find(configured)
			if !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("error = %v", err)
			}
			if lookedUp {
				t.Fatal("invalid configured path fell back to PATH")
			}
			if strings.Contains(err.Error(), configured) {
				t.Fatalf("error leaked configured path: %v", err)
			}
		})
	}
}

func TestFindConfiguredPathResolvesAndValidates(t *testing.T) {
	cases := []struct {
		name string
		goos string
		path string
		mode fs.FileMode
		want string
		err  error
	}{
		{name: "unix symlink", goos: "linux", path: "/links/mpv", mode: 0o755, want: "/real/mpv"},
		{name: "unix not executable", goos: "linux", path: "/configured/mpv", mode: 0o644, err: ErrInvalidPath},
		{name: "unix directory", goos: "darwin", path: "/configured/mpv", mode: fs.ModeDir | 0o755, err: ErrInvalidPath},
		{name: "windows executable", goos: "windows", path: `C:\configured\MPV.EXE`, mode: 0o644, want: `C:\real\mpv.exe`},
		{name: "windows extension", goos: "windows", path: `C:\configured\mpv`, mode: 0o644, err: ErrInvalidPath},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			locator := newTestLocator(t, test.goos, map[string]fs.FileMode{test.path: test.mode, test.want: test.mode})
			executable, err := locator.find(test.path)
			if test.err != nil {
				if !errors.Is(err, test.err) {
					t.Fatalf("error = %v, want %v", err, test.err)
				}
				return
			}
			if err != nil || executable.path != test.want {
				t.Fatalf("executable = %q, error = %v", executable.path, err)
			}
		})
	}
}

func TestFindWindowsExecutableSurvivesJunctionResolutionFailure(t *testing.T) {
	path := `C:\Scoop\apps\mpv\current\mpv.exe`
	locator := newTestLocator(t, "windows", map[string]fs.FileMode{path: 0o644})
	locator.deps.evalSymlinks = func(string) (string, error) { return "", os.ErrNotExist }
	executable, err := locator.find(path)
	if err != nil || executable.path != path {
		t.Fatalf("executable = %q, error = %v", executable.path, err)
	}
}

func TestFindUsesPATHBeforeCommonPaths(t *testing.T) {
	locator := newTestLocator(t, "linux", map[string]fs.FileMode{
		"/path/mpv":          0o755,
		"/usr/bin/mpv":       0o755,
		"/usr/local/bin/mpv": 0o755,
	})
	var lookedUp string
	locator.deps.lookPath = func(name string) (string, error) {
		lookedUp = name
		return "/path/mpv", nil
	}

	executable, err := locator.find("")
	if err != nil || executable.path != "/path/mpv" {
		t.Fatalf("executable = %q, error = %v", executable.path, err)
	}
	if lookedUp != "mpv" {
		t.Fatalf("PATH lookup = %q", lookedUp)
	}
}

func TestFindRejectsRelativePATHResult(t *testing.T) {
	locator := newTestLocator(t, "linux", map[string]fs.FileMode{
		"/workspace/mpv": 0o755,
	})
	locator.deps.lookPath = func(string) (string, error) { return "mpv", nil }
	if _, err := locator.find(""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("relative PATH result error = %v", err)
	}
}

func TestFindFallsBackAcrossCommonPaths(t *testing.T) {
	cases := []struct {
		name   string
		goos   string
		env    map[string]string
		files  map[string]fs.FileMode
		want   string
		lookup string
	}{
		{
			name:   "linux",
			goos:   "linux",
			files:  map[string]fs.FileMode{"/usr/local/bin/mpv": 0o755},
			want:   "/usr/local/bin/mpv",
			lookup: "mpv",
		},
		{
			name:   "mac app",
			goos:   "darwin",
			files:  map[string]fs.FileMode{"/Applications/mpv.app/Contents/MacOS/mpv": 0o755},
			want:   "/Applications/mpv.app/Contents/MacOS/mpv",
			lookup: "mpv",
		},
		{
			name: "windows local app",
			goos: "windows",
			env: map[string]string{
				"ProgramFiles":      `C:\Program Files`,
				"ProgramFiles(x86)": `C:\Program Files (x86)`,
				"LOCALAPPDATA":      `C:\Users\test\AppData\Local`,
			},
			files:  map[string]fs.FileMode{`C:\Users\test\AppData\Local\Programs\mpv\mpv.exe`: 0o644},
			want:   `C:\Users\test\AppData\Local\Programs\mpv\mpv.exe`,
			lookup: "mpv.exe",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			locator := newTestLocator(t, test.goos, test.files)
			locator.deps.getenv = func(name string) string { return test.env[name] }
			var lookedUp string
			locator.deps.lookPath = func(name string) (string, error) {
				lookedUp = name
				return "", os.ErrNotExist
			}
			executable, err := locator.find("")
			if err != nil || executable.path != test.want {
				t.Fatalf("executable = %q, error = %v", executable.path, err)
			}
			if lookedUp != test.lookup {
				t.Fatalf("PATH lookup = %q", lookedUp)
			}
		})
	}
}

func TestFindReturnsSanitizedNotFound(t *testing.T) {
	locator := newTestLocator(t, "plan9", nil)
	locator.deps.lookPath = func(string) (string, error) {
		return "", os.ErrNotExist
	}

	_, err := locator.find("")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked source detail: %v", err)
	}
}

func TestExecutableFormattingIsRedactedAndPathIsImmutable(t *testing.T) {
	executable := Executable{path: `C:\Users\test\secret\mpv.exe`}
	if got := executable.String(); got != "mpv.Executable{redacted}" {
		t.Fatalf("String() = %q", got)
	}
	if got := executable.GoString(); got != "mpv.Executable{redacted}" {
		t.Fatalf("GoString() = %q", got)
	}
	formatted := fmt.Sprintf("%v %#v", executable, executable)
	if strings.Contains(formatted, "secret") {
		t.Fatalf("formatted executable leaked path: %s", formatted)
	}
	copy := executable
	if !reflect.DeepEqual(copy, executable) || copy.path != executable.path {
		t.Fatal("executable value is not stable after copying")
	}
}

type fakeFileInfo struct {
	mode fs.FileMode
}

func (info fakeFileInfo) Name() string       { return "mpv" }
func (info fakeFileInfo) Size() int64        { return 1 }
func (info fakeFileInfo) Mode() fs.FileMode  { return info.mode }
func (info fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (info fakeFileInfo) IsDir() bool        { return info.mode.IsDir() }
func (info fakeFileInfo) Sys() any           { return nil }

func newTestLocator(t *testing.T, goos string, files map[string]fs.FileMode) locator {
	t.Helper()
	entries := make(map[string]fs.FileInfo, len(files))
	for name, mode := range files {
		entries[name] = fakeFileInfo{mode: mode}
	}
	abs := func(name string) (string, error) {
		if isAbsolute(goos, name) {
			return name, nil
		}
		if goos == "windows" {
			return joinWindows(`C:\workspace`, name), nil
		}
		return "/workspace/" + name, nil
	}
	return newLocator(locatorDeps{
		goos:     goos,
		getenv:   func(string) string { return "" },
		lookPath: func(string) (string, error) { return "", os.ErrNotExist },
		abs:      abs,
		evalSymlinks: func(name string) (string, error) {
			if goos == "linux" && name == "/links/mpv" {
				return "/real/mpv", nil
			}
			if goos == "windows" && name == `C:\configured\MPV.EXE` {
				return `C:\real\mpv.exe`, nil
			}
			return name, nil
		},
		stat: func(name string) (fs.FileInfo, error) {
			if info, ok := entries[name]; ok {
				return info, nil
			}
			return nil, os.ErrNotExist
		},
	})
}
