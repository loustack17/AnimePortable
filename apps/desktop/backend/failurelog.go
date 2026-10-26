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
	// A proxy rejection only explains a player failure that follows it closely.
	upstreamFailureWindow = 30 * time.Second
	// On Windows, antivirus or indexing can hold the just-replaced log open for a few
	// milliseconds, so replacing it fails transiently; a short retry rides that out.
	replaceAttempts   = 5
	replaceRetryDelay = 10 * time.Millisecond
)

type failureStage string

const (
	stageResolve       failureStage = "resolve"
	stageProxyUpstream failureStage = "proxy_upstream"
	stagePlayerLoad    failureStage = "player_load"
	stageStartTimeout  failureStage = "start_timeout"
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
// Each record reads and rewrites the small file, so callers on the UI thread must record
// from a goroutine. A nil *FailureLog is a valid no-op.
type FailureLog struct {
	path string
	now  func() time.Time

	mu sync.Mutex
	// The proxy rejection seen during the current attempt, if any; zero upstreamAt means none.
	upstreamStatus int
	upstreamAt     time.Time
}

func NewFailureLog(directory string) *FailureLog {
	return &FailureLog{path: filepath.Join(directory, failureLogName), now: time.Now}
}

// record appends one line. status outside 100..599 is written as "-".
func (log *FailureLog) record(stage failureStage, status int, category failureCategory) {
	if log == nil {
		return
	}
	line := formatFailureLine(log.now(), stage, status, category)
	log.mu.Lock()
	defer log.mu.Unlock()
	var lines []string
	if data, err := os.ReadFile(log.path); err == nil {
		for _, existing := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			if existing != "" {
				lines = append(lines, existing)
			}
		}
	}
	lines = append(lines, line)
	if extra := len(lines) - failureLogMaxLines; extra > 0 {
		lines = lines[extra:]
	}
	// Diagnostics must never break playback, so a failed write is dropped.
	_ = writeLinesAtomically(log.path, lines)
}

// beginAttempt forgets any proxy rejection left over from an earlier attempt.
func (log *FailureLog) beginAttempt() {
	if log == nil {
		return
	}
	log.mu.Lock()
	log.upstreamAt = time.Time{}
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
	log.mu.Unlock()
}

// PlayerFailed records a player failure, attributing it to the proxy when the proxy
// rejected an upstream response during the same attempt.
func (log *FailureLog) PlayerFailed() {
	if log == nil {
		return
	}
	if status, recent := log.takeRecentUpstream(); recent {
		log.record(stageProxyUpstream, status, upstreamCategory(status))
		return
	}
	log.record(stagePlayerLoad, 0, categoryPlayerFailed)
}

// StartTimedOut records a start that showed no frame in time, with the status of a proxy
// rejection seen during the same attempt, if any.
func (log *FailureLog) StartTimedOut() {
	if log == nil {
		return
	}
	status := 0
	if upstream, recent := log.takeRecentUpstream(); recent {
		status = upstream
	}
	log.record(stageStartTimeout, status, categoryTimeout)
}

// takeRecentUpstream returns the status of the proxy rejection seen within
// upstreamFailureWindow, if any, and forgets it so the next attempt starts clean.
func (log *FailureLog) takeRecentUpstream() (int, bool) {
	log.mu.Lock()
	defer log.mu.Unlock()
	recent := !log.upstreamAt.IsZero() && log.now().Sub(log.upstreamAt) <= upstreamFailureWindow
	log.upstreamAt = time.Time{}
	return log.upstreamStatus, recent
}

func upstreamCategory(status int) failureCategory {
	if status == 0 {
		return failureCategory(securehttp.KindNetwork)
	}
	return failureCategory(securehttp.KindStatus)
}

func writeLinesAtomically(path string, lines []string) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	err := os.Rename(temporary, path)
	for attempt := 1; err != nil && attempt < replaceAttempts; attempt++ {
		time.Sleep(replaceRetryDelay)
		err = os.Rename(temporary, path)
	}
	if err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func formatFailureLine(at time.Time, stage failureStage, status int, category failureCategory) string {
	switch stage {
	case stageResolve, stageProxyUpstream, stagePlayerLoad, stageStartTimeout, stageOther:
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
