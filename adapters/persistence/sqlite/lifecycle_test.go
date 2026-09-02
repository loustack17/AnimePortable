// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/core"
)

func TestCloseWaitsForActiveOperationAndConcurrentClosers(t *testing.T) {
	store, err := Open(context.Background(), filepathForLifecycleTest(t))
	if err != nil {
		t.Fatal(err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	operationDone := make(chan error, 1)
	go func() {
		operationDone <- store.withDB(context.Background(), func(*sql.DB) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	const closerCount = 4
	closerStart := make(chan struct{})
	closerDone := make(chan error, closerCount)
	var closers sync.WaitGroup
	closers.Add(closerCount)
	for range closerCount {
		go func() {
			defer closers.Done()
			<-closerStart
			closerDone <- store.Close()
		}()
	}
	close(closerStart)

	select {
	case err := <-closerDone:
		t.Fatalf("Close returned while operation was active: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-operationDone; err != nil {
		t.Fatalf("active operation error = %v", err)
	}
	closers.Wait()
	for index := 0; index < closerCount; index++ {
		if err := <-closerDone; err != nil {
			t.Fatalf("Close[%d] error = %v", index, err)
		}
	}
	if err := store.withDB(context.Background(), func(*sql.DB) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Fatalf("operation after concurrent Close error = %v, want ErrClosed", err)
	}
}

func TestWithDBSanitizesWrappedCancellation(t *testing.T) {
	store, err := Open(context.Background(), filepathForLifecycleTest(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	wrapped := fmt.Errorf("driver detail: %w", context.Canceled)
	err = store.withDB(context.Background(), func(*sql.DB) error { return wrapped })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("wrapped cancellation error = %v, want context.Canceled", err)
	}
	if strings.Contains(err.Error(), "driver detail") {
		t.Fatalf("wrapped cancellation leaked driver detail: %v", err)
	}
}

func TestWithDBPreservesSafeSentinels(t *testing.T) {
	store, err := Open(context.Background(), filepathForLifecycleTest(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for _, sentinel := range []error{
		ErrInvalidInput,
		ErrIdentityConflict,
		ErrMigration,
		ErrStorage,
		ErrClosed,
		core.ErrNotFound,
	} {
		t.Run(sentinel.Error(), func(t *testing.T) {
			wrapped := fmt.Errorf("private detail: %w", sentinel)
			err := store.withDB(context.Background(), func(*sql.DB) error { return wrapped })
			if err != sentinel {
				t.Fatalf("error = %v, want exact sentinel %v", err, sentinel)
			}
			if strings.Contains(err.Error(), "private detail") {
				t.Fatalf("safe sentinel leaked private detail: %v", err)
			}
		})
	}
}

func TestOpenGateHonorsCancellation(t *testing.T) {
	openGate <- struct{}{}
	defer func() { <-openGate }()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := Open(ctx, filepathForLifecycleTest(t))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Open error = %v, want context.DeadlineExceeded", err)
	}
}

func filepathForLifecycleTest(t *testing.T) string {
	t.Helper()
	return t.TempDir() + "/anime.sqlite"
}
