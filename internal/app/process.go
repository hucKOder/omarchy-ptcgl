package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"ptcgl-omarchy/internal/process"
)

type (
	Command  = process.Command
	OSRunner = process.OSRunner
)

// Runner is the subprocess behavior needed by the application.
type Runner interface {
	Run(context.Context, Command) error
	Output(context.Context, Command) ([]byte, error)
	Start(Command) error
}

var ErrBusy = errors.New("game or maintenance operation is running; close the game and retry")

const cleanupGuardName = "process-cleanup-guard"

var errCleanupRequired = errors.New("previous operation's subprocess cleanup is unconfirmed; reboot before retrying")

// A single lock covers setup, game lifetime, repair, and updates. Callback delivery
// deliberately does not acquire this lock: it must reach a game that holds it.
func acquireLock(p Paths) (*os.File, error) {
	if err := os.MkdirAll(p.State, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(p.State, "session.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("lock session: %w", err)
	}
	if err := checkCleanupGuard(p, true); err != nil {
		releaseLock(f)
		return nil, err
	}
	return f, nil
}

// A guard is armed before an operation can start subprocesses, not after a
// cleanup error: resource exhaustion may make writing a new marker impossible.
// Retain it on unconfirmed cleanup (or a launcher crash). A different boot proves
// all previous processes stopped; PID/PGID reuse on the same boot proves nothing.
func checkCleanupGuard(p Paths, clearStale bool) error {
	path := filepath.Join(p.State, cleanupGuardName)
	guard, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect subprocess safety guard: %w", err)
	}
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return fmt.Errorf("inspect safety guard boot identity: %w", err)
	}
	saved := strings.TrimSpace(string(guard))
	current := strings.TrimSpace(string(boot))
	if !validBootID(saved) || !validBootID(current) || saved == current {
		return fmt.Errorf("%w (safety guard: %s)", errCleanupRequired, path)
	}
	if clearStale {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("clear previous-boot safety guard: %w", err)
		}
	}
	return nil
}

func validBootID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func releaseLock(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }
