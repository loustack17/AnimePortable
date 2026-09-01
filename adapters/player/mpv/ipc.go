package mpv

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"animeportable/core"
)

var (
	ErrIPC          = errors.New("mpv: IPC unavailable")
	ErrIPCClosed    = errors.New("mpv: IPC session closed")
	ErrIPCCommand   = errors.New("mpv: IPC command failed")
	ErrIPCProtocol  = errors.New("mpv: IPC protocol error")
	ErrInvalidMedia = errors.New("mpv: playback media is invalid")
)

const (
	maxIPCLine       = 64 << 10
	maxEventQueue    = 64
	maxRequestID     = 1<<53 - 1
	propertyTimePos  = "time-pos"
	propertyDuration = "duration"
	propertyPause    = "pause"
)

type EventKind uint8

const (
	EventUnknown EventKind = iota
	EventPropertyChange
	EventEndFile
	EventFileLoaded
)

type Event struct {
	Kind     EventKind
	Property string
	Position time.Duration
	Duration time.Duration
	Paused   bool
	Reason   string
	sequence uint64
}

type ipcWireRequest struct {
	Command   []any  `json:"command"`
	RequestID uint64 `json:"request_id,omitempty"`
}

type ipcWireMessage struct {
	Event     string          `json:"event"`
	RequestID uint64          `json:"request_id"`
	ID        uint64          `json:"id"`
	Name      string          `json:"name"`
	Data      json.RawMessage `json:"data"`
	Reason    string          `json:"reason"`
}

type ipcResult struct {
	data     json.RawMessage
	err      error
	sequence uint64
}

type ipcReceipt struct {
	data     json.RawMessage
	err      error
	sequence uint64
}

type ipcLoadReceipt struct {
	barrier uint64
	ack     uint64
	err     error
}

type ipcEventCursor struct {
	done  chan struct{}
	once  sync.Once
	count uint64
}

func newEventCursor() *ipcEventCursor {
	return &ipcEventCursor{done: make(chan struct{})}
}

func (cursor *ipcEventCursor) signal() {
	if cursor == nil {
		return
	}
	cursor.once.Do(func() { close(cursor.done) })
}

type Client struct {
	conn net.Conn

	writeMu         sync.Mutex
	mu              sync.Mutex
	pending         map[uint64]chan ipcResult
	nextID          atomic.Uint64
	receiveSequence atomic.Uint64
	err             error

	done       chan struct{}
	readDone   chan struct{}
	events     chan Event
	eventDone  chan struct{}
	eventSpace chan struct{}
	eventWake  chan struct{}
	eventMu    sync.Mutex
	eventQueue []ipcQueuedEvent
	dispatched uint64
	closeOnce  sync.Once
}

type ipcQueuedEvent struct {
	event  Event
	cursor *ipcEventCursor
}

func NewClient(conn net.Conn) (*Client, error) {
	if conn == nil {
		return nil, ErrIPC
	}
	client := &Client{
		conn:       conn,
		pending:    make(map[uint64]chan ipcResult),
		done:       make(chan struct{}),
		readDone:   make(chan struct{}),
		events:     make(chan Event),
		eventDone:  make(chan struct{}),
		eventSpace: make(chan struct{}, 1),
		eventWake:  make(chan struct{}, 1),
	}
	go client.dispatchEvents()
	go client.readLoop()
	return client, nil
}

func (client *Client) String() string { return "mpv.Client{redacted}" }

func (client *Client) GoString() string { return "mpv.Client{redacted}" }

// Events delivers ordered typed events while IPC is open. Closing IPC may discard events not yet received.
func (client *Client) Events() <-chan Event {
	if client == nil || client.events == nil {
		return closedEvents
	}
	return client.events
}

var closedEvents = func() <-chan Event {
	channel := make(chan Event)
	close(channel)
	return channel
}()

func (client *Client) Close() error {
	if client == nil || client.conn == nil {
		return nil
	}
	client.shutdown(ErrIPCClosed)
	<-client.readDone
	<-client.eventDone
	return nil
}

func (client *Client) request(ctx context.Context, command []any) (json.RawMessage, error) {
	receipt := client.requestReceipt(ctx, command)
	if receipt.err != nil {
		return nil, receipt.err
	}
	return responseData(receipt.data), nil
}

