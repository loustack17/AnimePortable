//go:build windows && amd64

// SPDX-License-Identifier: MPL-2.0

package mpvwin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"
)

type fakeModuleLoader struct {
	loadedPath string
	loadCount  int
	freeCount  int
	missing    string
	onLoad     func(string) error
}

func (loader *fakeModuleLoader) Load(path string) (uintptr, error) {
	loader.loadedPath = path
	loader.loadCount++
	if loader.onLoad != nil {
		if err := loader.onLoad(path); err != nil {
			return 0, err
		}
	}
	return 41, nil
}

func (loader *fakeModuleLoader) Symbol(_ uintptr, name string) (uintptr, error) {
	if name == loader.missing {
		return 0, errors.New("private resolution details")
	}
	return 42, nil
}

func (loader *fakeModuleLoader) Free(uintptr) error {
	loader.freeCount++
	return nil
}

func tempDLL(t *testing.T) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "libmpv-2.dll")
	contents := []byte("test dll content")
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	return path, hex.EncodeToString(digest[:])
}

func TestValidateLibraryRequiresAbsolutePathAndMatchingHash(t *testing.T) {
	path, digest := tempDLL(t)
	_, file, err := validateLibraryFile(path, digest)
	if err != nil {
		t.Fatalf("valid test DLL rejected: %v", err)
	}
	defer file.Close()
	if _, _, err := validateLibraryFile(filepath.Base(path), digest); !errors.Is(err, ErrInvalidLibrary) {
		t.Fatalf("relative path returned %v", err)
	}
	if _, _, err := validateLibraryFile(path, strings.Repeat("0", 64)); !errors.Is(err, ErrLibraryHash) {
		t.Fatalf("mismatched hash returned %v", err)
	}
	if _, _, err := validateLibraryFile(path, "not-a-sha256"); !errors.Is(err, ErrInvalidLibrary) {
		t.Fatalf("invalid expected digest returned %v", err)
	}
	link := filepath.Join(t.TempDir(), "linked.dll")
	if err := os.Symlink(path, link); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if _, _, err := validateLibraryFile(link, digest); !errors.Is(err, ErrInvalidLibrary) {
		t.Fatalf("symbolic link returned %v", err)
	}
}

func TestOpenRejectsMissingSymbolsAndUnloadsPartialLibrary(t *testing.T) {
	path, digest := tempDLL(t)
	loader := &fakeModuleLoader{missing: "mpv_render_context_free"}
	_, err := openLibrary(path, digest, loader)
	if !errors.Is(err, ErrMissingSymbol) || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), path) {
		t.Fatalf("missing symbol error leaked details or returned wrong error: %v", err)
	}
	if loader.loadCount != 1 || loader.freeCount != 1 {
		t.Fatalf("partial module cleanup = loads %d, frees %d", loader.loadCount, loader.freeCount)
	}
}

func TestOpenDoesNotLoadOnHashMismatch(t *testing.T) {
	path, digest := tempDLL(t)
	loader := &fakeModuleLoader{}
	_, err := openLibrary(path, strings.Repeat("0", 64), loader)
	if !errors.Is(err, ErrLibraryHash) {
		t.Fatalf("hash mismatch returned %v", err)
	}
	if loader.loadCount != 0 || loader.freeCount != 0 {
		t.Fatalf("hash mismatch reached loader: loads %d, frees %d (digest %s)", loader.loadCount, loader.freeCount, digest)
	}
}

func TestOpenKeepsVerifiedFileLockedDuringLoad(t *testing.T) {
	path, digest := tempDLL(t)
	loader := &fakeModuleLoader{missing: "mpv_render_context_free"}
	loader.onLoad = func(path string) error {
		if err := os.Rename(path, path+".replaced"); err == nil {
			return errors.New("verified DLL was replaceable during load")
		}
		if err := os.WriteFile(path, []byte("different DLL"), 0600); err == nil {
			return errors.New("verified DLL was writable during load")
		}
		return nil
	}
	if _, err := openLibrary(path, digest, loader); !errors.Is(err, ErrMissingSymbol) {
		t.Fatalf("verified DLL was not held through load: %v", err)
	}
	if err := os.Rename(path, path+".replaced"); err != nil {
		t.Fatalf("DLL lock was not released after load: %v", err)
	}
}

