package mpv

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"time"
)

var (
	ErrStart  = errors.New("mpv: unable to start; verify the configured executable")
	ErrExited = errors.New("mpv: process exited unsuccessfully")
	ErrStop   = errors.New("mpv: unable to stop process")
)

const defaultStopGrace = 2 * time.Second

var completed = func() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}()

type Process struct {
	process *os.Process
	done    chan struct{}
	grace   time.Duration
	stop    func(*os.Process) error
	kill    func(*os.Process) error

	mu        sync.Mutex
	closing   bool
	waitErr   error
	closeErr  error
	closeOnce sync.Once
}

type launcherDeps struct {
	command func(string, ...string) *exec.Cmd
	grace   time.Duration
	stop    func(*os.Process) error
	kill    func(*os.Process) error
}

func Start(ctx context.Context, executable Executable) (*Process, error) {
	return start(ctx, executable, []string{"--idle=yes"}, launcherDeps{})
}

func start(ctx context.Context, executable Executable, args []string, deps launcherDeps) (*Process, error) {
	if ctx == nil || executable.path == "" {
		return nil, ErrStart
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if deps.command == nil {
		deps.command = exec.Command
	}
	if deps.grace <= 0 {
		deps.grace = defaultStopGrace
	}
	if deps.stop == nil {
		deps.stop = gracefulStop
	}
	if deps.kill == nil {
		deps.kill = forceStop
	}
	command := deps.command(executable.path, append([]string(nil), args...)...)
	if command == nil || command.Start() != nil {
		return nil, ErrStart
	}
	process := &Process{
		process: command.Process,
		done:    make(chan struct{}),
		grace:   deps.grace,
		stop:    deps.stop,
		kill:    deps.kill,
	}
	go process.reap(command)
	if err := ctx.Err(); err != nil {
		_ = process.Close()
		return nil, err
	}
	return process, nil
}

func (process *Process) PID() int {
	if process == nil || process.process == nil {
		return 0
	}
	return process.process.Pid
}

func (process *Process) Done() <-chan struct{} {
	if process == nil || process.done == nil {
		return completed
	}
	return process.done
}

func (process *Process) Wait() error {
	if process == nil || process.done == nil {
		return ErrStart
	}
	<-process.done
	process.mu.Lock()
	defer process.mu.Unlock()
	return process.waitErr
}

func (process *Process) Close() error {
	if process == nil || process.done == nil {
		return nil
	}
	process.closeOnce.Do(func() {
		select {
		case <-process.done:
			return
		default:
		}
		process.mu.Lock()
		process.closing = true
		process.mu.Unlock()
		_ = process.stop(process.process)
		timer := time.NewTimer(process.grace)
		select {
		case <-process.done:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
			if err := process.kill(process.process); err != nil && !errors.Is(err, os.ErrProcessDone) {
				process.setCloseError()
				return
			}
			forceTimer := time.NewTimer(process.grace)
			select {
			case <-process.done:
				if !forceTimer.Stop() {
					select {
					case <-forceTimer.C:
					default:
					}
				}
			case <-forceTimer.C:
				process.setCloseError()
			}
		}
	})
	process.mu.Lock()
	defer process.mu.Unlock()
	return process.closeErr
}

func (process *Process) setCloseError() {
	process.mu.Lock()
	process.closeErr = ErrStop
	process.mu.Unlock()
}

func (process *Process) String() string { return "mpv.Process{redacted}" }

func (process *Process) GoString() string { return "mpv.Process{redacted}" }

func (process *Process) reap(command *exec.Cmd) {
	err := command.Wait()
	process.mu.Lock()
	if err != nil && !process.closing {
		process.waitErr = ErrExited
	}
	process.mu.Unlock()
	close(process.done)
}
