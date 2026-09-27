package main

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestActionQueuePreservesPlaybackOrder(t *testing.T) {
	queue := newActionQueue()
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var mu sync.Mutex
	var order []int
	queue.Enqueue(func() {
		close(firstStarted)
		<-releaseFirst
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
	})
	queue.Enqueue(func() {
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
	})
	<-firstStarted
	mu.Lock()
	if len(order) != 0 {
		t.Fatalf("later playback ran before the active operation: %v", order)
	}
	mu.Unlock()
	close(releaseFirst)
	select {
	case <-queue.Drain():
	case <-time.After(time.Second):
		t.Fatal("queued playback did not finish")
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(order, []int{1, 2}) {
		t.Fatalf("playback order = %v", order)
	}
}
