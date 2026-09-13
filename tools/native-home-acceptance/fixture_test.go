// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"animeportable/adapters/persistence/sqlite"
	"animeportable/core"
)

func testPaths(t *testing.T) profilePaths {
	t.Helper()
	paths, err := newPaths(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestSettledInventoryRetriesTransientSharingFailure(t *testing.T) {
	p := testPaths(t)
	calls := 0
	want := []inventoryEntry{{Path: "appdata", Directory: true}}
	entries, err := settledInventory(p, func(profilePaths, bool) ([]inventoryEntry, error) {
		calls++
		if calls == 1 {
			return nil, &os.PathError{Op: "open", Path: "WebView2.tmp", Err: errors.New("file is being used by another process")}
		}
		return want, nil
	}, func() {})
	if err != nil || calls != 3 || !reflect.DeepEqual(entries, want) {
		t.Fatal(calls, entries, err)
	}
	calls = 0
	_, err = settledInventory(p, func(profilePaths, bool) ([]inventoryEntry, error) { calls++; return nil, errors.New("still locked") }, func() {})
	if err == nil || calls != 25 {
		t.Fatal("retry is not bounded", calls, err)
	}
}

func staleGuard(t *testing.T, p profilePaths, legacy bool) {
	t.Helper()
	var data []byte
	if legacy {
		data = []byte("pid=42456\n")
	} else {
		var err error
		data, err = json.Marshal(guardRecord{Version: 2, Root: p.root, PID: 42456})
		if err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, p.guard, data)
}

func TestRecoverStaleGuardPreservesDataAndAllowsLifecycle(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "v2", true: "legacy"}[legacy], func(t *testing.T) {
			p := testPaths(t)
			mustPrepare(t, p)
			before, err := os.ReadFile(p.database)
			if err != nil {
				t.Fatal(err)
			}
			staleGuard(t, p, legacy)
			browser := filepath.Join(p.appdata, "animeportable.exe", "EBWebView")
			if err := os.MkdirAll(browser, 0700); err != nil {
				t.Fatal(err)
			}
			mustWrite(t, filepath.Join(browser, "cache"), []byte("preserved WebView cache"))
			probed := false
			err = recoverProfile(p, func(pid int) (bool, error) {
				probed = true
				if pid != 42456 {
					t.Fatal(pid)
				}
				return false, nil
			}, func() error { return nil })
			if err != nil || !probed {
				t.Fatal(err)
			}
			after, err := os.ReadFile(p.database)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("recovery changed database", err)
			}
			if _, err := os.Lstat(p.guard); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("guard remains", err)
			}
			exe := testExecutable(t)
			for round := 0; round < 2; round++ {
				if err := launch(p, exe, func(string, []string) error { return nil }); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(p.guard); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("normal launch left guard", err)
				}
			}
			if err := reset(p); err != nil {
				t.Fatal(err)
			}
			mustPrepare(t, p)
		})
	}
}

func TestRecoveryRefusesLiveUnknownOrLeasedOperation(t *testing.T) {
	for _, scenario := range []string{"alive", "unknown", "native alive", "lease"} {
		t.Run(scenario, func(t *testing.T) {
			p := testPaths(t)
			mustPrepare(t, p)
			if scenario == "lease" {
				g, err := acquire(p)
				if err != nil {
					t.Fatal(err)
				}
				defer g.release()
			} else {
				staleGuard(t, p, false)
			}
			alive := func(int) (bool, error) { return scenario == "alive", nil }
			if scenario == "unknown" {
				alive = func(int) (bool, error) { return false, errors.New("access denied") }
			}
			stopped := func() error {
				if scenario == "native alive" {
					return errors.New("native process active")
				}
				return nil
			}
			if err := recoverProfile(p, alive, stopped); err == nil {
				t.Fatal("unsafe recovery succeeded")
			}
			if _, err := os.Lstat(p.guard); err != nil {
				t.Fatal("guard removed", err)
			}
			if _, err := os.Lstat(p.database); err != nil {
				t.Fatal("database removed", err)
			}
		})
	}
}