func TestWindowsABIStructLayouts(t *testing.T) {
	if unsafe.Sizeof(renderParam{}) != 16 || unsafe.Offsetof(renderParam{}.Data) != 8 {
		t.Fatalf("render parameter ABI layout: size=%d data=%d", unsafe.Sizeof(renderParam{}), unsafe.Offsetof(renderParam{}.Data))
	}
	if unsafe.Sizeof(glInitParams{}) != 16 || unsafe.Offsetof(glInitParams{}.Context) != 8 {
		t.Fatalf("OpenGL initialization ABI layout: size=%d context=%d", unsafe.Sizeof(glInitParams{}), unsafe.Offsetof(glInitParams{}.Context))
	}
	if unsafe.Sizeof(glFBO{}) != 16 {
		t.Fatalf("OpenGL FBO ABI layout: size=%d", unsafe.Sizeof(glFBO{}))
	}
	if unsafe.Sizeof(rawEvent{}) != 24 || unsafe.Offsetof(rawEvent{}.ReplyUserdata) != 8 || unsafe.Offsetof(rawEvent{}.Data) != 16 {
		t.Fatalf("event ABI layout: size=%d userdata=%d data=%d", unsafe.Sizeof(rawEvent{}), unsafe.Offsetof(rawEvent{}.ReplyUserdata), unsafe.Offsetof(rawEvent{}.Data))
	}
	if unsafe.Sizeof(rawEndFile{}) != 32 || unsafe.Offsetof(rawEndFile{}.EntryID) != 8 || unsafe.Offsetof(rawEndFile{}.InsertNumEntries) != 24 {
		t.Fatalf("end-file ABI layout: size=%d entry=%d insert-count=%d", unsafe.Sizeof(rawEndFile{}), unsafe.Offsetof(rawEndFile{}.EntryID), unsafe.Offsetof(rawEndFile{}.InsertNumEntries))
	}
}

func TestLibraryCannotUnloadWithLiveCore(t *testing.T) {
	loader := &fakeModuleLoader{}
	terminated := 0
	library := &Library{
		handle: 41,
		loader: loader,
		api: api{
			create:           func() uintptr { return 73 },
			terminateDestroy: func(uintptr) { terminated++ },
		},
	}
	player, err := library.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := library.Close(); !errors.Is(err, ErrLibraryInUse) {
		t.Fatalf("close with live player returned %v", err)
	}
	if loader.freeCount != 0 {
		t.Fatal("library unloaded while a player was live")
	}
	if err := player.TerminateDestroy(); err != nil {
		t.Fatal(err)
	}
	if terminated != 1 {
		t.Fatalf("terminate count = %d", terminated)
	}
	if err := library.Close(); err != nil {
		t.Fatal(err)
	}
	if loader.freeCount != 1 {
		t.Fatalf("FreeLibrary count = %d", loader.freeCount)
	}
}

func TestRenderFreeWaitsForCallbackBeforeReleasingContext(t *testing.T) {
	setCallback := 0
	freeRender := 0
	unregistered := make(chan struct{})
	player := &Mpv{renders: 1, library: &Library{api: api{
		renderContextSetUpdateCB: func(uintptr, uintptr, uintptr) {
			setCallback++
			close(unregistered)
		},
		renderContextFree: func(uintptr) { freeRender++ },
	}}}
	context := &RenderContext{id: 927, mpv: player, handle: 16, drained: make(chan struct{})}
	callbackRegistry.Lock()
	callbackRegistry.entries[context.id] = context
	callbackRegistry.Unlock()
	if acquireCallback(uintptr(context.id)) != context {
		t.Fatal("callback entry was not registered")
	}
	freed := make(chan error, 1)
	go func() { freed <- context.Free() }()
	select {
	case <-unregistered:
	case <-time.After(time.Second):
		t.Fatal("render update callback was not unregistered")
	}
	select {
	case err := <-freed:
		t.Fatalf("render context freed before callback drained: %v", err)
	default:
	}
	releaseCallback(context)
	select {
	case err := <-freed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("render context did not free after callback drained")
	}
	if setCallback != 1 || freeRender != 1 || player.renders != 0 {
		t.Fatalf("callback/render cleanup mismatch: unregister=%d free=%d live=%d", setCallback, freeRender, player.renders)
	}
	if acquireCallback(uintptr(context.id)) != nil {
		t.Fatal("callback remained registered after render context free")
	}
}
