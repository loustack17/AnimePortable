//go:build windows && amd64 && cgo

package fltkengine_test

import (
	"context"
	"testing"
	"time"

	"animeportable/apps/desktop/fltkengine"
)

func (fixture *endFixture) property(name string) string {
	fixture.t.Helper()
	value, err := pumpFLTKResult(fixture.t, 10*time.Second, func() (string, error) {
		return fixture.engine.CoreProperty(context.Background(), name)
	})
	if err != nil {
		fixture.t.Fatalf("read %s: %v", name, err)
	}
	return value
}

// loaded loads the fixture and waits until mpv has opened the file.
func (fixture *endFixture) loaded(generation uint64) {
	fixture.t.Helper()
	var duration time.Duration
	fixture.engine.SetStateHandler(func(_, d time.Duration, _ bool, _ int) { duration = d })
	fixture.load(fixture.media, 0, generation)
	if !pumpUntil(fixture.t, endWait, func() bool { return duration > 0 }) {
		fixture.t.Fatalf("generation %d never reported a duration", generation)
	}
}

func TestEngineEveryLoadStartsAtTheConfiguredSpeed(t *testing.T) {
	fixture := newEndFixture(t)
	fixture.engine.SetPlaybackSpeed(150)
	fixture.loaded(1)
	if got := fixture.property("speed"); got != "1.500000" {
		t.Fatalf("speed after a load at 150%% = %q, want 1.500000", got)
	}

	fixture.engine.SetPlaybackSpeed(100)
	fixture.loaded(2)
	if got := fixture.property("speed"); got != "1.000000" {
		t.Fatalf("speed after a later load at 100%% = %q, want 1.000000", got)
	}
}

func TestEngineHardwareDecodingFollowsTheOption(t *testing.T) {
	for _, test := range []struct {
		name    string
		enabled bool
		want    string
	}{
		{"disabled", false, "no"},
		{"enabled", true, "auto"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newEndFixtureWith(t, fltkengine.Options{HardwareDecoding: test.enabled})
			fixture.loaded(1)
			if got := fixture.property("hwdec"); got != test.want {
				t.Fatalf("hwdec option = %q, want %q", got, test.want)
			}
			if !test.enabled {
				if got := fixture.property("hwdec-current"); got != "no" {
					t.Fatalf("hwdec-current with hardware decoding off = %q, want no", got)
				}
			}
		})
	}
}

func TestEngineSpeedControlChangesLiveAndCarriesToTheNextLoad(t *testing.T) {
	fixture := newEndFixture(t)
	fixture.loaded(1)
	if got := fixture.property("speed"); got != "1.000000" {
		t.Fatalf("speed before the control = %q, want 1.000000", got)
	}
	if err := pumpFLTKError(t, 10*time.Second, func() error {
		return fixture.engine.Control(context.Background(), "speed", 150)
	}); err != nil {
		t.Fatalf("speed control: %v", err)
	}
	if got := fixture.property("speed"); got != "1.500000" {
		t.Fatalf("speed right after the control = %q, want 1.500000", got)
	}

	fixture.loaded(2)
	if got := fixture.property("speed"); got != "1.500000" {
		t.Fatalf("speed after the next load = %q, want the chosen 1.500000", got)
	}

	if err := pumpFLTKError(t, 10*time.Second, func() error {
		return fixture.engine.Control(context.Background(), "speed", 0)
	}); err == nil {
		t.Fatal("a speed of 0 was accepted")
	}
}