func TestRecoveryRejectsOwnershipAndPathViolations(t *testing.T) {
	for _, scenario := range []string{"marker missing", "marker invalid", "root mismatch", "guard root mismatch", "outside path", "unknown file"} {
		t.Run(scenario, func(t *testing.T) {
			p := testPaths(t)
			mustPrepare(t, p)
			staleGuard(t, p, false)
			switch scenario {
			case "marker missing":
				if err := os.Remove(filepath.Join(p.root, markerName)); err != nil {
					t.Fatal(err)
				}
			case "marker invalid":
				mustWrite(t, filepath.Join(p.root, markerName), []byte("{}"))
			case "root mismatch":
				owner, err := readOwnership(p)
				if err != nil {
					t.Fatal(err)
				}
				owner.Root = p.base
				if err := writeMarker(p, owner, false); err != nil {
					t.Fatal(err)
				}
			case "guard root mismatch":
				data, err := json.Marshal(guardRecord{Version: 2, Root: p.base, PID: 42456})
				if err != nil {
					t.Fatal(err)
				}
				mustWrite(t, p.guard, data)
			case "outside path":
				p.root = p.base
			case "unknown file":
				mustWrite(t, filepath.Join(p.appdata, "personal"), []byte("keep"))
			}
			if err := recoverProfile(p, func(int) (bool, error) { return false, nil }, func() error { return nil }); err == nil {
				t.Fatal("invalid recovery succeeded")
			}
			if _, err := os.Lstat(p.guard); err != nil {
				t.Fatal("guard lost", err)
			}
		})
	}
}

func TestRecoveryAfterSimulatedAbnormalTermination(t *testing.T) {
	p := testPaths(t)
	mustPrepare(t, p)
	if err := launch(p, testExecutable(t), func(string, []string) error { return errors.New("abnormal exit") }); err == nil {
		t.Fatal("abnormal exit ignored")
	}
	if err := recoverProfile(p, func(int) (bool, error) { return false, nil }, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := reset(p); err != nil {
		t.Fatal(err)
	}
}
func mustPrepare(t *testing.T, p profilePaths) {
	t.Helper()
	if err := prepare(p, prepareOptions{}, nil); err != nil {
		t.Fatal(err)
	}
}
func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func testExecutable(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "animeportable.exe")
	mustWrite(t, path, []byte("test executable; never executed"))
	return path
}
func readHistory(t *testing.T, p profilePaths) []core.HistoryEntry {
	t.Helper()
	store, err := sqlite.Open(context.Background(), p.database)
	if err != nil {
		t.Fatal(err)
	}
	history, readErr := store.History(context.Background())
	if err := errors.Join(readErr, store.Close()); err != nil {
		t.Fatal(err)
	}
	return history
}

