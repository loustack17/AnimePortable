//go:build windows && amd64 && cgo

package fltkengine_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"animeportable/apps/desktop/fltkengine"
	mpv "animeportable/apps/desktop/fltkengine/internal/mpvwin"
	fltk "github.com/pwiecz/go-fltk"
)

const (
	pinnedRuntimeURL  = "https://github.com/Predidit/libmpv-win32-video-cmake/releases/download/20260811/mpv-dev-x86_64-20260811-git-ad59ff1.7z"
	pinnedArchiveHash = "0161ad026f9ebd418a9b660c8e8c569f7959b5bb8e5f4a63e462ffc73327d4d8"
	pinnedDLLHash     = "24e848f59c047c9442501fdbe619ad39b98be7d4dd402691f79931c852c0070a"
	maxArchiveBytes   = 16 << 20
	glFrontBuffer     = 0x0404
	glRGBA            = 0x1908
	glUnsignedByte    = 0x1401
)

var playerReentryGL = syscall.NewLazyDLL("opengl32.dll")

func preparePinnedRuntime(t *testing.T) {
	t.Helper()
	dll, err := filepath.Abs(filepath.Join("..", "..", "..", "artifacts", "runtime", "libmpv-2.dll"))
	if err != nil {
		t.Fatalf("resolve local pinned runtime path: %v", err)
	}
	if data, err := os.ReadFile(dll); err != nil || hashBytes(data) != pinnedDLLHash {
		dll = downloadPinnedRuntime(t)
	}
	t.Setenv("ANIMEPORTABLE_LIBMPV_OVERRIDE", dll)
	t.Setenv("ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256", pinnedDLLHash)
}

// newHeadlessMpv starts an mpv core with the app's options but no video or audio output, so
// the test opens no window.
func newHeadlessMpv(t *testing.T, library *mpv.Library, options fltkengine.Options) *mpv.Mpv {
	t.Helper()
	player, err := library.New()
	if err != nil {
		t.Fatal(err)
	}
	headless := false
	for _, option := range append(fltkengine.InitOptions(options), [2]string{"ao", "null"}) {
		if option[0] == "vo" {
			option[1], headless = "null", true
		}
		if err := player.SetOptionString(option[0], option[1]); err != nil {
			t.Fatalf("option %s: %v", option[0], err)
		}
	}
	if !headless {
		t.Fatal("InitOptions has no vo option to replace; mpv would open a window")
	}
	if err := player.Initialize(); err != nil {
		t.Fatal(err)
	}
	return player
}

// waitForMpvEvent hands mpv's events to done until it returns true, and reports whether
// that happened before timeout.
func waitForMpvEvent(player *mpv.Mpv, timeout time.Duration, done func(*mpv.Event) bool) bool {
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		if event := player.WaitEvent(0.2); event != nil && done(event) {
			return true
		}
	}
	return false
}

func downloadPinnedRuntime(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pinnedRuntimeURL, nil)
	if err != nil {
		t.Fatalf("create pinned runtime request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("download approved pinned runtime archive: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("download approved pinned runtime archive: HTTP %s", response.Status)
	}
	if response.ContentLength > maxArchiveBytes {
		t.Fatalf("approved runtime archive exceeds %d bytes: %d", maxArchiveBytes, response.ContentLength)
	}
	archive, err := io.ReadAll(io.LimitReader(response.Body, maxArchiveBytes+1))
	if err != nil {
		t.Fatalf("read approved runtime archive: %v", err)
	}
	if len(archive) == 0 || len(archive) > maxArchiveBytes {
		t.Fatalf("approved runtime archive size is outside the 1..%d byte bound: %d", maxArchiveBytes, len(archive))
	}
	if got := hashBytes(archive); got != pinnedArchiveHash {
		t.Fatalf("approved runtime archive hash mismatch: got %s", got)
	}

	sevenZip := findSevenZip()
	if sevenZip == "" {
		t.Fatal("7z.exe is unavailable; refusing to extract the pinned runtime")
	}
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "runtime.7z")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("write approved runtime archive to test temp directory: %v", err)
	}
	dll := filepath.Join(directory, "libmpv-2.dll")
	cmdCtx, cmdCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cmdCancel()
	command := exec.CommandContext(cmdCtx, sevenZip, "e", archivePath, "libmpv-2.dll", "-o"+directory, "-y")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("extract only pinned libmpv-2.dll with 7z: %v: %s", err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(dll)
	if err != nil {
		t.Fatalf("read extracted pinned libmpv-2.dll: %v", err)
	}
	if got := hashBytes(data); got != pinnedDLLHash {
		t.Fatalf("extracted pinned libmpv-2.dll hash mismatch: got %s", got)
	}
	return dll
}

