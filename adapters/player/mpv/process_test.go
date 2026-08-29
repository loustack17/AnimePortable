package mpv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMPVHelperProcess(t *testing.T) {
	if os.Getenv("ANIMEPORTABLE_MPV_HELPER") != "1" {
		return
	}
	switch os.Getenv("ANIMEPORTABLE_MPV_HELPER_MODE") {
	case "success":
		return
	case "failure":
		os.Exit(7)
	case "wait":
		for {
			time.Sleep(time.Hour)
		}
	default:
		os.Exit(8)
	}
}

func TestStartUsesOnlyIdleArgumentAndReportsNaturalExit(t *testing.T) {
	var name string
	var args []string
	process, err := start(context.Background(), Executable{path: executableFixturePath()}, []string{"--idle=yes"}, launcherDeps{
		command: func(gotName string, gotArgs ...string) *exec.Cmd {
			name = gotName
			args = append([]string(nil), gotArgs...)
			return helperCommand("success")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != executableFixturePath() || len(args) != 1 || args[0] != "--idle=yes" {
		t.Fatalf("command = %q %q", name, args)
	}
	if process.PID() <= 0 {
		t.Fatalf("pid = %d", process.PID())
	}
	if err := process.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := process.Wait(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-process.Done():
	default:
		t.Fatal("done remained open after wait")
	}
}

func TestWaitReportsSanitizedFailureConcurrently(t *testing.T) {
	process, err := start(context.Background(), Executable{path: executableFixturePath()}, nil, launcherDeps{
		command: func(string, ...string) *exec.Cmd { return helperCommand("failure") },
	})
	if err != nil {
		t.Fatal(err)
	}
	errorsSeen := make(chan error, 8)
	var waiters sync.WaitGroup
	for range 8 {
		waiters.Add(1)
		go func() {
			defer waiters.Done()
			errorsSeen <- process.Wait()
		}()
	}
	waiters.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if !errors.Is(err, ErrExited) || err.Error() != ErrExited.Error() {
			t.Fatalf("wait error = %v", err)
		}
	}
}

func TestConcurrentCloseStopsAndReapsOnce(t *testing.T) {
	process, err := start(context.Background(), Executable{path: executableFixturePath()}, nil, launcherDeps{
		command: func(string, ...string) *exec.Cmd { return helperCommand("wait") },
		grace:   100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	errorsSeen := make(chan error, 16)
	var closers sync.WaitGroup
	for range 16 {
		closers.Add(1)
		go func() {
			defer closers.Done()
			errorsSeen <- process.Close()
		}()
	}
	closers.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := process.Wait(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-process.Done():
	default:
		t.Fatal("close returned before reap")
	}
}

func TestCloseEscalatesAfterGracePeriod(t *testing.T) {
	var stops atomic.Int32
	var kills atomic.Int32
	process, err := start(context.Background(), Executable{path: executableFixturePath()}, nil, launcherDeps{
		command: func(string, ...string) *exec.Cmd { return helperCommand("wait") },
		grace:   10 * time.Millisecond,
		stop: func(*os.Process) error {
			stops.Add(1)
			return nil
		},
		kill: func(process *os.Process) error {
			kills.Add(1)
			return process.Kill()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Close(); err != nil {
		t.Fatal(err)
	}
	if stops.Load() != 1 || kills.Load() != 1 {
		t.Fatalf("stops=%d kills=%d", stops.Load(), kills.Load())
	}
}

func TestCloseReturnsBoundedErrorWhenForceStopFails(t *testing.T) {
	process, err := start(context.Background(), Executable{path: executableFixturePath()}, nil, launcherDeps{
		command: func(string, ...string) *exec.Cmd { return helperCommand("wait") },
		grace:   10 * time.Millisecond,
		stop:    func(*os.Process) error { return nil },
		kill:    func(*os.Process) error { return errors.New("raw stop failure") },
	})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := process.Close(); !errors.Is(err, ErrStop) || err.Error() != ErrStop.Error() {
		t.Fatalf("close error = %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("close failure was not bounded")
	}
	if err := process.process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Fatal(err)
	}
	if err := process.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestRepeatedStartCloseCyclesRemainReaped(t *testing.T) {
	for range 12 {
		process, err := start(context.Background(), Executable{path: executableFixturePath()}, nil, launcherDeps{
			command: func(string, ...string) *exec.Cmd { return helperCommand("wait") },
			grace:   100 * time.Millisecond,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := process.Close(); err != nil {
			t.Fatal(err)
		}
		if err := process.Wait(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCloseRacingNaturalExitRemainsIdempotent(t *testing.T) {
	process, err := start(context.Background(), Executable{path: executableFixturePath()}, nil, launcherDeps{
		command: func(string, ...string) *exec.Cmd { return helperCommand("success") },
	})
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- process.Close() }()
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if err := process.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStartCancellationAndFailureDoNotLeaveProcesses(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	if process, err := start(canceled, Executable{path: executableFixturePath()}, nil, launcherDeps{
		command: func(string, ...string) *exec.Cmd {
			called = true
			return helperCommand("wait")
		},
	}); !errors.Is(err, context.Canceled) || process != nil || called {
		t.Fatalf("pre-canceled start = %v, %v, called=%v", process, err, called)
	}

	ctx, cancelDuringStart := context.WithCancel(context.Background())
	if process, err := start(ctx, Executable{path: executableFixturePath()}, nil, launcherDeps{
		command: func(string, ...string) *exec.Cmd {
			cancelDuringStart()
			return helperCommand("wait")
		},
		grace: 100 * time.Millisecond,
	}); !errors.Is(err, context.Canceled) || process != nil {
		t.Fatalf("raced cancellation = %v, %v", process, err)
	}

	if process, err := start(context.Background(), Executable{path: executableFixturePath()}, nil, launcherDeps{
		command: func(string, ...string) *exec.Cmd { return exec.Command("missing-animeportable-mpv-test-executable") },
	}); !errors.Is(err, ErrStart) || process != nil || err.Error() != ErrStart.Error() {
		t.Fatalf("start failure = %v, %v", process, err)
	}
}

func TestProcessZeroValuesAndFormattingAreSafe(t *testing.T) {
	var process *Process
	if process.PID() != 0 || process.Wait() != ErrStart || process.Close() != nil {
		t.Fatal("nil process methods are unsafe")
	}
	select {
	case <-process.Done():
	default:
		t.Fatal("nil process done channel was not closed")
	}
	value := &Process{}
	if fmt.Sprint(value) != "mpv.Process{redacted}" || fmt.Sprintf("%#v", value) != "mpv.Process{redacted}" {
		t.Fatal("process formatting exposed internals")
	}
	if started, err := Start(context.Background(), Executable{}); !errors.Is(err, ErrStart) || started != nil {
		t.Fatalf("zero executable start = %v, %v", started, err)
	}
}

func helperCommand(mode string) *exec.Cmd {
	command := exec.Command(os.Args[0], "-test.run=^TestMPVHelperProcess$")
	command.Env = append(os.Environ(), "ANIMEPORTABLE_MPV_HELPER=1", "ANIMEPORTABLE_MPV_HELPER_MODE="+mode)
	return command
}

func executableFixturePath() string {
	return os.Args[0]
}
