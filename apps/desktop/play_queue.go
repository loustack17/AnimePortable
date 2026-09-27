package main

import "sync"

type actionQueue struct {
	mu   sync.Mutex
	tail chan struct{}
}

func newActionQueue() *actionQueue {
	ready := make(chan struct{})
	close(ready)
	return &actionQueue{tail: ready}
}

func (queue *actionQueue) Enqueue(action func()) {
	queue.mu.Lock()
	previous := queue.tail
	done := make(chan struct{})
	queue.tail = done
	queue.mu.Unlock()
	go func() {
		<-previous
		defer close(done)
		action()
	}()
}

func (queue *actionQueue) Drain() <-chan struct{} {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return queue.tail
}