func (client *Client) requestReceipt(ctx context.Context, command []any) ipcReceipt {
	if ctx == nil {
		return ipcReceipt{err: ErrIPC}
	}
	if err := ctx.Err(); err != nil {
		return ipcReceipt{err: err}
	}
	id := client.nextID.Add(1)
	if id == 0 || id > maxRequestID {
		return ipcReceipt{err: ErrIPCProtocol}
	}
	response := make(chan ipcResult, 1)
	client.mu.Lock()
	if client.err != nil {
		err := client.err
		client.mu.Unlock()
		return ipcReceipt{err: err}
	}
	client.pending[id] = response
	client.mu.Unlock()
	if err := client.write(ctx, ipcWireRequest{Command: command, RequestID: id}); err != nil {
		client.removePending(id)
		return ipcReceipt{err: err}
	}
	select {
	case result := <-response:
		return receiptFromResult(result)
	case <-ctx.Done():
		client.removePending(id)
		return ipcReceipt{err: ctx.Err()}
	case <-client.done:
		select {
		case result := <-response:
			return receiptFromResult(result)
		default:
		}
		client.removePending(id)
		return ipcReceipt{err: ErrIPCClosed}
	}
}

func receiptFromResult(result ipcResult) ipcReceipt {
	receipt := ipcReceipt{data: result.data, err: result.err, sequence: result.sequence}
	if receipt.err == nil {
		receipt.err = responseError(receipt.data)
	}
	return receipt
}

func (client *Client) removePending(id uint64) {
	client.mu.Lock()
	delete(client.pending, id)
	client.mu.Unlock()
}

func (client *Client) write(ctx context.Context, request ipcWireRequest) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return ErrIPCProtocol
	}
	payload = append(payload, '\n')
	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	client.mu.Lock()
	closed := client.err != nil
	client.mu.Unlock()
	if closed {
		return ErrIPCClosed
	}
	watchDone := make(chan struct{})
	var watcherDone chan struct{}
	if ctx.Done() != nil {
		watcherDone = make(chan struct{})
		go func() {
			defer close(watcherDone)
			select {
			case <-ctx.Done():
				_ = client.conn.SetWriteDeadline(time.Now())
			case <-watchDone:
			}
		}()
	}
	_, err = client.conn.Write(payload)
	close(watchDone)
	if watcherDone != nil {
		<-watcherDone
	}
	_ = client.conn.SetWriteDeadline(time.Time{})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		client.shutdown(ErrIPCClosed)
		return ErrIPC
	}
	return nil
}

func (client *Client) sendNoWait(ctx context.Context, command []any) error {
	if ctx == nil {
		return ErrIPC
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return client.write(ctx, ipcWireRequest{Command: command})
}

func (client *Client) readLoop() {
	defer close(client.readDone)
	scanner := bufio.NewScanner(client.conn)
	scanner.Buffer(make([]byte, 4096), maxIPCLine)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var message ipcWireMessage
		if json.Unmarshal(line, &message) != nil {
			continue
		}
		sequence := client.receiveSequence.Add(1)
		if message.RequestID != 0 {
			client.deliver(message.RequestID, ipcResult{data: append(json.RawMessage(nil), line...), sequence: sequence})
			continue
		}
		client.handleEvent(message, sequence)
	}
	client.shutdown(ErrIPCClosed)
}

func (client *Client) deliver(id uint64, result ipcResult) {
	client.mu.Lock()
	response, ok := client.pending[id]
	if ok {
		delete(client.pending, id)
	}
	client.mu.Unlock()
	if ok {
		response <- result
	}
}

func (client *Client) shutdown(err error) {
	client.closeOnce.Do(func() {
		client.mu.Lock()
		client.err = err
		pending := client.pending
		client.pending = make(map[uint64]chan ipcResult)
		close(client.done)
		client.mu.Unlock()
		_ = client.conn.Close()
		for _, response := range pending {
			response <- ipcResult{err: err}
		}
	})
}

func responseError(payload []byte) error {
	var message struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(payload, &message) != nil {
		return ErrIPCProtocol
	}
	if message.Error == "" || message.Error == "success" {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrIPCCommand, sanitizedMPVError(message.Error))
}

func responseData(payload []byte) json.RawMessage {
	var message struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(payload, &message) != nil {
		return nil
	}
	return append(json.RawMessage(nil), message.Data...)
}

