// SPDX-License-Identifier: MPL-2.0

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const markerName = ".animeportable-loop23-acceptance.json"
const guardName = ".animeportable-loop23-operation"
const maxEntries = 20000
const maxFileBytes int64 = 512 << 20
const maxTreeBytes int64 = 2 << 30

type profilePaths struct{ base, parent, root, appdata, database, guard string }
type inventoryEntry struct {
	Path      string
	Directory bool
	SHA256    string
}
type ownership struct {
	Version          int
	Root             string
	Mode             string
	FixtureSHA256    string
	ExecutableSHA256 string
	RuntimeData      bool
	Entries          []inventoryEntry
}
type operationGuard struct {
	file *os.File
	path string
}

type guardRecord struct {
	Version int
	Root    string
	PID     int
}

func writeGuard(file *os.File, p profilePaths) error {
	data, err := json.Marshal(guardRecord{Version: 2, Root: p.root, PID: os.Getpid()})
	if err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Truncate(int64(len(data))); err != nil {
		return err
	}
	return file.Sync()
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func newPaths(base string) (profilePaths, error) {
	if !filepath.IsAbs(base) || strings.HasPrefix(base, `\\`) || strings.HasPrefix(base, "//") {
		return profilePaths{}, errors.New("application data base must be a local absolute path")
	}
	base = filepath.Clean(base)
	parent := filepath.Join(base, "AnimePortable", "acceptance")
	root := filepath.Join(parent, "loop23")
	return profilePaths{base, parent, root, filepath.Join(root, "appdata"), filepath.Join(root, "appdata", "AnimePortable", "animeportable.db"), filepath.Join(parent, guardName)}, nil
}

func validatePaths(p profilePaths) error {
	expected, err := newPaths(p.base)
	if err != nil || expected != p {
		return errors.New("unexpected acceptance root")
	}
	relative, err := filepath.Rel(p.parent, p.root)
	if err != nil || relative != "loop23" {
		return errors.New("acceptance root escapes parent")
	}
	return checkAncestors(p.parent)
}

func regularOrDirectory(info os.FileInfo) bool {
	if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		attributes := reflect.ValueOf(info.Sys())
		if attributes.Kind() != reflect.Pointer || attributes.IsNil() {
			return false
		}
		field := attributes.Elem().FieldByName("FileAttributes")
		if !field.IsValid() || field.Kind() != reflect.Uint32 || field.Uint()&0x400 != 0 {
			return false
		}
	}
	return info.IsDir() || info.Mode().IsRegular()
}

