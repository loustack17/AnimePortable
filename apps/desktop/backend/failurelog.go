// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/adapters/player/libmpv"
	"animeportable/core"
)

const (
	failureLogName     = "playback-failures.log"
	failureLogMaxLines = 50
	failureLogQueue    = 64
	// A proxy rejection only explains a player failure that follows it closely.
	upstreamFailureWindow = 30 * time.Second
)

type failureStage string

const (
	stageResolve       failureStage = "resolve"
	stageProxyUpstream failureStage = "proxy_upstream"
	stagePlayerLoad    failureStage = "player_load"
	stageOther         failureStage = "other"
)

// failureCategory is a fixed vocabulary. Free-form error text never reaches the log
// because only values from this set can be written (docs/03_SECURITY.md, Logging).
type failureCategory string

const (
	categoryUnknown      failureCategory = "unknown"
	categoryTimeout      failureCategory = "timeout"
	categoryNotFound     failureCategory = "not_found"
	categoryUnavailable  failureCategory = "unavailable"
	categoryInvalidInput failureCategory = "invalid_input"
	categoryPlayerFailed failureCategory = "player_failed"
	categoryPlayerClosed failureCategory = "player_closed"
	categoryPlayerClean  failureCategory = "player_cleanup"
	categoryTracking     failureCategory = "tracking"
)

var allowedCategories = map[failureCategory]struct{}{
	categoryUnknown: {}, categoryTimeout: {}, categoryNotFound: {}, categoryUnavailable: {},
	categoryInvalidInput: {}, categoryPlayerFailed: {}, categoryPlayerClosed: {}, categoryPlayerClean: {},
	categoryTracking:                             {},
	failureCategory(securehttp.KindConfig):       {},
	failureCategory(securehttp.KindURL):          {},
	failureCategory(securehttp.KindOrigin):       {},
	failureCategory(securehttp.KindAddress):      {},
	failureCategory(securehttp.KindRedirect):     {},
	failureCategory(securehttp.KindNetwork):      {},
	failureCategory(securehttp.KindResponse):     {},
	failureCategory(securehttp.KindBodyTooLarge): {},
	failureCategory(securehttp.KindStatus):       {},
}

// FailureLog keeps the newest failureLogMaxLines playback failures next to the database.
// Records are queued and written by one goroutine so callers (including the UI thread)
// never wait for the disk. A full queue drops the record instead of blocking.
// A nil *FailureLog is a valid no-op.
type FailureLog struct {
	path  string
	queue chan string
	done  chan struct{}
	now   func() time.Time

	mu     sync.Mutex
	closed bool
	// The proxy rejection seen during the current attempt, if any.
	upstreamStatus int
	upstreamAt     time.Time
	upstreamSeen   bool

	// Owned by the writer goroutine.
	lines     []string
	loaded    bool
	readFile  func(path string) ([]byte, error)
	writeFile func(path string, lines []string) error
}

func NewFailureLog(directory string) *FailureLog {
	log := &FailureLog{
		path:      filepath.Join(directory, failureLogName),
		queue:     make(chan string, failureLogQueue),
		done:      make(chan struct{}),
		now:       time.Now,
		writeFile: writeLinesAtomically,
		readFile:  os.ReadFile,
	}
	go log.run()
	return log
}

// record queues one line. status outside 100..599 is written as "-".
func (log *FailureLog) record(stage failureStage, status int, category failureCategory) {
	if log == nil {
		return
	}
	log.enqueue(formatFailureLine(log.now(), stage, status, category))
}

func (log *FailureLog) enqueue(line string) {
	log.mu.Lock()
	defer log.mu.Unlock()
	if log.closed {
		return
	}
	select {
	case log.queue <- line:
	default:
	}
}

// beginAttempt forgets any proxy rejection left over from an earlier attempt.
func (log *FailureLog) beginAttempt() {
	if log == nil {
		return
	}
	log.mu.Lock()
	log.upstreamSeen = false
	log.mu.Unlock()
}

// UpstreamRejected remembers the proxy status; the attempt's single line is written
// when the player reports the load failure.
func (log *FailureLog) UpstreamRejected(status int) {
	if log == nil {
		return
	}
	log.mu.Lock()
	log.upstreamStatus = status
	log.upstreamAt = log.now()
	log.upstreamSeen = true
	log.mu.Unlock()
}

