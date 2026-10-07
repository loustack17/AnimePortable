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
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"animeportable/apps/desktop/fltkengine"
	mpv "animeportable/apps/desktop/fltkengine/internal/mpvwin"
	"animeportable/apps/desktop/fltkplayer"
	"animeportable/core"
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

func TestPlayerControlsRenderAfterEngineReentry(t *testing.T) {
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	t.Cleanup(func() {
		fltk.Unlock()
		runtime.UnlockOSThread()
	})

	preparePinnedRuntime(t)
	mediaDirectory := t.TempDir()
	media := []string{filepath.Join(mediaDirectory, "first-black.mp4"), filepath.Join(mediaDirectory, "second-black.mp4")}
	for _, path := range media {
		writeBlackVideo(t, path)
	}

	view := fltkplayer.NewWindow(fltkplayer.Callbacks{})
	view.SetEpisodes([]string{"TEST EPISODE"}, 0)
	view.SetState(fltkplayer.State{Playing: true, Volume: 100, Episode: 0, Focus: -1})
	view.Window().Show()
	t.Cleanup(func() {
		view.SetRenderHook(nil)
		view.Close()
		view.Window().Hide()
		view.Window().Destroy()
	})

	var drawCounts []int
	var contexts []uintptr
	var glErrors []uint32
	var engine *fltkengine.Engine
	var seekPosition time.Duration
	diagnostic := func() string {
		return "drawCounts=" + intsString(drawCounts) + " contextIDs=" + uintptrsString(contexts) + " glErrors=" + uintsString(glErrors)
	}

	for pass := 0; pass < 2; pass++ {
		var err error
		newContext, cancelNew := context.WithTimeout(context.Background(), 12*time.Second)
		engine, err = pumpFLTKResult(t, 15*time.Second, func() (*fltkengine.Engine, error) {
			return fltkengine.New(newContext, view.Video())
		})
		cancelNew()
		if err != nil {
			t.Fatalf("pass%d: create engine: %v; %s; %s", pass+1, err, diagnostic(), diagnoseEngineCreation(view.Video()))
		}
		t.Cleanup(func() {
			if engine != nil {
				if err := pumpFLTKError(t, 15*time.Second, engine.Close); err != nil {
					t.Errorf("cleanup engine: %v; %s", err, diagnostic())
				}
				engine = nil
			}
		})
		drawCount := 0
		mediaReady := false
		readyDraws := 0
		engine.SetStateHandler(func(position, duration time.Duration, paused bool, resolution int) {
			mediaReady = duration >= 2900*time.Millisecond && duration <= 3100*time.Millisecond && resolution == 180
		})
		var drawContext uintptr
		view.SetRenderHook(func(width, height int) {
			drawCount++
			drawContext = currentGLContext()
			if renderErr := engine.Render(width, height); renderErr != nil {
				glErrors = append(glErrors, ^uint32(0))
			}
			if mediaReady {
				readyDraws++
			}
		})
		view.SetState(fltkplayer.State{Playing: true, Volume: 100, Episode: 0, Focus: 1})
		view.SetFocus(1)
		loadContext, cancelLoad := context.WithTimeout(context.Background(), 8*time.Second)
		if err := pumpFLTKError(t, 10*time.Second, func() error {
			return engine.Load(loadContext, media[pass], 0, uint64(pass+1))
		}); err != nil {
			cancelLoad()
			t.Fatalf("pass%d: load synthetic video: %v; %s", pass+1, err, diagnostic())
		}
		cancelLoad()

		if !pumpUntil(t, 8*time.Second, func() bool { return mediaReady && readyDraws > 0 && drawCount >= 3 }) {
			drawCounts = append(drawCounts, drawCount)
			contexts = append(contexts, drawContext)
			t.Fatalf("pass%d: mediaReady=%t, readyDraws=%d; timed out waiting for actual media rendering; %s", pass+1, mediaReady, readyDraws, diagnostic())
		}
		drawCounts = append(drawCounts, drawCount)
		contexts = append(contexts, drawContext)
		view.Video().MakeCurrent()
		glErrors = append(glErrors, drainGLErrors())
		if glErrors[len(glErrors)-1] != 0 {
			t.Fatalf("pass%d: OpenGL reported an error after rendering; %s", pass+1, diagnostic())
		}
		glyphErr := assertVisibleControlGlyphs(view.Video().W(), view.Video().H())
		glErrors = append(glErrors, drainGLErrors())
		if glErrors[len(glErrors)-1] != 0 {
			t.Fatalf("pass%d: OpenGL reported an error while sampling the swapped front buffer; %s", pass+1, diagnostic())
		}
		if glyphErr != nil {
			t.Fatalf("pass%d: player control glyphs are missing from the swapped GL front buffer: %v; %s", pass+1, glyphErr, diagnostic())
		}
		if drawContext == 0 {
			t.Fatalf("pass%d: render hook had no current GL context; %s", pass+1, diagnostic())
		}
		if pass == 0 {
			if err := pumpFLTKError(t, 10*time.Second, func() error { return engine.Control(context.Background(), "seek_absolute", 2) }); err != nil {
				t.Fatal(err)
			}
			snapshot, err := pumpFLTKResult(t, 10*time.Second, func() (core.PlaybackSnapshot, error) { return engine.Snapshot(context.Background()) })
			if err != nil {
				t.Fatal(err)
			}
			seekPosition = snapshot.Position
		}

		if err := pumpFLTKError(t, 15*time.Second, engine.Close); err != nil {
			t.Fatalf("pass%d: close engine: %v; %s", pass+1, err, diagnostic())
		}
		engine = nil
		view.SetRenderHook(nil)
		view.Window().Hide()
		fltk.Wait(0.05)
		view.Window().Show()
		view.Video().Redraw()
		if !pumpUntil(t, 5*time.Second, func() bool { return view.Window().IsShown() }) {
			t.Fatalf("pass%d: player parent did not show again; %s", pass+1, diagnostic())
		}
	}
	if seekPosition < 1500*time.Millisecond || seekPosition > 2500*time.Millisecond {
		t.Fatalf("immediate seek checkpoint position=%s, want near2s", seekPosition)
	}
}