func sanitizedMPVError(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(value, "property") && strings.Contains(value, "unavailable"):
		return "property unavailable"
	case strings.Contains(value, "unknown") && strings.Contains(value, "command"):
		return "unknown command"
	case strings.Contains(value, "disabled"):
		return "command disabled"
	case strings.Contains(value, "not found"):
		return "not found"
	default:
		return "command rejected"
	}
}

func (client *Client) handleEvent(message ipcWireMessage, sequence uint64) {
	switch message.Event {
	case "end-file":
		client.publish(Event{Kind: EventEndFile, Reason: sanitizedEndReason(message.Reason), sequence: sequence})
	case "file-loaded":
		client.publish(Event{Kind: EventFileLoaded, sequence: sequence})
	case "property-change":
		property := message.Name
		if property == "" {
			property = propertyForID(message.ID)
		}
		if property == "" {
			return
		}
		event := Event{Kind: EventPropertyChange, Property: property, sequence: sequence}
		switch property {
		case propertyTimePos:
			position, ok := decodeSeconds(message.Data)
			if !ok {
				return
			}
			event.Position = position
		case propertyDuration:
			duration, ok := decodeSeconds(message.Data)
			if !ok {
				return
			}
			event.Duration = duration
		case propertyPause:
			if json.Unmarshal(message.Data, &event.Paused) != nil {
				return
			}
		default:
			return
		}
		client.publish(event)
	}
}

func (client *Client) enqueueEventCursor(ctx context.Context) (*ipcEventCursor, error) {
	if client == nil || client.conn == nil {
		return nil, ErrIPCClosed
	}
	if ctx == nil {
		return nil, ErrIPC
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case <-client.done:
		return nil, ErrIPCClosed
	default:
	}
	cursor := newEventCursor()
	for {
		client.eventMu.Lock()
		select {
		case <-client.done:
			client.eventMu.Unlock()
			return nil, ErrIPCClosed
		default:
		}
		if len(client.eventQueue) < maxEventQueue || client.discardQueuedProperty() {
			client.eventQueue = append(client.eventQueue, ipcQueuedEvent{cursor: cursor})
			client.eventMu.Unlock()
			client.wakeEvents()
			return cursor, nil
		}
		client.eventMu.Unlock()
		select {
		case <-client.eventSpace:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-client.done:
			return nil, ErrIPCClosed
		}
	}
}

func propertyForID(id uint64) string {
	switch id {
	case 1:
		return propertyTimePos
	case 2:
		return propertyDuration
	case 3:
		return propertyPause
	default:
		return ""
	}
}

func (client *Client) publish(event Event) {
	for {
		client.eventMu.Lock()
		if event.Kind == EventPropertyChange {
			for index := range client.eventQueue {
				queued := client.eventQueue[index].event
				if queued.Kind == EventPropertyChange && queued.Property == event.Property {
					client.eventQueue[index].event = event
					client.eventMu.Unlock()
					client.wakeEvents()
					return
				}
			}
		}
		if len(client.eventQueue) < maxEventQueue || (isPriorityEvent(event.Kind) && client.discardQueuedProperty()) {
			client.eventQueue = append(client.eventQueue, ipcQueuedEvent{event: event})
			client.eventMu.Unlock()
			client.wakeEvents()
			return
		}
		client.eventMu.Unlock()
		if !isPriorityEvent(event.Kind) {
			return
		}
		select {
		case <-client.eventSpace:
		case <-client.done:
			return
		}
	}
}

func isPriorityEvent(kind EventKind) bool {
	return kind == EventEndFile || kind == EventFileLoaded
}

func (client *Client) discardQueuedProperty() bool {
	for index, item := range client.eventQueue {
		if item.event.Kind == EventPropertyChange {
			client.eventQueue = append(client.eventQueue[:index], client.eventQueue[index+1:]...)
			return true
		}
	}
	return false
}

func (client *Client) wakeEvents() {
	select {
	case client.eventWake <- struct{}{}:
	default:
	}
}

func (client *Client) dispatchEvents() {
	defer close(client.eventDone)
	defer close(client.events)
	defer client.cancelEventCursors()
	for {
		client.eventMu.Lock()
		if len(client.eventQueue) > 0 {
			item := client.eventQueue[0]
			client.eventQueue = client.eventQueue[1:]
			if item.cursor != nil {
				item.cursor.count = client.dispatched
				client.eventMu.Unlock()
				item.cursor.signal()
				continue
			}
			event := item.event
			client.dispatched++
			client.eventMu.Unlock()
			select {
			case client.eventSpace <- struct{}{}:
			default:
			}
			select {
			case client.events <- event:
				continue
			case <-client.done:
				return
			}
		}
		client.eventMu.Unlock()
		select {
		case <-client.eventWake:
		case <-client.done:
			return
		}
	}
}

