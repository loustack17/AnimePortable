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

// reopenedSettings saves settings through the writer, restarts the backend on the same database
// and loads them again, as the next start does. The playback speed is chosen in the player and
// saved on its own, so a non-zero speed is saved that way first; the writer must not undo it.
func reopenedSettings(t *testing.T, saved backend.Settings) backend.Settings {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data", "animeportable.db")
	service := backend.NewAt(path)
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	if saved.PlaybackSpeed != 0 {
		if err := service.SavePlaybackSpeed(context.Background(), saved.PlaybackSpeed); err != nil {
			t.Fatal(err)
		}
	}
	writer := newSettingsWriter(service, nil, nil)
	// The page holds an older speed than the one just saved; the writer must not store it.
	stale := saved
	if saved.PlaybackSpeed != 0 {
		stale.PlaybackSpeed = 100
		if saved.PlaybackSpeed == 100 {
			stale.PlaybackSpeed = 150
		}
	}
	writer.save(stale)
	if err := writer.close(); err != nil {
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
	loaded, err := loadSettings(context.Background(), reopened)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

func TestSettingsSurviveDatabaseReopen(t *testing.T) {
	want := backend.Settings{
		Appearance: "dark", AutoplayNext: "disabled", AutoPlay: "disabled", ResumePlayback: "disabled", Language: "en",
		PlaybackSpeed: 150, HardwareDecoding: "disabled", ProxyMode: "manual", ProxyAddress: "127.0.0.1:7890",
	}
	if got := reopenedSettings(t, want); got != want {
		t.Fatalf("reopened settings=%#v; want %#v", got, want)
	}
}

// The older settings screens saved only the first choices; the fields added since come back
// as the backend's resolved defaults, not as zero values.
func TestSettingsSavedWithoutNewFieldsReopenWithResolvedDefaults(t *testing.T) {
	saved := backend.Settings{Appearance: "dark", AutoplayNext: "disabled", AutoPlay: "disabled", ResumePlayback: "disabled", Language: "en"}
	want := backend.DefaultSettings()
	want.Appearance, want.AutoplayNext, want.AutoPlay, want.ResumePlayback, want.Language = saved.Appearance, saved.AutoplayNext, saved.AutoPlay, saved.ResumePlayback, saved.Language
	if got := reopenedSettings(t, saved); got != want {
		t.Fatalf("reopened settings=%#v; want %#v", got, want)
	}
}

func TestSettingsMissingServiceReportsFailure(t *testing.T) {
	if _, err := loadSettings(context.Background(), nil); !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("missing service load=%v", err)
	}
	reported := make(chan error, 1)
	writer := newSettingsWriter(nil, func(err error) { reported <- err }, nil)
	writer.save(backend.DefaultSettings())
	if err := writer.close(); !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("missing service save=%v", err)
	}
	if err := <-reported; !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("reported failure=%v", err)
	}
}

type settingsTestService struct {
	mu            sync.Mutex
	settings      backend.Settings
	saved         []backend.Settings
	writeStarted  chan struct{}
	continueWrite chan struct{}
	writeErr      error
}

func (service *settingsTestService) Settings(ctx context.Context) (backend.Settings, error) {
	if _, ok := ctx.Deadline(); !ok {
		return backend.Settings{}, errors.New("settings read has no deadline")
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.settings, nil
}

func (service *settingsTestService) SaveSettings(ctx context.Context, settings backend.Settings) error {
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

func (service *settingsTestService) savedSettings() []backend.Settings {
	service.mu.Lock()
	defer service.mu.Unlock()
	return append([]backend.Settings(nil), service.saved...)
}

func TestSettingsWriterCoalescesRequestsAndDrainsLatestOnClose(t *testing.T) {
	service := &settingsTestService{writeStarted: make(chan struct{}, 1), continueWrite: make(chan struct{})}
	writer := newSettingsWriter(service, nil, nil)
	withAppearance := func(appearance string) backend.Settings {
		settings := backend.DefaultSettings()
		settings.Appearance = appearance
		return settings
	}
	if !writer.save(withAppearance("dark")) {
		t.Fatal("initial save was rejected")
	}
	<-service.writeStarted
	for _, appearance := range []string{"light", "system", "light"} {
		if !writer.save(withAppearance(appearance)) {
			t.Fatal("pending save was rejected")
		}
	}
	closed := make(chan error, 1)
	go func() { closed <- writer.close() }()
	close(service.continueWrite)
	if err := <-closed; err != nil {
		t.Fatalf("close() = %v", err)
	}
	if err := writer.close(); err != nil {
		t.Fatalf("repeated close() = %v", err)
	}
	saved := service.savedSettings()
	if len(saved) != 2 || saved[0].Appearance != "dark" || saved[1].Appearance != "light" {
		t.Fatalf("saved values = %#v, want the active write and the latest coalesced one", saved)
	}
	if writer.save(withAppearance("dark")) {
		t.Fatal("save after close was accepted")
	}
}

func TestSettingsWriterReportsFailureAndReturnsLatestWriteResult(t *testing.T) {
	failure := errors.New("first write failed")
	service := &settingsTestService{writeStarted: make(chan struct{}, 1), continueWrite: make(chan struct{}), writeErr: failure}
	reported := make(chan error, 1)
	writer := newSettingsWriter(service, func(err error) { reported <- err }, nil)
	first, latest := backend.DefaultSettings(), backend.DefaultSettings()
	first.Appearance, latest.Appearance = "dark", "light"
	if !writer.save(first) {
		t.Fatal("initial save was rejected")
	}
	<-service.writeStarted
	if !writer.save(latest) {
		t.Fatal("latest save was rejected")
	}
	closed := make(chan error, 1)
	go func() { closed <- writer.close() }()
	close(service.continueWrite)
	if err := <-reported; !errors.Is(err, failure) {
		t.Fatalf("reported error = %v, want %v", err, failure)
	}
	if err := <-closed; err != nil {
		t.Fatalf("close() returned stale failure after latest write succeeded: %v", err)
	}
	if saved := service.savedSettings(); len(saved) != 1 || saved[0] != latest {
		t.Fatalf("successful latest write = %#v", saved)
	}
}

func TestSettingsWriterCloseWaitsForAcceptedWrite(t *testing.T) {
	service := &settingsTestService{writeStarted: make(chan struct{}, 1), continueWrite: make(chan struct{})}
	writer := newSettingsWriter(service, nil, nil)
	if !writer.save(backend.DefaultSettings()) {
		t.Fatal("save was rejected")
	}
	<-service.writeStarted
	closed := make(chan error, 1)
	go func() { closed <- writer.close() }()
	select {
	case err := <-closed:
		t.Fatalf("close returned before accepted write drained: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(service.continueWrite)
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close() = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not complete after the write drained")
	}
}