// PlayerFailed records a player failure, attributing it to the proxy when the proxy
// rejected an upstream response during the same attempt.
func (log *FailureLog) PlayerFailed() {
	if log == nil {
		return
	}
	log.mu.Lock()
	status, recent := log.upstreamStatus, log.upstreamSeen && log.now().Sub(log.upstreamAt) <= upstreamFailureWindow
	log.upstreamSeen = false
	log.mu.Unlock()
	if recent {
		log.record(stageProxyUpstream, status, upstreamCategory(status))
		return
	}
	log.record(stagePlayerLoad, 0, categoryPlayerFailed)
}

func upstreamCategory(status int) failureCategory {
	if status == 0 {
		return failureCategory(securehttp.KindNetwork)
	}
	return failureCategory(securehttp.KindStatus)
}

func (log *FailureLog) Close() {
	if log == nil {
		return
	}
	log.mu.Lock()
	if log.closed {
		log.mu.Unlock()
		<-log.done
		return
	}
	log.closed = true
	close(log.queue)
	log.mu.Unlock()
	<-log.done
}

func (log *FailureLog) run() {
	defer close(log.done)
	for line := range log.queue {
		log.append(line)
	}
}

func (log *FailureLog) append(line string) {
	if !log.loaded {
		log.loaded = true
		if data, err := log.readFile(log.path); err == nil {
			for _, existing := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
				if existing != "" {
					log.lines = append(log.lines, existing)
				}
			}
		}
	}
	log.lines = append(log.lines, line)
	if extra := len(log.lines) - failureLogMaxLines; extra > 0 {
		log.lines = append([]string(nil), log.lines[extra:]...)
	}
	// Diagnostics must never break playback, so a failed write is dropped.
	_ = log.writeFile(log.path, log.lines)
}

func writeLinesAtomically(path string, lines []string) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func formatFailureLine(at time.Time, stage failureStage, status int, category failureCategory) string {
	switch stage {
	case stageResolve, stageProxyUpstream, stagePlayerLoad, stageOther:
	default:
		stage = stageOther
	}
	if _, ok := allowedCategories[category]; !ok {
		category = categoryUnknown
	}
	code := "-"
	if status >= 100 && status <= 599 {
		code = strconv.Itoa(status)
	}
	return at.UTC().Format(time.RFC3339) + " " + string(stage) + " " + code + " " + string(category)
}

// categoryFor maps an error to the fixed vocabulary. It never reads error text.
func categoryFor(err error) failureCategory {
	var secure *securehttp.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return categoryTimeout
	case errors.As(err, &secure):
		return failureCategory(secure.Kind)
	case errors.Is(err, core.ErrNotFound):
		return categoryNotFound
	case errors.Is(err, ErrUnavailable):
		return categoryUnavailable
	case errors.Is(err, ErrInvalidInput):
		return categoryInvalidInput
	case errors.Is(err, libmpv.ErrPlayerFailed):
		return categoryPlayerFailed
	case errors.Is(err, libmpv.ErrPlayerClosed):
		return categoryPlayerClosed
	case errors.Is(err, libmpv.ErrPlayerCleanup):
		return categoryPlayerClean
	case errors.Is(err, core.ErrPlaybackTracking):
		return categoryTracking
	default:
		return categoryUnknown
	}
}

// stageError marks an error as coming from resolving the playback source.
type stageError struct {
	stage failureStage
	err   error
}

func (e *stageError) Error() string { return e.err.Error() }
func (e *stageError) Unwrap() error { return e.err }

// stageSource tags Resolve failures so Play can tell resolve from player failures.
type stageSource struct{ core.AnimeSource }

func (source stageSource) Resolve(ctx context.Context, ref core.EpisodeRef) (core.PlaybackSource, error) {
	resolved, err := source.AnimeSource.Resolve(ctx, ref)
	if err != nil {
		return resolved, &stageError{stage: stageResolve, err: err}
	}
	return resolved, nil
}

// recordPlayFailure logs one failed Play attempt. Cancellation and shutdown are not failures.
func (log *FailureLog) recordPlayFailure(err error) {
	if log == nil || err == nil || errors.Is(err, context.Canceled) || errors.Is(err, ErrClosed) {
		return
	}
	var secure *securehttp.Error
	if errors.As(err, &secure) && secure.Kind == securehttp.KindCanceled {
		return
	}
	stage := stageOther
	var tagged *stageError
	switch {
	case errors.As(err, &tagged):
		stage = tagged.stage
	case errors.Is(err, libmpv.ErrPlayerFailed), errors.Is(err, libmpv.ErrPlayerCleanup):
		stage = stagePlayerLoad
	}
	status := 0
	if secure != nil {
		status = secure.Status
	}
	log.record(stage, status, categoryFor(err))
}
