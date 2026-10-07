//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"strings"
	"sync"
	"time"

	"animeportable/apps/desktop/backend"
)

const appearanceIOTimeout = 3 * time.Second

type appearanceService interface {
	Settings(context.Context) (backend.Settings, error)
	SaveSettings(context.Context, backend.Settings) error
}

func loadAppearance(ctx context.Context, service appearanceService) (bool, error) {
	if service == nil {
		return false, backend.ErrUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, appearanceIOTimeout)
	defer cancel()
	settings, err := service.Settings(ctx)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(settings.Appearance), "dark"), nil
}

type appearancePersistence struct {
	service appearanceService
	onError func(error)

	mu           sync.Mutex
	pending      *bool
	workerActive bool
	closed       bool
	done         chan struct{}
	doneOnce     sync.Once
	lastErr      error
}

func newAppearancePersistence(service appearanceService, onError func(error)) *appearancePersistence {
	return &appearancePersistence{service: service, onError: onError, done: make(chan struct{})}
}

func (persistence *appearancePersistence) setDark(dark bool) bool {
	persistence.mu.Lock()
	defer persistence.mu.Unlock()
	if persistence.closed {
		return false
	}
	persistence.pending = &dark
	if !persistence.workerActive {
		persistence.workerActive = true
		go persistence.run()
	}
	return true
}

func (persistence *appearancePersistence) close() error {
	persistence.mu.Lock()
	persistence.closed = true
	if !persistence.workerActive {
		persistence.doneOnce.Do(func() { close(persistence.done) })
	}
	persistence.mu.Unlock()
	<-persistence.done

	persistence.mu.Lock()
	err := persistence.lastErr
	persistence.mu.Unlock()
	return err
}

func (persistence *appearancePersistence) run() {
	for {
		persistence.mu.Lock()
		if persistence.pending == nil {
			persistence.workerActive = false
			if persistence.closed {
				persistence.doneOnce.Do(func() { close(persistence.done) })
			}
			persistence.mu.Unlock()
			return
		}
		dark := *persistence.pending
		persistence.pending = nil
		persistence.mu.Unlock()

		err := persistence.save(dark)
		persistence.mu.Lock()
		persistence.lastErr = err
		persistence.mu.Unlock()
		if err != nil && persistence.onError != nil {
			persistence.onError(err)
		}
	}
}

func (persistence *appearancePersistence) save(dark bool) error {
	if persistence.service == nil {
		return backend.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(context.Background(), appearanceIOTimeout)
	defer cancel()
	settings, err := persistence.service.Settings(ctx)
	if err != nil {
		return err
	}
	if dark {
		settings.Appearance = "dark"
	} else {
		settings.Appearance = "light"
	}
	return persistence.service.SaveSettings(ctx, settings)
}
