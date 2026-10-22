//go:build windows && amd64 && cgo

package fltkengine_test

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"animeportable/apps/desktop/fltkengine"
)

const (
	proxyChildMarker  = "ANIMEPORTABLE_PROXY_CHILD"
	proxyChildControl = "ANIMEPORTABLE_PROXY_CHILD_CONTROL"
)

// countingProxy accepts connections, counts them and drops them: a stand-in for the proxy a
// user's environment names. It returns the address to put into http_proxy.
func countingProxy(t *testing.T) (string, *atomic.Int32) {
	t.Helper()
	var connections atomic.Int32
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			connection.Close()
		}
	}()
	return "http://" + listener.Addr().String(), &connections
}

// TestProxyEnvChild is the body of the child process the tests below start. It is inert in a
// normal run. It loads a loopback stream and prints what happened, one "key=value" per line.
func TestProxyEnvChild(t *testing.T) {
	if os.Getenv(proxyChildMarker) == "" {
		t.Skip("only runs as the child of the proxy environment tests")
	}
	var mediaRequests atomic.Int32
	var media string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mediaRequests.Add(1)
		http.ServeFile(w, r, media)
	}))
	t.Cleanup(server.Close)

	if os.Getenv(proxyChildControl) != "" {
		fltkengine.AllowEnvironmentProxy()
	}
	fixture := newEndFixture(t)
	media = fixture.media
	var duration time.Duration
	fixture.engine.SetStateHandler(func(_, d time.Duration, _ bool, _ int) { duration = d })
	fixture.load(server.URL+"/stream/token/black.mp4", 0, 1)
	pumpUntil(t, 8*time.Second, func() bool { return duration > 0 })
	println("CHILDRESULT loaded=" + strconv.FormatBool(duration > 0) + " mediaRequests=" + strconv.Itoa(int(mediaRequests.Load())))
}

type proxyChildResult struct {
	loaded           bool
	mediaRequests    int
	proxyConnections int32
	output           string
}

// runProxyChild starts this test binary as a fresh process whose initial environment names the
// counting proxy as http_proxy plus the extra variables, so the C runtime inside libmpv sees
// them the way it would for a user who launched the app with them set.
func runProxyChild(t *testing.T, extraEnv ...string) proxyChildResult {
	t.Helper()
	proxy, connections := countingProxy(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProxyEnvChild$", "-test.count=1", "-test.v")
	environment := make([]string, 0, len(os.Environ())+4)
	for _, entry := range os.Environ() {
		name := strings.ToLower(strings.SplitN(entry, "=", 2)[0])
		if name == "http_proxy" || name == "no_proxy" || name == "https_proxy" {
			continue
		}
		environment = append(environment, entry)
	}
	environment = append(environment, proxyChildMarker+"=1", "http_proxy="+proxy)
	environment = append(environment, extraEnv...)
	command.Env = environment
	raw, err := command.CombinedOutput()
	output := string(raw)
	if err != nil {
		t.Fatalf("child failed: %v\n%s", err, output)
	}
	result := proxyChildResult{proxyConnections: connections.Load(), output: output}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "CHILDRESULT ") {
			continue
		}
		for _, field := range strings.Fields(line)[1:] {
			key, value, _ := strings.Cut(field, "=")
			switch key {
			case "loaded":
				result.loaded = value == "true"
			case "mediaRequests":
				result.mediaRequests, _ = strconv.Atoi(value)
			}
		}
	}
	return result
}

// The playback URL points at the local playback proxy and carries the session token, so mpv
// must fetch it directly even when the user's environment names an HTTP proxy. The C runtime
// inside libmpv snapshots the environment, so each run is a fresh process that starts with
// http_proxy set, the way a user would launch the app. The control run drops the
// direct-connection option and must reach the proxy, so the zero count of the real run comes
// from the option and not from mpv ignoring the variable.
func TestEngineFetchesLoopbackStreamsDirectlyDespiteAnEnvironmentProxy(t *testing.T) {
	control := runProxyChild(t, proxyChildControl+"=1")
	t.Logf("control (option dropped): loaded=%t mediaRequests=%d proxyConnections=%d", control.loaded, control.mediaRequests, control.proxyConnections)
	if control.proxyConnections == 0 {
		t.Fatalf("mpv never used the environment proxy without the option, so the check below proves nothing:\n%s", control.output)
	}
	if control.loaded {
		t.Fatal("the control stream loaded although its only route was a proxy that drops connections")
	}

	direct := runProxyChild(t)
	t.Logf("with the option: loaded=%t mediaRequests=%d proxyConnections=%d", direct.loaded, direct.mediaRequests, direct.proxyConnections)
	if direct.proxyConnections != 0 {
		t.Fatalf("mpv sent %d connections to the environment proxy; the loopback URL must be fetched directly", direct.proxyConnections)
	}
	if !direct.loaded || direct.mediaRequests == 0 {
		t.Fatalf("the loopback stream did not load directly:\n%s", direct.output)
	}
}