func (client *Client) cancelEventCursors() {
	client.eventMu.Lock()
	defer client.eventMu.Unlock()
	for _, item := range client.eventQueue {
		if item.cursor != nil {
			item.cursor.signal()
		}
	}
	client.eventQueue = nil
}

func decodeSeconds(payload []byte) (time.Duration, bool) {
	var seconds float64
	if len(payload) == 0 || bytes.Equal(bytes.TrimSpace(payload), []byte("null")) || json.Unmarshal(payload, &seconds) != nil {
		return 0, false
	}
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 || seconds > float64(math.MaxInt64)/float64(time.Second) {
		return 0, false
	}
	return time.Duration(seconds * float64(time.Second)), true
}

func sanitizedEndReason(reason string) string {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "eof", "stop", "quit", "error", "redirect", "unknown":
		return strings.ToLower(strings.TrimSpace(reason))
	default:
		return "unknown"
	}
}

func validMediaURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Hostname() != "127.0.0.1" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	token := strings.TrimPrefix(parsed.EscapedPath(), "/media/")
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return parsed.Port() != "" && token != parsed.EscapedPath() && len(decoded) == 32 && err == nil
}

func (client *Client) LoadFile(ctx context.Context, mediaURL string) error {
	return client.loadFileSequenced(ctx, mediaURL).err
}

func (client *Client) loadFileSequenced(ctx context.Context, mediaURL string) ipcLoadReceipt {
	if client == nil || client.conn == nil {
		return ipcLoadReceipt{err: ErrIPCClosed}
	}
	if !validMediaURL(mediaURL) {
		return ipcLoadReceipt{err: ErrInvalidMedia}
	}
	barrier := client.requestReceipt(ctx, []any{"get_property", "mpv-version"})
	receipt := ipcLoadReceipt{barrier: barrier.sequence, err: barrier.err}
	if receipt.err != nil {
		return receipt
	}
	ack := client.requestReceipt(ctx, []any{"loadfile", mediaURL, "replace"})
	receipt.ack = ack.sequence
	receipt.err = ack.err
	return receipt
}

func (client *Client) Stop(ctx context.Context) error {
	_, err := client.request(ctx, []any{"stop"})
	return err
}

func (client *Client) Seek(ctx context.Context, position time.Duration) error {
	if position < 0 {
		return ErrIPCProtocol
	}
	_, err := client.request(ctx, []any{"seek", float64(position) / float64(time.Second), "absolute"})
	return err
}

func (client *Client) ObserveTimePos(ctx context.Context) error {
	return client.observe(ctx, 1, propertyTimePos)
}

func (client *Client) ObserveDuration(ctx context.Context) error {
	return client.observe(ctx, 2, propertyDuration)
}

func (client *Client) ObservePause(ctx context.Context) error {
	return client.observe(ctx, 3, propertyPause)
}

func (client *Client) observe(ctx context.Context, id uint64, property string) error {
	_, err := client.request(ctx, []any{"observe_property", id, property})
	return err
}

func (client *Client) TimePos(ctx context.Context) (time.Duration, error) {
	return client.getDurationProperty(ctx, propertyTimePos)
}

func (client *Client) Duration(ctx context.Context) (time.Duration, error) {
	return client.getDurationProperty(ctx, propertyDuration)
}

func (client *Client) getDurationProperty(ctx context.Context, property string) (time.Duration, error) {
	data, err := client.request(ctx, []any{"get_property", property})
	if err != nil {
		return 0, err
	}
	value, ok := decodeSeconds(data)
	if !ok {
		return 0, ErrIPCProtocol
	}
	return value, nil
}

func (client *Client) Paused(ctx context.Context) (bool, error) {
	data, err := client.request(ctx, []any{"get_property", propertyPause})
	if err != nil {
		return false, err
	}
	var value bool
	if json.Unmarshal(data, &value) != nil {
		return false, ErrIPCProtocol
	}
	return value, nil
}

