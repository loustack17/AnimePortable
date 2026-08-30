package mpv

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"animeportable/core"
)

const liveMediaCount = 3

type liveProxyService struct {
	urls   []string
	active [liveMediaCount]atomic.Bool

	mu     sync.Mutex
	next   int
	closed bool
}

type liveProxyCapability struct {
	service *liveProxyService
	index   int
	once    sync.Once
}

func newLiveProxyService(urls []string) *liveProxyService {
	return &liveProxyService{urls: append([]string(nil), urls...)}
}

func (service *liveProxyService) NewSession(core.PlaybackSource) (proxyCapability, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.next >= len(service.urls) || service.next >= liveMediaCount {
		return nil, ErrPlayerFailed
	}
	index := service.next
	service.next++
	service.active[index].Store(true)
	return &liveProxyCapability{service: service, index: index}, nil
}

func (service *liveProxyService) Close() error {
	service.mu.Lock()
	service.closed = true
	service.mu.Unlock()
	for index := range service.active {
		service.active[index].Store(false)
	}
	return nil
}

func (service *liveProxyService) isActive(index int) bool {
	return index >= 0 && index < liveMediaCount && service.active[index].Load()
}

func (capability *liveProxyCapability) URL() string {
	if capability == nil || capability.service == nil || capability.index >= len(capability.service.urls) {
		return ""
	}
	return capability.service.urls[capability.index]
}

func (capability *liveProxyCapability) Close() error {
	if capability == nil || capability.service == nil {
		return nil
	}
	capability.once.Do(func() {
		capability.service.active[capability.index].Store(false)
	})
	return nil
}

func TestLiveMPVLoadsThreeMediaURLsOnOneProcess(t *testing.T) {
	if os.Getenv("ANIMEPORTABLE_MPV_LIVE") != "1" {
		t.Skip("live MPV smoke disabled")
	}

	executable, err := Find("")
	if err != nil {
		t.Skipf("MPV unavailable: %v", err)
	}

	paths := liveMediaPaths()
	routes := make(map[string]int, len(paths))
	for index, path := range paths {
		routes[path] = index
	}
	service := newLiveProxyService(paths)
	media := shortWAV()
	var served [liveMediaCount]atomic.Uint32
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		index, ok := routes[request.URL.EscapedPath()]
		if !ok || !service.isActive(index) {
			http.NotFound(response, request)
			return
		}
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		served[index].Add(1)
		response.Header().Set("Content-Type", "audio/wav")
		http.ServeContent(response, request, "media.wav", time.Unix(0, 0), bytes.NewReader(media))
	})

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal("local media listener unavailable")
	}
	server := httptest.NewUnstartedServer(handler)
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	serverClosed := false
	defer func() {
		if !serverClosed {
			server.Close()
		}
	}()
	for index := range paths {
		paths[index] = server.URL + paths[index]
	}
	service.urls = append([]string(nil), paths...)

	var rawSession *Session
	player := newPlayer(executable, playerDeps{
		startRaw: func(ctx context.Context, executable Executable) (rawPlaybackSession, error) {
			var err error
			rawSession, err = StartIPC(ctx, executable)
			return rawSession, err
		},
		newProxy:    func() (proxyService, error) { return service, nil },
		loadTimeout: 20 * time.Second,
	})

	playContext, cancelPlay := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelPlay()
	session, err := player.Start(playContext, liveRequest("ep01"))
	if err != nil {
		t.Fatal("unable to start MPV playback")
	}
	playback := session.(*playbackSession)
	sessionClosed := false
	defer func() {
		if !sessionClosed {
			_ = session.Close()
		}
	}()

	if rawSession == nil || rawSession.Process() == nil || playback.PID() <= 0 {
		t.Fatal("MPV process identity unavailable")
	}
	process := rawSession.Process()
	expectedPID := playback.PID()
	if processDone(process.Done()) {
		t.Fatal("MPV process exited before media switching")
	}
	assertLiveStatus(t, paths[0], 200, 0)

	if err := playback.Load(playContext, liveRequest("ep02")); err != nil {
		t.Fatal("episode 2 load failed")
	}
	assertLiveStatus(t, paths[0], 404, 0)
	assertLiveStatus(t, paths[1], 200, 1)
	assertLiveProcess(t, playback, process, expectedPID, 2)

	if err := playback.Load(playContext, liveRequest("ep03")); err != nil {
		t.Fatal("episode 3 load failed")
	}
	assertLiveStatus(t, paths[1], 404, 1)
	assertLiveStatus(t, paths[2], 200, 2)
	assertLiveProcess(t, playback, process, expectedPID, 3)

	if err := session.Close(); err != nil {
		t.Fatal("MPV session cleanup failed")
	}
	sessionClosed = true
	select {
	case <-process.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("MPV process did not exit after close")
	}
	if err := process.Wait(); err != nil {
		t.Fatal("MPV process wait failed")
	}
	for index, path := range paths {
		assertLiveStatus(t, path, 404, index)
		if served[index].Load() == 0 {
			t.Fatalf("media %d was not served", index+1)
		}
	}

	server.Close()
	serverClosed = true
}

func liveMediaPaths() []string {
	paths := make([]string, liveMediaCount)
	for index := range paths {
		var token [32]byte
		for offset := range token {
			token[offset] = byte(index + 1)
		}
		paths[index] = "/media/" + base64.RawURLEncoding.EncodeToString(token[:])
	}
	return paths
}

func liveRequest(episode string) core.PlayRequest {
	return core.PlayRequest{
		AnimeID:   core.AnimeID("live-anime"),
		EpisodeID: core.EpisodeID(episode),
		Source:    core.NewPlaybackSource("http://127.0.0.1/source", nil),
	}
}

func assertLiveStatus(t *testing.T, mediaURL string, want, index int) {
	t.Helper()
	requestContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, mediaURL, nil)
	if err != nil {
		t.Fatalf("media %d status request could not be built", index+1)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("media %d status request failed", index+1)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != want {
		t.Fatalf("media %d status = %d, want %d", index+1, response.StatusCode, want)
	}
}

func assertLiveProcess(t *testing.T, playback *playbackSession, process *Process, expectedPID, episode int) {
	t.Helper()
	if playback.PID() != expectedPID {
		t.Fatalf("MPV process changed during media %d", episode)
	}
	if processDone(process.Done()) {
		t.Fatalf("MPV process exited during media %d", episode)
	}
}

func processDone(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func shortWAV() []byte {
	const (
		sampleRate = 8000
		channels   = 1
		bits       = 16
		samples    = sampleRate / 4
		dataSize   = samples * channels * bits / 8
	)

	buffer := bytes.NewBuffer(make([]byte, 0, 44+dataSize))
	writeString := func(value string) { _, _ = buffer.WriteString(value) }
	writeUint32 := func(value uint32) { _ = binary.Write(buffer, binary.LittleEndian, value) }
	writeUint16 := func(value uint16) { _ = binary.Write(buffer, binary.LittleEndian, value) }

	writeString("RIFF")
	writeUint32(uint32(36 + dataSize))
	writeString("WAVE")
	writeString("fmt ")
	writeUint32(16)
	writeUint16(1)
	writeUint16(channels)
	writeUint32(sampleRate)
	writeUint32(sampleRate * channels * bits / 8)
	writeUint16(channels * bits / 8)
	writeUint16(bits)
	writeString("data")
	writeUint32(dataSize)
	_, _ = buffer.Write(make([]byte, dataSize))
	return buffer.Bytes()
}
