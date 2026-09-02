// SPDX-License-Identifier: MPL-2.0

package mpv

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	ErrNotFound    = errors.New("mpv: executable not found; install mpv or configure its path")
	ErrInvalidPath = errors.New("mpv: configured executable path is invalid")
)

type Executable struct {
	path string
}

func (Executable) String() string {
	return "mpv.Executable{redacted}"
}

func (Executable) GoString() string {
	return "mpv.Executable{redacted}"
}

type locatorDeps struct {
	goos         string
	getenv       func(string) string
	lookPath     func(string) (string, error)
	abs          func(string) (string, error)
	evalSymlinks func(string) (string, error)
	stat         func(string) (fs.FileInfo, error)
}

type locator struct {
	deps locatorDeps
}

func Find(configured string) (Executable, error) {
	return newLocator(locatorDeps{}).find(configured)
}

func newLocator(deps locatorDeps) locator {
	if deps.goos == "" {
		deps.goos = runtime.GOOS
	}
	if deps.getenv == nil {
		deps.getenv = os.Getenv
	}
	if deps.lookPath == nil {
		deps.lookPath = exec.LookPath
	}
	if deps.abs == nil {
		deps.abs = filepath.Abs
	}
	if deps.evalSymlinks == nil {
		deps.evalSymlinks = filepath.EvalSymlinks
	}
	if deps.stat == nil {
		deps.stat = os.Stat
	}
	return locator{deps: deps}
}

func (locator locator) find(configured string) (Executable, error) {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		if !isAbsolute(locator.deps.goos, configured) {
			return Executable{}, ErrInvalidPath
		}
		executable, ok := locator.resolve(configured)
		if !ok {
			return Executable{}, ErrInvalidPath
		}
		return executable, nil
	}

	name := "mpv"
	if locator.deps.goos == "windows" {
		name = "mpv.exe"
	}
	if candidate, err := locator.deps.lookPath(name); err == nil {
		if executable, ok := locator.resolve(candidate); ok {
			return executable, nil
		}
	}
	for _, candidate := range locator.commonPaths() {
		if executable, ok := locator.resolve(candidate); ok {
			return executable, nil
		}
	}
	return Executable{}, ErrNotFound
}

func (locator locator) resolve(candidate string) (Executable, bool) {
	if !isAbsolute(locator.deps.goos, candidate) {
		return Executable{}, false
	}
	absolute, err := locator.deps.abs(candidate)
	if err != nil || !isAbsolute(locator.deps.goos, absolute) {
		return Executable{}, false
	}
	resolved, err := locator.deps.evalSymlinks(absolute)
	if err != nil {
		if locator.deps.goos != "windows" {
			return Executable{}, false
		}
		resolved = absolute
	}
	resolved, err = locator.deps.abs(resolved)
	if err != nil || !isAbsolute(locator.deps.goos, resolved) || !validExtension(locator.deps.goos, resolved) {
		return Executable{}, false
	}
	info, err := locator.deps.stat(resolved)
	if err != nil || info == nil || !info.Mode().IsRegular() {
		return Executable{}, false
	}
	if locator.deps.goos != "windows" && info.Mode()&0111 == 0 {
		return Executable{}, false
	}
	return Executable{path: resolved}, true
}

func (locator locator) commonPaths() []string {
	var candidates []string
	switch locator.deps.goos {
	case "linux":
		candidates = []string{"/usr/bin/mpv", "/usr/local/bin/mpv", "/snap/bin/mpv"}
	case "darwin":
		candidates = []string{
			"/Applications/mpv.app/Contents/MacOS/mpv",
			"/opt/homebrew/bin/mpv",
			"/usr/local/bin/mpv",
			"/opt/local/bin/mpv",
		}
	case "windows":
		candidates = make([]string, 0, 3)
		for _, root := range []string{locator.deps.getenv("ProgramFiles"), locator.deps.getenv("ProgramFiles(x86)")} {
			if root != "" {
				candidates = append(candidates, joinWindows(root, "mpv", "mpv.exe"))
			}
		}
		if root := locator.deps.getenv("LOCALAPPDATA"); root != "" {
			candidates = append(candidates, joinWindows(root, "Programs", "mpv", "mpv.exe"))
		}
	}
	return uniquePaths(locator.deps.goos, candidates)
}

func uniquePaths(goos string, candidates []string) []string {
	seen := make(map[string]struct{}, len(candidates))
	unique := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		key := candidate
		if goos == "windows" {
			key = strings.ToLower(strings.TrimRight(strings.ReplaceAll(candidate, "/", `\`), `\`))
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, candidate)
	}
	return unique
}

func joinWindows(parts ...string) string {
	joined := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		part = strings.Trim(part, `\/`)
		if joined == "" {
			joined = part
			continue
		}
		joined += `\` + part
	}
	return joined
}

func isAbsolute(goos, value string) bool {
	if goos != "windows" {
		return strings.HasPrefix(value, "/")
	}
	if strings.HasPrefix(value, `\\`) || strings.HasPrefix(value, "//") {
		return true
	}
	return len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '\\' || value[2] == '/')
}

func validExtension(goos, value string) bool {
	if goos != "windows" {
		return true
	}
	lastSeparator := strings.LastIndexAny(value, `\/`)
	base := value[lastSeparator+1:]
	return len(base) > len(".exe") && strings.EqualFold(base[len(base)-len(".exe"):], ".exe")
}
