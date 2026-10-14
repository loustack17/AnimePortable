//go:build windows && amd64 && cgo

package fltkengine_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"

	"animeportable/apps/desktop/fltkengine"
	"animeportable/apps/desktop/fltkplayer"
	fltk "github.com/pwiecz/go-fltk"
)

// TestPlaybackSmoothnessBench is an opt-in measurement tool: it plays a locally generated
// 23.976 fps clip through the real engine and window and records render/decode cadence.
// It asserts only that playback starts and the engine does not error.
func TestPlaybackSmoothnessBench(t *testing.T) {
	if os.Getenv("ANIMEPORTABLE_PLAYBACK_BENCH") != "1" {
		t.Skip("set ANIMEPORTABLE_PLAYBACK_BENCH=1 to run the native playback smoothness benchmark")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not on PATH; cannot generate the benchmark clip")
	}
	benchSeconds := 12
	if value := os.Getenv("BENCH_SECONDS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 18 {
			t.Fatalf("BENCH_SECONDS must be 1..18 (clip is 20s), got %q", value)
		}
		benchSeconds = parsed
	}

	clip := filepath.Join(t.TempDir(), "clip.mp4")
	generate := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=24000/1001",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
		"-t", "20", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-g", "48", "-c:a", "aac", "-shortest", clip)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate clip with ffmpeg: %v: %s", err, bytes.TrimSpace(output))
	}

	source := os.Getenv("ANIMEPORTABLE_BENCH_SOURCE")
	switch source {
	case "":
		source = "file"
	case "file", "http":
	case "proxy":
		// adapters/playback/proxy only opens public https origins through securehttp, which rejects
		// loopback and private addresses, and its only test seam (serverDeps) is unexported.
		t.Skip("proxy source needs a production test seam; use file or http")
	default:
		t.Fatalf("ANIMEPORTABLE_BENCH_SOURCE must be file|http|proxy, got %q", source)
	}
	rateKBps := 0
	if value := os.Getenv("ANIMEPORTABLE_BENCH_RATE_KBPS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			t.Fatalf("ANIMEPORTABLE_BENCH_RATE_KBPS must be a positive integer, got %q", value)
		}
		if source == "file" {
			t.Fatal("ANIMEPORTABLE_BENCH_RATE_KBPS needs ANIMEPORTABLE_BENCH_SOURCE=http")
		}
		rateKBps = parsed
	}
	overlay := os.Getenv("ANIMEPORTABLE_BENCH_OVERLAY") == "1"
	mediaLocation := clip
	if source == "http" {
		mediaLocation = serveBenchClip(t, clip, rateKBps)
	}

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
	t.Setenv("ANIMEPORTABLE_PLAYBACK_DIAGNOSTICS", "1")
	// The engine writes both files next to the executable, so claim them before and consume them after.
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	reportPath := filepath.Join(filepath.Dir(executable), "playback-diagnostics.json")
	_ = os.Remove(reportPath)
	t.Cleanup(func() { _ = os.Remove(reportPath) })

	view := fltkplayer.NewWindow(fltkplayer.Callbacks{})
	view.SetEpisodes([]string{"BENCH"}, 0)
	view.SetState(fltkplayer.State{Playing: true, Volume: 0, Episode: 0, Focus: fltkplayer.FocusProgress})
	view.Window().Resize(100, 100, 1000, 562)
	view.Window().Show()
	t.Cleanup(func() {
		view.SetRenderHook(nil)
		view.Close()
		view.Window().Hide()
		view.Window().Destroy()
	})

	newContext, cancelNew := context.WithTimeout(context.Background(), 12*time.Second)
	engine, err := pumpFLTKResult(t, 15*time.Second, func() (*fltkengine.Engine, error) {
		return fltkengine.New(newContext, view.Video())
	})
	cancelNew()
	if err != nil {
		t.Fatalf("create engine: %s: %v", diagnoseEngineCreation(view.Video()), err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = pumpFLTKError(t, 15*time.Second, engine.Close)
		}
	})

	draws := 0
	playing := false
	engine.SetStateHandler(func(position, duration time.Duration, paused bool, resolution int) {
		playing = playing || (duration > 0 && !paused && position > 0)
	})
	view.SetRenderHook(func(width, height int) {
		draws++
		if err := engine.Render(width, height); err != nil {
			t.Errorf("engine render: %v", err)
		}
	})

	loadContext, cancelLoad := context.WithTimeout(context.Background(), 8*time.Second)
	err = pumpFLTKError(t, 10*time.Second, func() error { return engine.Load(loadContext, mediaLocation, 0, 1) })
	cancelLoad()
	if err != nil {
		t.Fatalf("load clip: %v", err)
	}
	if !pumpUntil(t, 10*time.Second, func() bool { return playing && draws >= 3 }) {
		t.Fatalf("playback never started: playing=%t draws=%d", playing, draws)
	}

	started := time.Now()
	nextMotion := started
	motionX := 480
	pumpUntil(t, time.Duration(benchSeconds)*time.Second, func() bool {
		if overlay && !time.Now().Before(nextMotion) {
			// Alternating coordinates make every message a real pointer move, which keeps the controls shown.
			motionX = 1000 - motionX
			sendBenchPointerMove(view.Video().RawHandle(), motionX, 200)
			nextMotion = time.Now().Add(500 * time.Millisecond)
		}
		return false
	})
	elapsed := time.Since(started)

	if err := pumpFLTKError(t, 15*time.Second, engine.Close); err != nil {
		t.Fatalf("close engine: %v", err)
	}
	closed = true

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read playback diagnostics: %v", err)
	}
	var report struct {
		MPVUpdateCallbacks     uint64 `json:"mpv_update_callbacks"`
		RenderCalls            uint64 `json:"render_calls"`
		RenderUpdateFrames     uint64 `json:"render_update_frames"`
		RenderIntervalMaxNanos uint64 `json:"render_interval_max_nanos"`
		MPV                    struct {
			FrameDropCount *uint64 `json:"frame_drop_count"`
		} `json:"mpv"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode playback diagnostics: %v", err)
	}

	seconds := elapsed.Seconds()
	// The report counts from engine creation, so rates are over that whole window and the
	// clip's pre-roll is included; compare runs, not absolute values.
	summary := map[string]any{
		"source":               source,
		"overlay":              overlay,
		"rate_kbps":            rateKBps,
		"seconds":              seconds,
		"mpv_updates_per_s":    float64(report.MPVUpdateCallbacks) / seconds,
		"renders_per_s":        float64(report.RenderCalls) / seconds,
		"new_frames_per_s":     float64(report.RenderUpdateFrames) / seconds,
		"frame_drop_count":     report.MPV.FrameDropCount,
		"max_render_gap_ms":    float64(report.RenderIntervalMaxNanos) / float64(time.Millisecond),
		"mpv_update_callbacks": report.MPVUpdateCallbacks,
		"render_calls":         report.RenderCalls,
		"render_update_frames": report.RenderUpdateFrames,
	}
	line, _ := json.Marshal(summary)
	t.Logf("bench summary: %s", line)

	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	outputDirectory := filepath.Join(repoRoot, "artifacts", "cache", "bench")
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		t.Fatalf("create bench output directory: %v", err)
	}
	pretty, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(filepath.Join(outputDirectory, "last.json"), pretty, 0o644); err != nil {
		t.Fatalf("write bench summary: %v", err)
	}
	_ = os.WriteFile(filepath.Join(outputDirectory, "last-diagnostics.json"), data, 0o644)
}

var benchUser32 = syscall.NewLazyDLL("user32.dll")

func sendBenchPointerMove(hwnd uintptr, x, y int) {
	const wmMouseMove = 0x0200
	benchUser32.NewProc("SendMessageW").Call(hwnd, wmMouseMove, 0, uintptr(y<<16|x&0xFFFF))
}

// serveBenchClip serves the clip over loopback HTTP with Range support, optionally limited to rateKBps.
func serveBenchClip(t *testing.T, clip string, rateKBps int) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for bench clip: %v", err)
	}
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		file, err := os.Open(clip)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()
		var content io.ReadSeeker = file
		if rateKBps > 0 {
			content = &throttledReader{ReadSeeker: file, bytesPerSecond: rateKBps * 1000 / 8}
		}
		writer.Header().Set("Content-Type", "video/mp4")
		http.ServeContent(writer, request, "clip.mp4", time.Time{}, content)
	})
	server := &http.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	return fmt.Sprintf("http://%s/clip.mp4", listener.Addr())
}

// throttledReader paces reads so a response approximates a constant-rate connection.
type throttledReader struct {
	io.ReadSeeker
	bytesPerSecond int
}

func (reader *throttledReader) Read(buffer []byte) (int, error) {
	chunk := reader.bytesPerSecond / 50
	if chunk < 1 {
		chunk = 1
	}
	if len(buffer) > chunk {
		buffer = buffer[:chunk]
	}
	count, err := reader.ReadSeeker.Read(buffer)
	time.Sleep(time.Duration(count) * time.Second / time.Duration(reader.bytesPerSecond))
	return count, err
}