func diagnoseEngineCreation(video *fltk.GlWindow) string {
	video.MakeCurrent()
	version := callGL("glGetString", 0x1F02)
	var versionText []byte
	if version != 0 {
		for index := uintptr(0); index < 256; index++ {
			value := *(*byte)(unsafe.Pointer(version + index))
			if value == 0 {
				break
			}
			versionText = append(versionText, value)
		}
	}
	prefix := fmt.Sprintf("GL version=%q context=%#x", versionText, currentGLContext())
	library, err := mpv.Open(os.Getenv("ANIMEPORTABLE_LIBMPV_OVERRIDE"), pinnedDLLHash)
	if err != nil {
		return fmt.Sprintf("%s library load: %v", prefix, err)
	}
	defer library.Close()
	player, err := library.New()
	if err != nil {
		return fmt.Sprintf("%s mpv create: %v", prefix, err)
	}
	defer player.TerminateDestroy()
	for _, option := range [][2]string{{"config", "no"}, {"vo", "libmpv"}, {"hwdec", "auto"}, {"demuxer-max-bytes", "16MiB"}, {"cache-secs", "5"}} {
		if err := player.SetOptionString(option[0], option[1]); err != nil {
			return fmt.Sprintf("%s option %s: %v", prefix, option[0], err)
		}
	}
	if err := player.Initialize(); err != nil {
		return fmt.Sprintf("%s mpv initialize: %v", prefix, err)
	}
	render, err := player.NewRenderContextGL(func(name string) uintptr {
		value, _ := syscall.BytePtrFromString(name)
		address, _, _ := playerReentryGL.NewProc("wglGetProcAddress").Call(uintptr(unsafe.Pointer(value)))
		if address > 3 && address != ^uintptr(0) {
			return address
		}
		proc := playerReentryGL.NewProc(name)
		if proc.Find() != nil {
			return 0
		}
		return proc.Addr()
	})
	if err != nil {
		return fmt.Sprintf("%s mpv render context: %v", prefix, err)
	}
	if err := render.Free(); err != nil {
		return fmt.Sprintf("%s render cleanup: %v", prefix, err)
	}
	return prefix + " independent creation succeeded"
}

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

func writeBlackVideo(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "black-3s.mp4"))
	if err != nil {
		t.Fatalf("read three-second H264 fixture: %v", err)
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

func currentGLContext() uintptr {
	context, _, _ := playerReentryGL.NewProc("wglGetCurrentContext").Call()
	return context
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

func intsString(values []int) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.Itoa(value)
	}
	return strings.Join(parts, ",")
}

func uintptrsString(values []uintptr) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = fmt.Sprintf("0x%x", value)
	}
	return strings.Join(parts, ",")
}

func uintsString(values []uint32) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = fmt.Sprintf("0x%x", value)
	}
	return strings.Join(parts, ",")
}
