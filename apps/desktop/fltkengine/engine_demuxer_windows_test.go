//go:build windows && amd64 && cgo

package fltkengine_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"animeportable/apps/desktop/fltkengine"
	mpv "animeportable/apps/desktop/fltkengine/internal/mpvwin"
)

// A compromised CDN can label any body video/mp4, and the proxy passes it on. If mpv opened
// such a body as a playlist, EDL or HLS manifest, it would fetch the addresses listed in it
// itself, past the proxy's origin and address checks. Each disguised body here lists a local
// server that must never be contacted; a real MP4 must still open. Runs headless (no video or
// audio output) with the pinned DLL.
func TestEngineOpensOnlyMP4BodiesNeverListedAddresses(t *testing.T) {
	preparePinnedRuntime(t)
	media, err := os.ReadFile(filepath.Join("testdata", "black-60s.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	var contacted atomic.Int32
	listed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contacted.Add(1)
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(media)
	}))
	defer listed.Close()
	bodies := map[string]string{
		"m3u":  "#EXTM3U\n#EXTINF:10,x\n" + listed.URL + "/m3u.mp4\n",
		"pls":  "[playlist]\nFile1=" + listed.URL + "/pls.mp4\nNumberOfEntries=1\n",
		"edl":  "# mpv EDL v0\n" + listed.URL + "/edl.mp4,0,5\n",
		"hls":  "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:10\n#EXTINF:10.0,\n" + listed.URL + "/segment.ts\n#EXT-X-ENDLIST\n",
		"real": string(media),
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		http.ServeContent(w, r, "media", time.Time{}, strings.NewReader(bodies[strings.TrimPrefix(r.URL.Path, "/")]))
	}))
	defer upstream.Close()

	library, err := mpv.Open(os.Getenv("ANIMEPORTABLE_LIBMPV_OVERRIDE"), pinnedDLLHash)
	if err != nil {
		t.Fatal(err)
	}
	defer library.Close()
	for _, name := range []string{"m3u", "pls", "edl", "hls", "real"} {
		player := newHeadlessMpv(t, library, fltkengine.Options{})
		if err := player.Command([]string{"loadfile", upstream.URL + "/" + name}); err != nil {
			t.Fatal(err)
		}
		loaded := false
		waitForMpvEvent(player, 8*time.Second, func(event *mpv.Event) bool {
			loaded = event.EventID == mpv.EventFileLoaded
			return loaded || event.EventID == mpv.EventEnd
		})
		// A listed entry would be opened after the first file ends, so give it time to show.
		time.Sleep(500 * time.Millisecond)
		_ = player.TerminateDestroy()
		if got := contacted.Load(); got != 0 {
			t.Fatalf("%s: mpv contacted an address listed in the body %d times", name, got)
		}
		if want := name == "real"; loaded != want {
			t.Fatalf("%s: file loaded = %t, want %t", name, loaded, want)
		}
	}
}