func findSevenZip() string {
	if path, err := exec.LookPath("7z.exe"); err == nil {
		return path
	}
	for _, path := range []string{
		`C:\Program Files\7-Zip\7z.exe`,
		`C:\Program Files (x86)\7-Zip\7z.exe`,
	} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return ""
}

// writeBlackVideo copies a fixture longer than the sum of the timeouts a pass spends playing
// before its checks, so the file cannot reach EOF first. At EOF mpv unloads the file: the
// seek fails and the duration readiness check turns false.
func writeBlackVideo(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "black-60s.mp4"))
	if err != nil {
		t.Fatalf("read sixty-second H264 fixture: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write black video: %v", err)
	}
}
func pumpFLTKResult[T any](t *testing.T, timeout time.Duration, action func() (T, error)) (T, error) {
	t.Helper()
	type result struct {
		value T
		err   error
	}
	completed := make(chan result, 1)
	go func() {
		value, err := action()
		completed <- result{value: value, err: err}
	}()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case result := <-completed:
			return result.value, result.err
		default:
			fltk.Wait(0.01)
		}
	}
	var zero T
	t.Fatalf("timed out after %s while pumping FLTK for an engine operation", timeout)
	return zero, context.DeadlineExceeded
}

func pumpFLTKError(t *testing.T, timeout time.Duration, action func() error) error {
	t.Helper()
	_, err := pumpFLTKResult(t, timeout, func() (struct{}, error) { return struct{}{}, action() })
	return err
}

func pumpUntil(t *testing.T, timeout time.Duration, condition func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		fltk.Wait(0.01)
	}
	return condition()
}

func assertVisibleControlGlyphs(width, height int) error {
	callGL("glReadBuffer", glFrontBuffer)
	callGL("glFinish")
	if err := findBrightGlyphPixels(width, height, width-185, height-48, width-95, height-16); err != nil {
		return err
	}
	return findBrightGlyphPixels(width, height, 32, 5, 65, 45)
}

func findBrightGlyphPixels(framebufferWidth, framebufferHeight, left, top, right, bottom int) error {
	width := right - left
	height := bottom - top
	pixels := make([]byte, width*height*4)
	callGL("glReadPixels", uintptr(left), uintptr(top), uintptr(width), uintptr(height), glRGBA, glUnsignedByte, uintptr(unsafe.Pointer(&pixels[0])))
	bright := 0
	for index := 0; index < len(pixels); index += 4 {
		if pixels[index] >= 180 && pixels[index+1] >= 180 && pixels[index+2] >= 180 && pixels[index+3] >= 200 {
			bright++
		}
	}
	if bright < 4 {
		return fmt.Errorf("glyph sample at (%d,%d)-(%d,%d) has only %d bright pixels (framebuffer %dx%d)", left, top, right, bottom, bright, framebufferWidth, framebufferHeight)
	}
	return nil
}

func drainGLErrors() uint32 {
	const noError = 0
	const maxErrors = 16
	var first uint32
	for range maxErrors {
		value, _, _ := playerReentryGL.NewProc("glGetError").Call()
		if value == noError {
			return first
		}
		if first == noError {
			first = uint32(value)
		}
	}
	return first
}

func callGL(name string, args ...uintptr) uintptr {
	value, _, _ := playerReentryGL.NewProc(name).Call(args...)
	return value
}

func hashBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
