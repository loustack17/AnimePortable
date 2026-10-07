//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
)

func TestAppearanceSurvivesDatabaseReopenWithoutChangingOtherSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "animeportable.db")
	service := backend.NewAt(path)
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	original := backend.Settings{Appearance: "light", AutoplayNext: "disabled", ResumePlayback: "enabled", Language: "en"}
	if err := service.SaveSettings(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	persistence := newAppearancePersistence(service, nil)
	persistence.setDark(true)
	if err := persistence.close(); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := backend.NewAt(path)
	t.Cleanup(func() { _ = reopened.Close() })
	if err := reopened.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	dark, err := loadAppearance(context.Background(), reopened)
	if err != nil || !dark {
		t.Fatalf("reopened theme dark=%v, err=%v", dark, err)
	}
	saved, err := reopened.Settings(context.Background())
	original.Appearance = "dark"
	if err != nil || saved != original {
		t.Fatalf("reopened settings=%#v, err=%v; want %#v", saved, err, original)
	}
}

func TestAppearanceMissingServiceReportsFailure(t *testing.T) {
	if _, err := loadAppearance(context.Background(), nil); !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("missing service load=%v", err)
	}
	reported := make(chan error, 1)
	persistence := newAppearancePersistence(nil, func(err error) { reported <- err })
	persistence.setDark(true)
	if err := persistence.close(); !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("missing service save=%v", err)
	}
	if err := <-reported; !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("reported failure=%v", err)
	}
}

type appearanceTestService struct {
	mu              sync.Mutex
	settings        backend.Settings
	saved           []backend.Settings
	settingsStarted chan struct{}
	continueRead    chan struct{}
	writeStarted    chan struct{}
	continueWrite   chan struct{}
	writeErr        error
}

func (service *appearanceTestService) Settings(ctx context.Context) (backend.Settings, error) {
	if service.settingsStarted != nil {
		select {
		case service.settingsStarted <- struct{}{}:
		default:
		}
	}
	if service.continueRead != nil {
		select {
		case <-service.continueRead:
		case <-ctx.Done():
			return backend.Settings{}, ctx.Err()
		}
	}
	if _, ok := ctx.Deadline(); !ok {
		return backend.Settings{}, errors.New("settings read has no deadline")
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.settings, nil
}

func (service *appearanceTestService) SaveSettings(ctx context.Context, settings backend.Settings) error {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("settings write has no deadline")
	}
	if service.writeStarted != nil {
		select {
		case service.writeStarted <- struct{}{}:
		default:
		}
	}
	if service.continueWrite != nil {
		select {
		case <-service.continueWrite:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.writeErr != nil {
		err := service.writeErr
		service.writeErr = nil
		return err
	}
	service.saved = append(service.saved, settings)
	service.settings = settings
	return nil
}

func TestLoadAppearanceDefaultsToLightAndRecognizesDark(t *testing.T) {
	for _, test := range []struct {
		name       string
		appearance string
		wantDark   bool
	}{
		{name: "missing", appearance: "", wantDark: false},
		{name: "light", appearance: "light", wantDark: false},
		{name: "dark", appearance: "dark", wantDark: true},
		{name: "normalized dark", appearance: " DARK ", wantDark: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &appearanceTestService{settings: backend.Settings{Appearance: test.appearance}}
			got, err := loadAppearance(context.Background(), service)
			if err != nil || got != test.wantDark {
				t.Fatalf("loadAppearance() = %v, %v; want %v, nil", got, err, test.wantDark)
			}
		})
	}
}

func TestAppearancePersistenceCoalescesRequestsAndDrainsLatestOnClose(t *testing.T) {
	service := &appearanceTestService{
		settings:        backend.Settings{Appearance: "light", AutoplayNext: "enabled", ResumePlayback: "disabled", Language: "zh-TW"},
		settingsStarted: make(chan struct{}, 1),
		continueRead:    make(chan struct{}),
	}
	persistence := newAppearancePersistence(service, nil)
	if !persistence.setDark(true) {
		t.Fatal("initial theme request was rejected")
	}
	<-service.settingsStarted
	for _, dark := range []bool{false, true, false} {
		if !persistence.setDark(dark) {
			t.Fatal("pending theme request was rejected")
		}
	}
	closed := make(chan error, 1)
	go func() { closed <- persistence.close() }()
	close(service.continueRead)
	if err := <-closed; err != nil {
		t.Fatalf("close() = %v", err)
	}
	if err := persistence.close(); err != nil {
		t.Fatalf("repeated close() = %v", err)
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if len(service.saved) != 2 {
		t.Fatalf("saved values = %#v, want active and coalesced writes", service.saved)
	}
	if service.saved[0].Appearance != "dark" || service.saved[1].Appearance != "light" {
		t.Fatalf("saved appearances = %q, %q", service.saved[0].Appearance, service.saved[1].Appearance)
	}
	for _, saved := range service.saved {
		if saved.AutoplayNext != "enabled" || saved.ResumePlayback != "disabled" || saved.Language != "zh-TW" {
			t.Fatalf("non-appearance settings changed: %#v", saved)
		}
	}
	if persistence.setDark(true) {
		t.Fatal("request after close was accepted")
	}
}

func TestAppearancePersistenceReportsFailureAndReturnsLatestWriteResult(t *testing.T) {
	failure := errors.New("first write failed")
	service := &appearanceTestService{
		settings:      backend.Settings{Appearance: "light", AutoplayNext: "enabled", ResumePlayback: "enabled", Language: "en"},
		writeStarted:  make(chan struct{}, 1),
		continueWrite: make(chan struct{}),
		writeErr:      failure,
	}
	reported := make(chan error, 1)
	persistence := newAppearancePersistence(service, func(err error) { reported <- err })
	if !persistence.setDark(true) {
		t.Fatal("initial theme request was rejected")
	}
	<-service.writeStarted
	if !persistence.setDark(false) {
		t.Fatal("latest theme request was rejected")
	}
	closed := make(chan error, 1)
	go func() { closed <- persistence.close() }()
	close(service.continueWrite)
	if err := <-reported; !errors.Is(err, failure) {
		t.Fatalf("reported error = %v, want %v", err, failure)
	}
	if err := <-closed; err != nil {
		t.Fatalf("close() returned stale failure after latest write succeeded: %v", err)
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if len(service.saved) != 1 || service.saved[0].Appearance != "light" {
		t.Fatalf("successful latest write = %#v", service.saved)
	}
}

func TestAppearancePersistenceCloseWaitsForAcceptedWrite(t *testing.T) {
	service := &appearanceTestService{
		settings:        backend.Settings{Appearance: "light"},
		settingsStarted: make(chan struct{}, 1),
		continueRead:    make(chan struct{}),
	}
	persistence := newAppearancePersistence(service, nil)
	if !persistence.setDark(true) {
		t.Fatal("theme request was rejected")
	}
	<-service.settingsStarted
	closeStarted := make(chan struct{})
	closed := make(chan error, 1)
	go func() {
		close(closeStarted)
		closed <- persistence.close()
	}()
	<-closeStarted
	select {
	case err := <-closed:
		t.Fatalf("close returned before accepted write drained: %v", err)
	default:
	}
	close(service.continueRead)
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close() = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not complete after the write drained")
	}
}