func TestPreparationRelationshipsAndRepeat(t *testing.T) {
	p := testPaths(t)
	normal := filepath.Join(p.base, "AnimePortable", "animeportable.db")
	if err := os.MkdirAll(filepath.Dir(normal), 0700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, normal, []byte("normal user data sentinel"))
	mustPrepare(t, p)
	store, err := sqlite.Open(context.Background(), p.database)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	anime, err := store.Anime(ctx, "loop23-demo")
	if err != nil {
		t.Fatal(err)
	}
	if anime.Title != "Loop 23 Acceptance Demo" {
		t.Fatal(anime)
	}
	refs, err := store.SourceRefs(ctx, anime.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(refs, []core.SourceRef{{Provider: "synthetic", ID: "loop23-demo"}}) {
		t.Fatal(refs)
	}
	maps, err := store.EpisodeMappings(ctx, anime.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(maps) != 1 || maps[0].AnimeID != anime.ID || maps[0].EpisodeID != "episode-1" || maps[0].Ref.Anime != refs[0] || maps[0].Ref.ID != "episode-1" {
		t.Fatal(maps)
	}
	follows, err := store.Following(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(follows, []core.AnimeID{anime.ID}) {
		t.Fatal(follows)
	}
	progress, err := store.Progress(ctx, anime.ID, "episode-1")
	if err != nil {
		t.Fatal(err)
	}
	if progress.Position != 125500*time.Millisecond || progress.Duration != 24*time.Minute || progress.Completed {
		t.Fatal(progress)
	}
	settings, err := store.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if settings.ResumePlayback != core.ToggleEnabled || settings.AutoplayNext != core.ToggleDisabled {
		t.Fatal(settings)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	first := readHistory(t, p)
	if len(first) != 1 || first[0].Progress != progress || first[0].LastPlayedAt.Format(time.RFC3339) != "2026-09-12T12:00:00Z" {
		t.Fatal(first)
	}
	if err := prepare(p, prepareOptions{}, nil); err == nil {
		t.Fatal("duplicate prepare overwrote state")
	}
	if err := reset(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(p.root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("reset left profile", err)
	}
	if err := reset(p); err == nil {
		t.Fatal("reset accepted missing profile")
	}
	mustPrepare(t, p)
	if !reflect.DeepEqual(first, readHistory(t, p)) {
		t.Fatal("reset/prepare changed deterministic state")
	}
	contents, err := os.ReadFile(normal)
	if err != nil || string(contents) != "normal user data sentinel" {
		t.Fatal("normal data changed", err)
	}
}

func TestLivePreparationSeedsValidatedIdentity(t *testing.T) {
	p := testPaths(t)
	calls := 0
	err := prepare(p, prepareOptions{CategoryID: "42", PostID: "101"}, func(ctx context.Context, category, post string) error {
		calls++
		if category != "42" || post != "101" {
			t.Fatal(category, post)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("preflight has no deadline")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal(calls)
	}
	owner, err := validateOwned(p)
	if err != nil {
		t.Fatal(err)
	}
	if owner.Mode != "real" {
		t.Fatal(owner.Mode)
	}
	store, err := sqlite.Open(context.Background(), p.database)
	if err != nil {
		t.Fatal(err)
	}
	mappings, readErr := store.EpisodeMappings(context.Background(), "loop23-demo")
	if err := errors.Join(readErr, store.Close()); err != nil {
		t.Fatal(err)
	}
	if len(mappings) != 1 || mappings[0].EpisodeID != "episode-1" || mappings[0].Ref != (core.EpisodeRef{Anime: core.SourceRef{Provider: "anime1", ID: "42"}, ID: "101"}) {
		t.Fatal(mappings)
	}
}

func TestInvalidInputsAndPreflightLeaveNoProfile(t *testing.T) {
	inputs := []prepareOptions{{CategoryID: "1"}, {PostID: "2"}, {CategoryID: "0", PostID: "2"}, {CategoryID: "12345678901", PostID: "2"}, {CategoryID: "1", PostID: strings.Repeat("9", 21)}, {MPVPath: "relative.exe"}, {CategoryID: "1", PostID: "2"}}
	for _, input := range inputs {
		p := testPaths(t)
		if err := prepare(p, input, func(context.Context, string, string) error { return errors.New("upstream unavailable") }); err == nil {
			t.Fatal("accepted", input)
		}
		if _, err := os.Lstat(p.root); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("failed validation created profile")
		}
		if _, err := os.Lstat(p.guard); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("failed preflight left active guard")
		}
	}
}

func TestResetRejectsUnownedOrChangedState(t *testing.T) {
	cases := map[string]func(*testing.T, profilePaths){
		"missing marker": func(t *testing.T, p profilePaths) {
			if err := os.Remove(filepath.Join(p.root, markerName)); err != nil {
				t.Fatal(err)
			}
		},
		"wrong marker": func(t *testing.T, p profilePaths) { mustWrite(t, filepath.Join(p.root, markerName), []byte("{}")) },
		"mismatched root": func(t *testing.T, p profilePaths) {
			owner, err := validateOwned(p)
			if err != nil {
				t.Fatal(err)
			}
			owner.Root = p.base
			data, err := json.Marshal(owner)
			if err != nil {
				t.Fatal(err)
			}
			mustWrite(t, filepath.Join(p.root, markerName), data)
		},
		"unknown root file": func(t *testing.T, p profilePaths) {
			mustWrite(t, filepath.Join(p.root, "personal.txt"), []byte("keep"))
		},
		"unknown nested file": func(t *testing.T, p profilePaths) {
			mustWrite(t, filepath.Join(p.appdata, "AnimePortable", "personal.txt"), []byte("keep"))
		},
		"unknown sibling directory": func(t *testing.T, p profilePaths) {
			if err := os.Mkdir(filepath.Join(p.appdata, "OtherApp"), 0700); err != nil {
				t.Fatal(err)
			}
		},
		"database changed": func(t *testing.T, p profilePaths) { mustWrite(t, p.database, []byte("changed")) },
		"database missing": func(t *testing.T, p profilePaths) {
			if err := os.Remove(p.database); err != nil {
				t.Fatal(err)
			}
		},
		"database directory": func(t *testing.T, p profilePaths) {
			if err := os.Remove(p.database); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(p.database, 0700); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := testPaths(t)
			mustPrepare(t, p)
			mutate(t, p)
			if err := reset(p); err == nil {
				t.Fatal("unsafe reset succeeded")
			}
			if _, err := os.Lstat(p.root); err != nil {
				t.Fatal("refusal removed root", err)
			}
		})
	}
}

func TestActiveGuardBlocksAllOperations(t *testing.T) {
	p := testPaths(t)
	mustPrepare(t, p)
	guard, err := acquire(p)
	if err != nil {
		t.Fatal(err)
	}
	defer guard.release()
	if err := prepare(p, prepareOptions{}, nil); err == nil {
		t.Fatal("prepare ignored guard")
	}
	if err := reset(p); err == nil {
		t.Fatal("reset ignored guard")
	}
	if err := launch(p, testExecutable(t), func(string, []string) error { t.Fatal("runner started"); return nil }); err == nil {
		t.Fatal("launch ignored guard")
	}
}

func TestCleanLaunchInventoriesRuntimeStateAndResets(t *testing.T) {
	p := testPaths(t)
	mustPrepare(t, p)
	original := os.Getenv("APPDATA")
	exe := testExecutable(t)
	err := launch(p, exe, func(actualExe string, env []string) error {
		if actualExe != exe {
			t.Fatal(actualExe)
		}
		count := 0
		for _, value := range env {
			key, val, _ := strings.Cut(value, "=")
			if strings.EqualFold(key, "APPDATA") {
				count++
				if val != p.appdata {
					t.Fatal(val)
				}
			}
		}
		if count != 1 {
			t.Fatal(count)
		}
		if err := reset(p); err == nil {
			t.Fatal("reset raced launch")
		}
		browser := filepath.Join(p.appdata, "animeportable.exe", "cache")
		if err := os.MkdirAll(browser, 0700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(browser, "data"), []byte("runtime browser cache"), 0600); err != nil {
			return err
		}
		store, err := sqlite.Open(context.Background(), p.database)
		if err != nil {
			return err
		}
		history, err := store.History(context.Background())
		if err != nil {
			_ = store.Close()
			return err
		}
		history[0].Progress.Position += 30 * time.Second
		history[0].Progress.UpdatedAt = history[0].Progress.UpdatedAt.Add(time.Minute)
		history[0].LastPlayedAt = history[0].Progress.UpdatedAt
		return errors.Join(store.SavePlaybackCheckpoint(context.Background(), history[0]), store.Close())
	})
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("APPDATA") != original {
		t.Fatal("parent environment changed")
	}
	owner, err := validateOwned(p)
	if err != nil {
		t.Fatal(err)
	}
	if owner.ExecutableSHA256 == "" {
		t.Fatal("missing executable identity")
	}
	if got := readHistory(t, p)[0].Progress.Position; got != 155500*time.Millisecond {
		t.Fatal(got)
	}
	if err := reset(p); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchFailurePreservesGuard(t *testing.T) {
	p := testPaths(t)
	mustPrepare(t, p)
	if err := launch(p, testExecutable(t), func(string, []string) error { return errors.New("crash") }); err == nil {
		t.Fatal("failure ignored")
	}
	if _, err := os.Lstat(p.guard); err != nil {
		t.Fatal("guard removed", err)
	}
	if err := reset(p); err == nil {
		t.Fatal("reset removed uncertain running state")
	}
}

func TestWrongExecutableAndAlteredPathsRefused(t *testing.T) {
	p := testPaths(t)
	mustPrepare(t, p)
	if err := launch(p, filepath.Join(t.TempDir(), "other.exe"), func(string, []string) error { t.Fatal("launched"); return nil }); err == nil {
		t.Fatal("wrong executable accepted")
	}
	altered := p
	altered.root = p.base
	if err := reset(altered); err == nil {
		t.Fatal("altered root accepted")
	}
	if _, err := newPaths("relative"); err == nil {
		t.Fatal("relative path accepted")
	}
}

func TestLinksRefused(t *testing.T) {
	t.Run("ancestor", func(t *testing.T) {
		base := t.TempDir()
		target := t.TempDir()
		link := filepath.Join(base, "linked")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		p, err := newPaths(link)
		if err != nil {
			t.Fatal(err)
		}
		if err := prepare(p, prepareOptions{}, nil); err == nil {
			t.Fatal("ancestor link followed")
		}
		entries, err := os.ReadDir(target)
		if err != nil || len(entries) != 0 {
			t.Fatal("link target modified", err)
		}
	})
	t.Run("database leaf", func(t *testing.T) {
		p := testPaths(t)
		mustPrepare(t, p)
		outside := filepath.Join(t.TempDir(), "personal")
		mustWrite(t, outside, []byte("keep"))
		if err := os.Remove(p.database); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, p.database); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		if err := reset(p); err == nil {
			t.Fatal("leaf symlink accepted")
		}
		data, err := os.ReadFile(outside)
		if err != nil || string(data) != "keep" {
			t.Fatal("link target changed")
		}
	})
}

func TestIncompletePreparationCannotLaunchOrReset(t *testing.T) {
	p := testPaths(t)
	if err := os.MkdirAll(filepath.Dir(p.database), 0700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, p.database, []byte("incomplete"))
	if err := prepare(p, prepareOptions{}, nil); err == nil {
		t.Fatal("partial profile overwritten")
	}
	if err := reset(p); err == nil {
		t.Fatal("partial profile deleted")
	}
	if err := launch(p, testExecutable(t), func(string, []string) error { t.Fatal("launched partial state"); return nil }); err == nil {
		t.Fatal("partial profile launched")
	}
}

func TestFixtureValidationAndEnvironment(t *testing.T) {
	for _, data := range [][]byte{
		append(append([]byte{}, fixtureJSON...), []byte("{}")...),
		bytes.Replace(fixtureJSON, []byte(`"version":1`), []byte(`"version":2`), 1),
		bytes.Replace(fixtureJSON, []byte(`"positionMs":125500`), []byte(`"positionMs":-1`), 1),
		bytes.Replace(fixtureJSON, []byte(`"durationMs":1440000`), []byte(`"durationMs":9223372036854775807`), 1),
		bytes.Replace(fixtureJSON, []byte("2026-09-12T12:00:00Z"), []byte("invalid"), 1),
	} {
		if _, err := decodeFixture(data); err == nil {
			t.Fatal("invalid fixture accepted")
		}
	}
	for _, value := range []string{"", "0", "01", "a", "12345678901"} {
		if positiveDecimal(value, 10) {
			t.Fatal(value)
		}
	}
	if !positiveDecimal("1", 10) {
		t.Fatal("valid decimal rejected")
	}
	env := []string{"PATH=x", "APPDATA=old", "appdata=old2", "A=y"}
	actual := childEnvironment(env, "new")
	if !reflect.DeepEqual(actual, []string{"PATH=x", "A=y", "APPDATA=new"}) {
		t.Fatal(actual)
	}
	if env[1] != "APPDATA=old" {
		t.Fatal("input mutated")
	}
}
