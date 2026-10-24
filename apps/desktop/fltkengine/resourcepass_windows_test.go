//go:build windows && amd64 && cgo && resourcepass

// SPDX-License-Identifier: MPL-2.0

package fltkengine_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"animeportable/apps/desktop/fltkengine"
	mpv "animeportable/apps/desktop/fltkengine/internal/mpvwin"
	"animeportable/internal/resourcepass"
)

// Each cycle is one player session in real libmpv, as the app runs it but without a window:
// load the pinned DLL, start mpv with the app's options, play one episode until its first
// frame, switch to a second one, then destroy mpv and release the DLL. Private bytes include
// the memory libmpv and FFmpeg hold in C. Their heaps grow in steps for the first hundred or so
// sessions before they level off (Loop 38b evidence), so this scenario runs 2000 cycles, about
// 18 minutes, to tell a level from a trend.
func TestResourcePassRealLibmpvOpenSwitchClose(t *testing.T) {
	preparePinnedRuntime(t)
	media, err := os.ReadFile(filepath.Join("testdata", "black-60s.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	var server *httptest.Server
	playUntilFirstFrame := func(player *mpv.Mpv, url string) {
		if err := player.Command([]string{"loadfile", url}); err != nil {
			t.Fatal(err)
		}
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			event := player.WaitEvent(0.2)
			if event == nil {
				continue
			}
			switch event.EventID {
			case mpv.EventPlaybackRestart:
				return
			case mpv.EventEnd:
				if event.EndFile().Reason == mpv.EndFileError {
					t.Fatal("mpv failed to load the fixture")
				}
			}
		}
		t.Fatal("mpv never showed the first frame")
	}
	resourcepass.Run(t, "real-libmpv-open-switch-close", 2000, func() {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "video/mp4")
			http.ServeContent(w, r, "media.mp4", time.Time{}, bytes.NewReader(media))
		}))
	}, func(int) {
		library, err := mpv.Open(os.Getenv("ANIMEPORTABLE_LIBMPV_OVERRIDE"), pinnedDLLHash)
		if err != nil {
			t.Fatal(err)
		}
		player, err := library.New()
		if err != nil {
			t.Fatal(err)
		}
		options := append(fltkengine.InitOptions(fltkengine.Options{HardwareDecoding: true}), [2]string{"ao", "null"})
		headless := false
		for _, option := range options {
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
		playUntilFirstFrame(player, server.URL+"/episode-1")
		playUntilFirstFrame(player, server.URL+"/episode-2")
		if err := player.TerminateDestroy(); err != nil {
			t.Fatal(err)
		}
		if err := library.Close(); err != nil {
			t.Fatal(err)
		}
	}, func() {
		server.CloseClientConnections()
		server.Close()
	})
}