func checkAncestors(path string) error {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil {
			if !regularOrDirectory(info) || !info.IsDir() {
				return errors.New("unsafe directory or reparse point")
			}
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil || !samePath(resolved, current) {
				return errors.New("directory is not canonical")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	return nil
}

func makeDirectories(path string) error {
	if err := checkAncestors(path); err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	return checkAncestors(path)
}

func acquire(p profilePaths) (*operationGuard, error) {
	if err := validatePaths(p); err != nil {
		return nil, err
	}
	if err := makeDirectories(p.parent); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(p.guard, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, errors.New("operation guard exists; use recover to verify process liveness safely")
	}
	if _, err := lockGuard(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := writeGuard(file, p); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &operationGuard{file, p.guard}, nil
}

func (g *operationGuard) release() error {
	before, err := g.file.Stat()
	if closeErr := g.file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	after, err := os.Lstat(g.path)
	if err != nil || !regularOrDirectory(after) || !os.SameFile(before, after) {
		return errors.New("operation guard changed; preserved")
	}
	return os.Remove(g.path)
}

func fileDigest(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !regularOrDirectory(info) || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return "", errors.New("unsafe or oversized file")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return "", errors.New("file changed while opening")
	}
	hash := sha256.New()
	count, err := io.Copy(hash, io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return "", err
	}
	if count > maxFileBytes {
		return "", errors.New("file grew beyond limit")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func permittedEntry(path string, directory bool, runtimeData bool) bool {
	if directory && (path == "appdata" || path == "appdata/AnimePortable") {
		return true
	}
	if !directory && (path == "appdata/AnimePortable/animeportable.db" || path == "appdata/AnimePortable/animeportable.db-wal" || path == "appdata/AnimePortable/animeportable.db-shm") {
		return true
	}
	return runtimeData && (path == "appdata/animeportable.exe" && directory || strings.HasPrefix(path, "appdata/animeportable.exe/"))
}

func inventory(p profilePaths, runtimeData bool) ([]inventoryEntry, error) {
	if err := validatePaths(p); err != nil {
		return nil, err
	}
	if err := checkAncestors(p.root); err != nil {
		return nil, err
	}
	entries := []inventoryEntry{}
	var total int64
	err := filepath.WalkDir(p.root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == p.root {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !regularOrDirectory(info) {
			return errors.New("nonregular entry or reparse point in acceptance profile")
		}
		relative, err := filepath.Rel(p.root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == markerName {
			if !info.Mode().IsRegular() {
				return errors.New("invalid ownership marker")
			}
			return nil
		}
		if !permittedEntry(relative, info.IsDir(), runtimeData) {
			return fmt.Errorf("unknown acceptance entry: %s", relative)
		}
		item := inventoryEntry{Path: relative, Directory: info.IsDir()}
		if !info.IsDir() {
			total += info.Size()
			if total > maxTreeBytes {
				return errors.New("acceptance profile exceeds size limit")
			}
			item.SHA256, err = fileDigest(path)
			if err != nil {
				return err
			}
		}
		entries = append(entries, item)
		if len(entries) > maxEntries {
			return errors.New("acceptance profile exceeds entry limit")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func writeMarker(p profilePaths, owner ownership, exclusive bool) error {
	data, err := json.MarshalIndent(owner, "", "  ")
	if err != nil {
		return err
	}
	flags := os.O_WRONLY | os.O_TRUNC
	if exclusive {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}
	file, err := os.OpenFile(filepath.Join(p.root, markerName), flags, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	return errors.Join(writeErr, file.Close())
}

func readOwnership(p profilePaths) (ownership, error) {
	var owner ownership
	if err := validatePaths(p); err != nil {
		return owner, err
	}
	if err := checkAncestors(p.root); err != nil {
		return owner, err
	}
	marker := filepath.Join(p.root, markerName)
	info, err := os.Lstat(marker)
	if err != nil || !regularOrDirectory(info) || !info.Mode().IsRegular() || info.Size() > 8<<20 {
		return owner, errors.New("ownership marker missing or invalid")
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		return owner, err
	}
	if err := json.Unmarshal(data, &owner); err != nil {
		return owner, errors.New("invalid ownership metadata")
	}
	if owner.Version != 1 || owner.Root != p.root || owner.FixtureSHA256 != fixtureDigest() || (owner.Mode != "synthetic" && owner.Mode != "real") {
		return owner, errors.New("ownership metadata does not match this tool/profile")
	}
	if err := requireProfileEntries(owner.Entries); err != nil {
		return owner, err
	}
	return owner, nil
}

func requireProfileEntries(entries []inventoryEntry) error {
	required := map[string]bool{"appdata": false, "appdata/AnimePortable": false, "appdata/AnimePortable/animeportable.db": false}
	seen := make(map[string]bool)
	for _, entry := range entries {
		if seen[entry.Path] || !permittedEntry(entry.Path, entry.Directory, true) {
			return errors.New("invalid ownership inventory")
		}
		seen[entry.Path] = true
		if !entry.Directory {
			if digest, err := hex.DecodeString(entry.SHA256); err != nil || len(digest) != sha256.Size {
				return errors.New("invalid ownership file digest")
			}
		}
		if _, ok := required[entry.Path]; ok {
			required[entry.Path] = entry.Directory == (entry.Path != "appdata/AnimePortable/animeportable.db")
		}
	}
	for _, found := range required {
		if !found {
			return errors.New("profile is incomplete")
		}
	}
	return nil
}

func validateOwned(p profilePaths) (ownership, error) {
	owner, err := readOwnership(p)
	if err != nil {
		return owner, err
	}
	actual, err := inventory(p, owner.RuntimeData || owner.ExecutableSHA256 != "")
	if err != nil {
		return owner, err
	}
	if !reflect.DeepEqual(actual, owner.Entries) {
		return owner, errors.New("profile contains unknown, missing or modified state; preserve it")
	}
	if err := requireProfileEntries(actual); err != nil {
		return owner, err
	}
	return owner, nil
}

func settledInventory(p profilePaths, scan func(profilePaths, bool) ([]inventoryEntry, error), pause func()) ([]inventoryEntry, error) {
	var previous []inventoryEntry
	var lastErr error
	for attempt := 0; attempt < 25; attempt++ {
		current, err := scan(p, true)
		if err == nil && previous != nil && reflect.DeepEqual(previous, current) {
			return current, nil
		}
		previous = nil
		lastErr = err
		if err == nil {
			previous = current
			lastErr = errors.New("runtime files are still changing")
		}
		if attempt < 24 {
			pause()
		}
	}
	return nil, fmt.Errorf("post-exit inventory did not settle; preserve state and run recover after app exit: %w", lastErr)
}

func collectRuntimeInventory(p profilePaths) ([]inventoryEntry, error) {
	return settledInventory(p, inventory, func() { time.Sleep(200 * time.Millisecond) })
}

func parseGuard(data []byte, p profilePaths) (int, error) {
	var record guardRecord
	if json.Unmarshal(data, &record) == nil {
		if record.Version != 2 || record.Root != p.root || record.PID <= 0 || uint64(record.PID) > uint64(^uint32(0)) {
			return 0, errors.New("guard does not belong to this acceptance root")
		}
		return record.PID, nil
	}
	text := string(data)
	if !strings.HasPrefix(text, "pid=") || !strings.HasSuffix(text, "\n") {
		return 0, errors.New("invalid guard metadata")
	}
	value := strings.TrimSuffix(strings.TrimPrefix(text, "pid="), "\n")
	pid, err := strconv.ParseUint(value, 10, 32)
	if err != nil || pid == 0 || strconv.FormatUint(pid, 10) != value {
		return 0, errors.New("invalid legacy guard PID")
	}
	return int(pid), nil
}

func recoverProfile(p profilePaths, alive func(int) (bool, error), stopped func() error) (result error) {
	owner, err := readOwnership(p)
	if err != nil {
		return err
	}
	before, err := os.Lstat(p.guard)
	if err != nil || !regularOrDirectory(before) || !before.Mode().IsRegular() || before.Size() > 4096 {
		return errors.New("tool-owned guard missing or invalid")
	}
	file, err := os.OpenFile(p.guard, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	guard := &operationGuard{file: file, path: p.guard}
	release := false
	defer func() {
		if release {
			result = errors.Join(result, guard.release())
		} else {
			result = errors.Join(result, file.Close())
		}
	}()
	if _, err := lockGuard(file); err != nil {
		return errors.New("operation lease is active; recovery refused")
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return errors.New("guard changed during recovery")
	}
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return err
	}
	if len(data) > 4096 {
		return errors.New("guard exceeds metadata limit")
	}
	pid, err := parseGuard(data, p)
	if err != nil {
		return err
	}
	running, err := alive(pid)
	if err != nil {
		return fmt.Errorf("cannot prove operation process stopped: %w", err)
	}
	if running {
		return errors.New("recorded operation process is alive; recovery refused")
	}
	if err := stopped(); err != nil {
		return fmt.Errorf("cannot prove native app stopped: %w", err)
	}
	entries, err := collectRuntimeInventory(p)
	if err != nil {
		return err
	}
	if err := requireProfileEntries(entries); err != nil {
		return err
	}
	current, err := readOwnership(p)
	if err != nil || !reflect.DeepEqual(current, owner) {
		return errors.New("ownership changed during recovery")
	}
	if err := stopped(); err != nil {
		return err
	}
	if err := writeGuard(file, p); err != nil {
		return err
	}
	owner.RuntimeData = true
	owner.Entries = entries
	if err := writeMarker(p, owner, false); err != nil {
		return err
	}
	if _, err := validateOwned(p); err != nil {
		return err
	}
	release = true
	fmt.Println("Recovered stale tool-owned guard; acceptance data preserved. You may launch again or reset.")
	return nil
}

func reset(p profilePaths) (result error) {
	guard, err := acquire(p)
	if err != nil {
		return err
	}
	retain := false
	defer func() {
		if retain {
			_ = guard.file.Close()
		} else {
			result = errors.Join(result, guard.release())
		}
	}()
	owner, err := validateOwned(p)
	if err != nil {
		return err
	}
	retain = true
	entries := append([]inventoryEntry(nil), owner.Entries...)
	sort.Slice(entries, func(i, j int) bool { return len(entries[i].Path) > len(entries[j].Path) })
	for _, entry := range entries {
		path := filepath.Join(p.root, filepath.FromSlash(entry.Path))
		if err := checkAncestors(filepath.Dir(path)); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil || !regularOrDirectory(info) || info.IsDir() != entry.Directory {
			return errors.New("entry changed during reset")
		}
		if !entry.Directory {
			digest, err := fileDigest(path)
			if err != nil || digest != entry.SHA256 {
				return errors.New("file changed during reset")
			}
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	if err := os.Remove(filepath.Join(p.root, markerName)); err != nil {
		return err
	}
	if err := os.Remove(p.root); err != nil {
		return err
	}
	retain = false
	return nil
}

func childEnvironment(environment []string, appdata string) []string {
	result := make([]string, 0, len(environment)+1)
	for _, value := range environment {
		key, _, _ := strings.Cut(value, "=")
		if !strings.EqualFold(key, "APPDATA") {
			result = append(result, value)
		}
	}
	return append(result, "APPDATA="+appdata)
}

func runProduction(executable string, environment []string) error {
	cmd := exec.Command(executable)
	cmd.Env = environment
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func launch(p profilePaths, exe string, runner func(string, []string) error) (result error) {
	guard, err := acquire(p)
	if err != nil {
		return err
	}
	retain := false
	defer func() {
		if retain {
			_ = guard.file.Close()
		} else {
			result = errors.Join(result, guard.release())
		}
	}()
	owner, err := validateOwned(p)
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(exe)
	if err != nil || !strings.EqualFold(filepath.Base(absolute), "animeportable.exe") {
		return errors.New("--exe must name the production animeportable.exe")
	}
	if err := checkAncestors(filepath.Dir(absolute)); err != nil {
		return err
	}
	digest, err := fileDigest(absolute)
	if err != nil {
		return err
	}
	fmt.Printf("Mode: %s\nExecutable SHA256: %s\nChild APPDATA: %s\nDatabase: %s\nKeep this terminal open until the app exits.\n", owner.Mode, digest, p.appdata, p.database)
	retain = true
	if err := runner(absolute, childEnvironment(os.Environ(), p.appdata)); err != nil {
		return fmt.Errorf("native launch did not finish cleanly; guard retained for recovery: %w", err)
	}
	owner.ExecutableSHA256 = digest
	owner.RuntimeData = true
	owner.Entries, err = collectRuntimeInventory(p)
	if err != nil {
		return err
	}
	if err := requireProfileEntries(owner.Entries); err != nil {
		return err
	}
	if err := writeMarker(p, owner, false); err != nil {
		return err
	}
	retain = false
	return nil
}

func run(arguments []string) error {
	if runtime.GOOS != "windows" {
		return errors.New("native acceptance CLI is Windows-only")
	}
	if len(arguments) == 0 {
		return errors.New("usage: prepare|launch|reset|recover")
	}
	flags := flag.NewFlagSet(arguments[0], flag.ContinueOnError)
	exe := flags.String("exe", "", "production executable (launch)")
	category := flags.String("category-id", "", "Anime1 category ID (prepare)")
	post := flags.String("post-id", "", "Anime1 post ID (prepare)")
	mpv := flags.String("mpv-path", "", "absolute installed MPV path (prepare, optional)")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	paths, err := newPaths(base)
	if err != nil {
		return err
	}
	switch arguments[0] {
	case "prepare":
		if *exe != "" {
			return errors.New("--exe is only valid for launch")
		}
		return prepare(paths, prepareOptions{*category, *post, *mpv}, preflightEpisode)
	case "launch":
		if *category != "" || *post != "" || *mpv != "" || *exe == "" {
			return errors.New("launch requires only --exe")
		}
		return launch(paths, *exe, runProduction)
	case "reset":
		if *exe != "" || *category != "" || *post != "" || *mpv != "" {
			return errors.New("reset accepts no flags")
		}
		return reset(paths)
	case "recover":
		if *exe != "" || *category != "" || *post != "" || *mpv != "" {
			return errors.New("recover accepts no flags")
		}
		return recoverProfile(paths, processAlive, nativeProcessesStopped)
	default:
		return errors.New("unknown operation")
	}
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
