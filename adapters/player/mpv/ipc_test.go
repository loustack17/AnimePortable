package mpv

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClientUsesTypedCommandsAndSurvivesMalformedJSON(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		defer serverConn.Close()
		reader := bufio.NewReader(serverConn)
		for {
			line, readErr := reader.ReadBytes('\n')
			if readErr != nil {
				return
			}
			var request struct {
				Command   []any  `json:"command"`
				RequestID uint64 `json:"request_id"`
			}
			if json.Unmarshal(line, &request) != nil || len(request.Command) == 0 {
				return
			}
			command, _ := request.Command[0].(string)
			response := map[string]any{"error": "success", "request_id": request.RequestID}
			switch command {
			case "get_property":
				property, _ := request.Command[1].(string)
				switch property {
				case propertyTimePos:
					response["data"] = 12.5
				case propertyDuration:
					response["data"] = 24.0 * 60
				case propertyPause:
					response["data"] = true
				}
			case "loadfile":
				if len(request.Command) != 3 || request.Command[2] != "replace" {
					return
				}
			}
			payload, _ := json.Marshal(response)
			if _, writeErr := serverConn.Write(append(payload, '\n')); writeErr != nil {
				return
			}
		}
	}()

	if err := client.LoadFile(context.Background(), proxyMediaURL()); err != nil {
		t.Fatal(err)
	}
	position, err := client.TimePos(context.Background())
	if err != nil || position != 12*time.Second+500*time.Millisecond {
		t.Fatalf("position = %v, %v", position, err)
	}
	duration, err := client.Duration(context.Background())
	if err != nil || duration != 24*time.Minute {
		t.Fatalf("duration = %v, %v", duration, err)
	}
	paused, err := client.Paused(context.Background())
	if err != nil || !paused {
		t.Fatalf("paused = %v, %v", paused, err)
	}
	if err := client.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := serverConn.Write([]byte("not json\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := serverConn.Write([]byte(`{"event":"property-change","name":"time-pos","data":3.25}` + "\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-client.Events():
		if event.Kind != EventPropertyChange || event.Property != propertyTimePos || event.Position != 3*time.Second+250*time.Millisecond {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
	_ = client.Close()
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
}

func TestClientCancellationDoesNotLeavePendingRequest(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	serverDone := make(chan struct{})
	release := make(chan struct{})
	go func() {
		defer close(serverDone)
		defer serverConn.Close()
		_, _ = bufio.NewReader(serverConn).ReadBytes('\n')
		<-release
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := client.TimePos(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation error = %v", err)
	}
	close(release)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClientEndFileEventIsTypedAndReasonSanitized(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := serverConn.Write([]byte(`{"event":"end-file","reason":"secret-url"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-client.Events():
		if event.Kind != EventEndFile || event.Reason != "unknown" {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
	_ = serverConn.Close()
}

func TestClientFileLoadedEventIsTypedAndSanitizedInEitherOrder(t *testing.T) {
	for _, eventFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("event-first=%t", eventFirst), func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			client, err := NewClient(clientConn)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			defer serverConn.Close()

			serverDone := make(chan struct{})
			release := make(chan struct{})
			go func() {
				defer close(serverDone)
				defer serverConn.Close()
				reader := bufio.NewReader(serverConn)
				barrierLine, err := reader.ReadBytes('\n')
				if err != nil {
					return
				}
				var barrier ipcWireRequest
				if json.Unmarshal(barrierLine, &barrier) != nil || len(barrier.Command) != 2 || barrier.Command[0] != "get_property" || barrier.Command[1] != "mpv-version" {
					return
				}
				barrierResponse, _ := json.Marshal(map[string]any{"error": "success", "request_id": barrier.RequestID, "data": "mpv"})
				if _, err := serverConn.Write(append(barrierResponse, '\n')); err != nil {
					return
				}
				loadLine, err := reader.ReadBytes('\n')
				if err != nil {
					return
				}
				var load ipcWireRequest
				if json.Unmarshal(loadLine, &load) != nil || len(load.Command) != 3 || load.Command[0] != "loadfile" || load.Command[2] != "replace" {
					return
				}
				response, _ := json.Marshal(map[string]any{"error": "success", "request_id": load.RequestID})
				event := []byte(`{"event":"file-loaded","filename":"http://127.0.0.1:43210/media/secret","data":{"url":"secret"}}` + "\n")
				if eventFirst {
					_, _ = serverConn.Write(event)
					_, _ = serverConn.Write(append(response, '\n'))
				} else {
					_, _ = serverConn.Write(append(response, '\n'))
					_, _ = serverConn.Write(event)
				}
				<-release
			}()

			if err := client.LoadFile(context.Background(), proxyMediaURL()); err != nil {
				t.Fatal(err)
			}
			select {
			case event := <-client.Events():
				if event.Kind != EventFileLoaded || event.sequence == 0 || event.Property != "" || event.Position != 0 || event.Duration != 0 || event.Paused || event.Reason != "" {
					t.Fatalf("event = %+v", event)
				}
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for file-loaded event")
			}
			close(release)
			select {
			case <-serverDone:
			case <-time.After(time.Second):
				t.Fatal("server did not stop")
			}
		})
	}
}

func TestClientLoadFileReceiptCorrelatesBarrierAndNACK(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer serverConn.Close()
	serverDone := make(chan struct{})
	releaseServer := make(chan struct{})
	go func() {
		defer close(serverDone)
		defer serverConn.Close()
		reader := bufio.NewReader(serverConn)
		barrierLine, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var barrier ipcWireRequest
		if json.Unmarshal(barrierLine, &barrier) != nil {
			return
		}
		if _, err := serverConn.Write([]byte(`{"event":"file-loaded","filename":"private"}` + "\n")); err != nil {
			return
		}
		barrierResponse, _ := json.Marshal(map[string]any{"error": "success", "request_id": barrier.RequestID, "data": "mpv"})
		if _, err := serverConn.Write(append(barrierResponse, '\n')); err != nil {
			return
		}
		loadLine, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var load ipcWireRequest
		if json.Unmarshal(loadLine, &load) != nil {
			return
		}
		response, _ := json.Marshal(map[string]any{"error": "file not found", "request_id": load.RequestID})
		if _, err := serverConn.Write(append(response, '\n')); err != nil {
			return
		}
		<-releaseServer
	}()

	receipt := client.loadFileSequenced(context.Background(), proxyMediaURL())
	if !errors.Is(receipt.err, ErrIPCCommand) {
		t.Fatalf("receipt error = %v", receipt.err)
	}
	if receipt.barrier <= 1 || receipt.ack <= receipt.barrier {
		t.Fatalf("receipt cursors = barrier:%d ack:%d", receipt.barrier, receipt.ack)
	}
	select {
	case event := <-client.Events():
		if event.Kind != EventFileLoaded || event.sequence == 0 || event.sequence >= receipt.barrier {
			t.Fatalf("stale event = %+v barrier=%d", event, receipt.barrier)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stale event")
	}
	close(releaseServer)
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
}

func TestClientEventCursorSurvivesPriorityPressureAndClose(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer serverConn.Close()
	for range maxEventQueue {
		client.publish(Event{Kind: EventEndFile, Reason: "eof"})
	}
	cursor, err := client.enqueueEventCursor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	consumed := make(chan struct{})
	go func() {
		defer close(consumed)
		for range client.Events() {
		}
	}()
	select {
	case <-cursor.done:
	case <-time.After(time.Second):
		t.Fatal("event cursor did not pass queued events")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-consumed:
	case <-time.After(time.Second):
		t.Fatal("event consumer did not stop")
	}
}

func TestSessionWaitEventsThroughUsesPostCommandBarrier(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer serverConn.Close()
	releaseServer := make(chan struct{})
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		defer serverConn.Close()
		reader := bufio.NewReader(serverConn)
		readRequest := func() (ipcWireRequest, bool) {
			line, readErr := reader.ReadBytes('\n')
			if readErr != nil {
				return ipcWireRequest{}, false
			}
			var request ipcWireRequest
			return request, json.Unmarshal(line, &request) == nil
		}
		respond := func(request ipcWireRequest, messageError string) bool {
			payload, _ := json.Marshal(map[string]any{"error": messageError, "request_id": request.RequestID, "data": "mpv"})
			_, writeErr := serverConn.Write(append(payload, '\n'))
			return writeErr == nil
		}
		barrier, ok := readRequest()
		if !ok || !respond(barrier, "success") {
			return
		}
		load, ok := readRequest()
		if !ok || !respond(load, "file not found") {
			return
		}
		if _, err := serverConn.Write([]byte(`{"event":"file-loaded"}` + "\n")); err != nil {
			return
		}
		postBarrier, ok := readRequest()
		if !ok || !respond(postBarrier, "success") {
			return
		}
		<-releaseServer
	}()

	receipt := client.loadFileSequenced(context.Background(), proxyMediaURL())
	if !errors.Is(receipt.err, ErrIPCCommand) || receipt.ack == 0 {
		t.Fatalf("load receipt = %+v", receipt)
	}
	eventDone := make(chan Event, 1)
	go func() { eventDone <- <-client.Events() }()
	processed, err := (&Session{client: client}).waitEventsThrough(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if processed != 1 {
		t.Fatalf("processed events = %d", processed)
	}
	select {
	case event := <-eventDone:
		if event.Kind != EventFileLoaded || event.sequence <= receipt.ack {
			t.Fatalf("event = %+v, ack = %d", event, receipt.ack)
		}
	case <-time.After(time.Second):
		t.Fatal("file-loaded event was not drained")
	}
	close(releaseServer)
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
}

func TestClientCurrentMediaIsPrivateAndValidated(t *testing.T) {
	for iteration := range 100 {
		clientConn, serverConn := net.Pipe()
		client, err := NewClient(clientConn)
		if err != nil {
			t.Fatal(err)
		}
		serverDone := make(chan struct{})
		go func() {
			defer close(serverDone)
			defer serverConn.Close()
			line, err := bufio.NewReader(serverConn).ReadBytes('\n')
			if err != nil {
				return
			}
			var request ipcWireRequest
			if json.Unmarshal(line, &request) != nil || len(request.Command) != 2 || request.Command[0] != "get_property" || request.Command[1] != "path" {
				return
			}
			response, _ := json.Marshal(map[string]any{"error": "success", "request_id": request.RequestID, "data": proxyMediaURL()})
			_, _ = serverConn.Write(append(response, '\n'))
		}()
		media, err := client.currentMedia(context.Background())
		if err != nil || media != proxyMediaURL() {
			t.Fatalf("iteration %d media = %q, %v", iteration, media, err)
		}
		<-serverDone
		_ = client.Close()
	}
}

func TestClientRejectsNonProxyMediaAndRedactsFormatting(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer serverConn.Close()
	for _, mediaURL := range []string{
		"",
		"file:///tmp/video.mp4",
		"https://anime.example/video.mp4",
		"http://127.0.0.1/video.mp4",
		"http://127.0.0.1:1234/not-media/token",
		"http://user:secret@127.0.0.1:1234/video.mp4",
		"http://127.0.0.1:1234/video.mp4\nquit",
	} {
		if err := client.LoadFile(context.Background(), mediaURL); !errors.Is(err, ErrInvalidMedia) {
			t.Fatalf("media %q error = %v", mediaURL, err)
		}
	}
	if fmt.Sprint(client) != "mpv.Client{redacted}" || fmt.Sprintf("%#v", client) != "mpv.Client{redacted}" {
		t.Fatal("client formatting exposed IPC state")
	}
}

func TestClientOversizedFrameClosesReader(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	written := make(chan struct{})
	go func() {
		_, _ = serverConn.Write(append(make([]byte, maxIPCLine+1), '\n'))
		_ = serverConn.Close()
		close(written)
	}()
	select {
	case <-client.done:
	case <-time.After(time.Second):
		t.Fatal("oversized frame did not close IPC reader")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	<-written
}

func TestClientPreservesTerminalEventWhenProgressQueueIsFull(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer serverConn.Close()
	for index := range 64 {
		client.publish(Event{Kind: EventPropertyChange, Property: fmt.Sprintf("property-%d", index)})
	}
	client.publish(Event{Kind: EventEndFile, Reason: "eof"})
	for range 65 {
		select {
		case event := <-client.Events():
			if event.Kind == EventEndFile {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("terminal event was not delivered")
		}
	}
	t.Fatal("terminal event was not preserved")
}

func TestClientPreservesFileLoadedEventWhenProgressQueueIsFull(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer serverConn.Close()
	for index := range maxEventQueue {
		client.publish(Event{Kind: EventPropertyChange, Property: fmt.Sprintf("property-%d", index)})
	}
	client.publish(Event{Kind: EventFileLoaded})
	for range maxEventQueue + 1 {
		select {
		case event := <-client.Events():
			if event.Kind == EventFileLoaded {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("file-loaded event was not delivered")
		}
	}
	t.Fatal("file-loaded event was not preserved")
}

func TestClientBackpressuresWithoutDroppingTerminalEvents(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer serverConn.Close()
	published := make(chan struct{})
	go func() {
		for range 65 {
			client.publish(Event{Kind: EventEndFile, Reason: "eof"})
		}
		close(published)
	}()
	for range 65 {
		select {
		case event := <-client.Events():
			if event.Kind != EventEndFile {
				t.Fatalf("event = %+v", event)
			}
		case <-time.After(time.Second):
			t.Fatal("terminal event was dropped")
		}
	}
	select {
	case <-published:
	case <-time.After(time.Second):
		t.Fatal("terminal publisher remained blocked")
	}
}

func TestClientBackpressuresWithoutDroppingFileLoadedEvents(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer serverConn.Close()
	published := make(chan struct{})
	go func() {
		for range maxEventQueue + 1 {
			client.publish(Event{Kind: EventFileLoaded})
		}
		close(published)
	}()
	for range maxEventQueue + 1 {
		select {
		case event := <-client.Events():
			if event.Kind != EventFileLoaded {
				t.Fatalf("event = %+v", event)
			}
		case <-time.After(time.Second):
			t.Fatal("file-loaded event was dropped")
		}
	}
	select {
	case <-published:
	case <-time.After(time.Second):
		t.Fatal("file-loaded publisher remained blocked")
	}
}

func TestStartIPCUsesServerEndpointAndCleansUp(t *testing.T) {
	backendConn, mpvConn := net.Pipe()
	var args []string
	var cleanups int
	endpoint := &ipcEndpoint{name: "private-endpoint", cleanupFn: func() error {
		cleanups++
		return nil
	}}
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		defer mpvConn.Close()
		reader := bufio.NewReader(mpvConn)
		for {
			if _, err := reader.ReadBytes('\n'); err != nil {
				return
			}
		}
	}()
	session, err := startIPC(context.Background(), Executable{path: executableFixturePath()}, ipcStartDeps{
		endpoint: func() (*ipcEndpoint, error) { return endpoint, nil },
		dial: func(context.Context, *ipcEndpoint, <-chan struct{}) (net.Conn, error) {
			return backendConn, nil
		},
		launcher: launcherDeps{command: func(_ string, gotArgs ...string) *exec.Cmd {
			args = append([]string(nil), gotArgs...)
			return helperCommand("wait")
		}, grace: 20 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || args[0] != "--idle=yes" || args[1] != "--input-ipc-server=private-endpoint" {
		t.Fatalf("args = %q", args)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	<-drained
	if cleanups != 1 {
		t.Fatalf("cleanups = %d", cleanups)
	}
}

func TestStartIPCHandshakeHasInternalTimeout(t *testing.T) {
	backendConn, mpvConn := net.Pipe()
	defer mpvConn.Close()
	endpoint := &ipcEndpoint{name: "private-endpoint", cleanupFn: func() error { return nil }}
	started := time.Now()
	session, err := startIPC(context.Background(), Executable{path: executableFixturePath()}, ipcStartDeps{
		endpoint: func() (*ipcEndpoint, error) { return endpoint, nil },
		dial: func(context.Context, *ipcEndpoint, <-chan struct{}) (net.Conn, error) {
			return backendConn, nil
		},
		launcher: launcherDeps{
			command: func(string, ...string) *exec.Cmd { return helperCommand("wait") },
			grace:   20 * time.Millisecond,
		},
		startupTimeout: 20 * time.Millisecond,
	})
	if session != nil || !errors.Is(err, ErrIPC) {
		t.Fatalf("start = %v, %v", session, err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("IPC handshake timeout was not bounded")
	}
}

func TestEndpointIsPrivateAndCleanedUp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix endpoint permissions are not applicable")
	}
	endpoint, err := newEndpoint()
	if err != nil {
		t.Skipf("IPC endpoint unavailable: %v", err)
	}
	if endpoint.name == "" {
		t.Fatal("endpoint name is empty")
	}
	if endpoint.String() != "mpv.IPCEndpoint{redacted}" || strings.Contains(endpoint.String(), endpoint.name) {
		t.Fatal("endpoint formatting exposed endpoint")
	}
	parent := filepath.Dir(endpoint.name)
	if info, err := os.Stat(parent); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("endpoint directory permissions = %v, %v", info, err)
	}
	listener, err := net.Listen("unix", endpoint.name)
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, _ := listener.Accept()
		accepted <- conn
	}()
	conn, err := dialEndpoint(context.Background(), endpoint, make(chan struct{}))
	if err != nil {
		t.Fatal(err)
	}
	serverConn := <-accepted
	if info, err := os.Stat(endpoint.name); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("endpoint permissions = %v, %v", info, err)
	}
	_ = serverConn.Close()
	_ = conn.Close()
	_ = listener.Close()
	if err := endpoint.cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(parent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("endpoint directory still exists: %v", err)
	}
}

func TestSessionCloseIsConcurrentAndIdempotent(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	endpoint := &ipcEndpoint{name: "test-endpoint", cleanupFn: func() error { return nil }}
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		defer serverConn.Close()
		reader := bufio.NewReader(serverConn)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			var request ipcWireRequest
			if json.Unmarshal(line, &request) != nil {
				return
			}
			response, _ := json.Marshal(map[string]any{"error": "success", "request_id": request.RequestID})
			if _, err := serverConn.Write(append(response, '\n')); err != nil {
				return
			}
		}
	}()
	client, err := NewClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	process, err := start(context.Background(), Executable{path: executableFixturePath()}, []string{"--idle=yes"}, launcherDeps{
		command: func(string, ...string) *exec.Cmd { return helperCommand("wait") },
		grace:   20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	session := &Session{process: process, client: client, endpoint: endpoint, closeDone: make(chan struct{}), reapDone: make(chan struct{})}
	go session.reap()
	var waiters sync.WaitGroup
	for range 8 {
		waiters.Add(1)
		go func() {
			defer waiters.Done()
			_ = session.Close()
		}()
	}
	waiters.Wait()
	<-serverDone
	if err := session.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestLiveIPC(t *testing.T) {
	if os.Getenv("ANIMEPORTABLE_MPV_LIVE") != "1" {
		t.Skip("live MPV smoke disabled")
	}
	executable, err := Find("")
	if err != nil {
		t.Skipf("MPV unavailable: %v", err)
	}
	session, err := StartIPC(context.Background(), executable)
	if err != nil {
		t.Fatal(err)
	}
	if session.PID() <= 0 {
		t.Fatal("MPV did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := session.Paused(ctx); err != nil {
		t.Fatal(err)
	}
	if err := session.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func proxyMediaURL() string {
	return "http://127.0.0.1:43210/media/" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
}