func (client *Client) Snapshot(ctx context.Context) (core.PlaybackSnapshot, error) {
	position, err := client.TimePos(ctx)
	if err != nil {
		return core.PlaybackSnapshot{}, err
	}
	duration, err := client.Duration(ctx)
	if err != nil {
		return core.PlaybackSnapshot{}, err
	}
	paused, err := client.Paused(ctx)
	if err != nil {
		return core.PlaybackSnapshot{}, err
	}
	return core.PlaybackSnapshot{Position: position, Duration: duration, Paused: paused}, nil
}

func (client *Client) currentMedia(ctx context.Context) (string, error) {
	data, err := client.request(ctx, []any{"get_property", "path"})
	if err != nil {
		return "", err
	}
	var value string
	if json.Unmarshal(data, &value) != nil || !validMediaURL(value) {
		return "", ErrIPCProtocol
	}
	return value, nil
}

func observeDefaults(ctx context.Context, client *Client) error {
	for _, command := range [][]any{
		{"observe_property", uint64(1), propertyTimePos},
		{"observe_property", uint64(2), propertyDuration},
		{"observe_property", uint64(3), propertyPause},
	} {
		if err := client.sendNoWait(ctx, command); err != nil {
			return err
		}
	}
	return nil
}

type Session struct {
	process  *Process
	client   *Client
	endpoint *ipcEndpoint

	opMu      sync.Mutex
	closeOnce sync.Once
	closeDone chan struct{}
	reapDone  chan struct{}
	closing   chan struct{}
	mu        sync.Mutex
	closeErr  error
}

type ipcStartDeps struct {
	endpoint       func() (*ipcEndpoint, error)
	launcher       launcherDeps
	dial           func(context.Context, *ipcEndpoint, <-chan struct{}) (net.Conn, error)
	startupTimeout time.Duration
}

func StartIPC(ctx context.Context, executable Executable) (*Session, error) {
	return startIPC(ctx, executable, ipcStartDeps{})
}

func startIPC(ctx context.Context, executable Executable, deps ipcStartDeps) (*Session, error) {
	if ctx == nil {
		return nil, ErrStart
	}
	if deps.endpoint == nil {
		deps.endpoint = newEndpoint
	}
	if deps.dial == nil {
		deps.dial = dialEndpoint
	}
	if deps.startupTimeout <= 0 {
		deps.startupTimeout = 5 * time.Second
	}
	endpoint, err := deps.endpoint()
	if err != nil {
		return nil, ErrIPC
	}
	if endpoint == nil || endpoint.name == "" {
		if endpoint != nil {
			_ = endpoint.cleanup()
		}
		return nil, ErrIPC
	}
	process, err := start(ctx, executable, []string{"--idle=yes", "--input-ipc-server=" + endpoint.name}, deps.launcher)
	if err != nil {
		_ = endpoint.cleanup()
		return nil, err
	}
	startupCtx, cancelStartup := context.WithTimeout(ctx, deps.startupTimeout)
	defer cancelStartup()
	connection, err := deps.dial(startupCtx, endpoint, process.Done())
	if err != nil {
		_ = process.Close()
		_ = endpoint.cleanup()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, ErrExited) {
			return nil, ErrExited
		}
		return nil, ErrIPC
	}
	client, err := NewClient(connection)
	if err != nil {
		_ = connection.Close()
		_ = process.Close()
		_ = endpoint.cleanup()
		return nil, ErrIPC
	}
	if err := observeDefaults(startupCtx, client); err != nil {
		_ = client.Close()
		_ = process.Close()
		_ = endpoint.cleanup()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrIPC
	}
	session := &Session{
		process:   process,
		client:    client,
		endpoint:  endpoint,
		closeDone: make(chan struct{}),
		reapDone:  make(chan struct{}),
		closing:   make(chan struct{}),
	}
	go session.reap()
	return session, nil
}

func (session *Session) reap() {
	defer close(session.reapDone)
	<-session.process.Done()
	_ = session.client.Close()
	if session.endpoint.cleanup() != nil {
		session.setCloseError(ErrIPC)
	}
}

func (session *Session) setCloseError(err error) {
	if err == nil {
		return
	}
	session.mu.Lock()
	if session.closeErr == nil {
		session.closeErr = err
	}
	session.mu.Unlock()
}

func (session *Session) PID() int {
	if session == nil || session.process == nil {
		return 0
	}
	return session.process.PID()
}

func (session *Session) Process() *Process {
	if session == nil {
		return nil
	}
	return session.process
}

