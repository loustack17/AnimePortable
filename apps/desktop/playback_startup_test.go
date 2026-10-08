package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPlaybackEpisodeLookupDoesNotBlockControls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lookupStarted := make(chan struct{})
	lookupDone := make(chan struct{})
	controlDone := make(chan struct{})
	queue := newActionQueue()
	queue.Enqueue(func() {
		err := startPlayback(ctx, cancel, func(context.Context) error { return nil }, func(ctx context.Context) {
			close(lookupStarted)
			<-ctx.Done()
			close(lookupDone)
		})
		if err != nil {
			t.Errorf("start playback: %v", err)
		}
	})
	queue.Enqueue(func() { close(controlDone) })
	select {
	case <-controlDone:
	case <-time.After(time.Second):
		t.Fatal("episode lookup blocked playback controls")
	}
	select {
	case <-lookupStarted:
	case <-time.After(time.Second):
		t.Fatal("episode lookup did not start")
	}
	cancel()
	select {
	case <-lookupDone:
	case <-time.After(time.Second):
		t.Fatal("episode lookup did not honor playback cancellation")
	}
}

func TestPlaybackFailureSkipsEpisodeLookup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	want := errors.New("playback failed")
	err := startPlayback(ctx, cancel, func(context.Context) error { return want }, func(context.Context) {
		t.Error("episode lookup ran after playback failed")
	})
	if !errors.Is(err, want) || ctx.Err() != context.Canceled {
		t.Fatalf("error = %v, context = %v", err, ctx.Err())
	}
}

func TestPlaybackEpisodeLookupCompletionReleasesContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := startPlayback(ctx, cancel, func(context.Context) error { return nil }, func(context.Context) {})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("completed lookup retained playback context")
	}
}
