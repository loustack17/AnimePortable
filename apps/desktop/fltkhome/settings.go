//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"sync"
	"time"

	"animeportable/apps/desktop/backend"
)

const settingsIOTimeout = 3 * time.Second

// settingsActionService is what the rows that act (network test, update check, data folder)
// ask of the backend.
type settingsActionService interface {
	TestProxy(context.Context, backend.Settings) (backend.ProxyTestResult, error)
	CheckForUpdate(context.Context) (backend.UpdateInfo, error)
	DataLocation(context.Context) (backend.DataLocation, error)
	MoveDataLocation(context.Context, backend.MoveDataLocationRequest) (backend.MoveDataLocationResult, error)
}

type settingsService interface {
	Settings(context.Context) (backend.Settings, error)
	SaveSettings(context.Context, backend.Settings) error
}

func loadSettings(ctx context.Context, service settingsService) (backend.Settings, error) {
	if service == nil {
		return backend.Settings{}, backend.ErrUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, settingsIOTimeout)
	defer cancel()
	return service.Settings(ctx)
}

// settingsWriter saves settings in the background. The UI holds the whole settings value and
// hands over all of it, so a save never reads the stored row first and two toggles changed in
// quick succession cannot overwrite each other. Saves made while one is running are coalesced
// into the latest value, which is written after it.
type settingsWriter struct {
	service settingsService
	onError func(error)
	onSaved func()

	mu           sync.Mutex
	pending      *backend.Settings
	workerActive bool
	closed       bool
	done         chan struct{}
	doneOnce     sync.Once
	lastErr      error
}

func newSettingsWriter(service settingsService, onError func(error), onSaved func()) *settingsWriter {
	return &settingsWriter{service: service, onError: onError, onSaved: onSaved, done: make(chan struct{})}
}

// save queues settings and reports false once the writer is closed.
func (writer *settingsWriter) save(settings backend.Settings) bool {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.closed {
		return false
	}
	writer.pending = &settings
	if !writer.workerActive {
		writer.workerActive = true
		go writer.run()
	}
	return true
}

// close waits for every accepted save and returns the result of the last one.
func (writer *settingsWriter) close() error {
	writer.mu.Lock()
	writer.closed = true
	if !writer.workerActive {
		writer.doneOnce.Do(func() { close(writer.done) })
	}
	writer.mu.Unlock()
	<-writer.done

	writer.mu.Lock()
	err := writer.lastErr
	writer.mu.Unlock()
	return err
}

func (writer *settingsWriter) run() {
	for {
		writer.mu.Lock()
		if writer.pending == nil {
			writer.workerActive = false
			if writer.closed {
				writer.doneOnce.Do(func() { close(writer.done) })
			}
			writer.mu.Unlock()
			return
		}
		settings := *writer.pending
		writer.pending = nil
		writer.mu.Unlock()

		err := writer.write(settings)
		writer.mu.Lock()
		writer.lastErr = err
		writer.mu.Unlock()
		if err != nil && writer.onError != nil {
			writer.onError(err)
		} else if err == nil && writer.onSaved != nil {
			writer.onSaved()
		}
	}
}

func (writer *settingsWriter) write(settings backend.Settings) error {
	if writer.service == nil {
		return backend.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsIOTimeout)
	defer cancel()
	return writer.service.SaveSettings(ctx, settings)
}