// Events delivers ordered typed events while the session is open. Closing the session may discard pending events.
func (session *Session) Events() <-chan Event {
	if session == nil || session.client == nil {
		return closedEvents
	}
	return session.client.Events()
}

func (session *Session) LoadFile(ctx context.Context, mediaURL string) error {
	if session == nil || session.client == nil {
		return ErrIPCClosed
	}
	session.opMu.Lock()
	defer session.opMu.Unlock()
	if session.isClosing() {
		return ErrIPCClosed
	}
	return session.client.LoadFile(ctx, mediaURL)
}

func (session *Session) loadFileSequenced(ctx context.Context, mediaURL string) ipcLoadReceipt {
	if session == nil || session.client == nil {
		return ipcLoadReceipt{err: ErrIPCClosed}
	}
	return session.client.loadFileSequenced(ctx, mediaURL)
}

func (session *Session) waitEventsThrough(ctx context.Context) (uint64, error) {
	if session == nil || session.client == nil {
		return 0, ErrIPCClosed
	}
	barrier := session.client.requestReceipt(ctx, []any{"get_property", "mpv-version"})
	if barrier.err != nil {
		return 0, barrier.err
	}
	cursor, err := session.client.enqueueEventCursor(ctx)
	if err != nil {
		return 0, err
	}
	select {
	case <-cursor.done:
		return cursor.count, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-session.client.done:
		return 0, ErrIPCClosed
	}
}

func (session *Session) Stop(ctx context.Context) error {
	if session == nil || session.client == nil {
		return ErrIPCClosed
	}
	return session.client.Stop(ctx)
}

func (session *Session) Seek(ctx context.Context, position time.Duration) error {
	if session == nil || session.client == nil {
		return ErrIPCClosed
	}
	session.opMu.Lock()
	defer session.opMu.Unlock()
	if session.isClosing() {
		return ErrIPCClosed
	}
	return session.client.Seek(ctx, position)
}

func (session *Session) TimePos(ctx context.Context) (time.Duration, error) {
	if session == nil || session.client == nil {
		return 0, ErrIPCClosed
	}
	return session.client.TimePos(ctx)
}

func (session *Session) Duration(ctx context.Context) (time.Duration, error) {
	if session == nil || session.client == nil {
		return 0, ErrIPCClosed
	}
	return session.client.Duration(ctx)
}

func (session *Session) Paused(ctx context.Context) (bool, error) {
	if session == nil || session.client == nil {
		return false, ErrIPCClosed
	}
	return session.client.Paused(ctx)
}

func (session *Session) Snapshot(ctx context.Context) (core.PlaybackSnapshot, error) {
	if session == nil || session.client == nil {
		return core.PlaybackSnapshot{}, ErrIPCClosed
	}
	session.opMu.Lock()
	defer session.opMu.Unlock()
	if session.isClosing() {
		return core.PlaybackSnapshot{}, ErrIPCClosed
	}
	if _, err := session.waitEventsThrough(ctx); err != nil {
		return core.PlaybackSnapshot{}, err
	}
	return session.client.Snapshot(ctx)
}

func (session *Session) currentMedia(ctx context.Context) (string, error) {
	if session == nil || session.client == nil {
		return "", ErrIPCClosed
	}
	return session.client.currentMedia(ctx)
}

func (session *Session) Wait() error {
	if session == nil || session.process == nil {
		return ErrStart
	}
	return session.process.Wait()
}

func (session *Session) Close() error {
	if session == nil || session.process == nil {
		return nil
	}
	session.closeOnce.Do(func() {
		if session.closing != nil {
			close(session.closing)
		}
		_ = session.client.Close()
		session.opMu.Lock()
		session.opMu.Unlock()
		session.setCloseError(session.process.Close())
		select {
		case <-session.reapDone:
		case <-time.After(defaultStopGrace):
			if session.endpoint.cleanup() != nil {
				session.setCloseError(ErrIPC)
			}
		}
		close(session.closeDone)
	})
	<-session.closeDone
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.closeErr
}

func (session *Session) isClosing() bool {
	if session == nil || session.closing == nil {
		return false
	}
	select {
	case <-session.closing:
		return true
	default:
		return false
	}
}

func (session *Session) String() string {
	return "mpv.Session{redacted}"
}

func (session *Session) GoString() string {
	return "mpv.Session{redacted}"
}
