package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"ptcgl-omarchy/internal/process"
)

func TestUnconfirmedCleanupRetainsPersistentSafetyGuard(t *testing.T) {
	for _, mode := range []string{"run", "verification"} {
		t.Run(mode, func(t *testing.T) {
			a, runner := testApp(t)
			cleanupErr := &process.CleanupError{PGID: 123, Err: syscall.EMFILE}
			runner.run = func(Command) error { return cleanupErr }
			runner.output = func(Command) ([]byte, error) { return nil, cleanupErr }
			installer := filepath.Join(t.TempDir(), "installer.msi")
			writeTestFile(t, installer, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
			guard := filepath.Join(a.Paths.State, cleanupGuardName)
			err := a.withOperation("setup", func() error {
				// No file must need creating after observation starts failing.
				if data, err := os.ReadFile(guard); err != nil || !validBootID(strings.TrimSpace(string(data))) {
					t.Fatalf("safety guard was not prearmed: %q, %v", data, err)
				}
				if mode == "verification" {
					return a.verifyInstaller(context.Background(), installer)
				}
				return a.Runner.Run(context.Background(), Command{Name: "fixture"})
			})
			if !errors.Is(err, cleanupErr) || !strings.Contains(err.Error(), "safety guard retained") {
				t.Fatalf("got %v, want actionable unconfirmed cleanup", err)
			}
			if _, err := os.Stat(guard); err != nil {
				t.Fatalf("safety guard was removed: %v", err)
			}
			// The advisory lock itself may be released, but even a new App must
			// refuse prefix access while the prearmed persistent guard remains.
			b, err := New(a.Paths, io.Discard, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			err = b.withOperation("setup", func() error {
				t.Fatal("new operation ran after unsafe cleanup")
				return nil
			})
			if !errors.Is(err, errCleanupRequired) {
				t.Fatalf("new operation was not blocked: %v", err)
			}
			status := b.Status(context.Background())
			if status.Capabilities.Play || status.Capabilities.Maintain {
				t.Fatal("status advertised unsafe operations")
			}
		})
	}
}

func TestSafetyGuardClearedOnlyOnSafeReturnOrPreviousBoot(t *testing.T) {
	for _, mode := range []string{"success", "ordinary-error", "panic", "previous-boot", "invalid-guard"} {
		t.Run(mode, func(t *testing.T) {
			a, _ := testApp(t)
			guard := filepath.Join(a.Paths.State, cleanupGuardName)
			switch mode {
			case "previous-boot", "invalid-guard":
				value := "00000000-0000-0000-0000-000000000000\n"
				if mode == "invalid-guard" {
					value = "invalid"
				}
				writeTestFile(t, filepath.Join(a.Paths.State, "session.lock"), nil)
				writeTestFile(t, guard, []byte(value))
				_, _ = lockBusy(a.Paths)
				if _, err := os.Stat(guard); err != nil {
					t.Fatal("read-only status removed safety guard")
				}
			}
			func() {
				if mode == "panic" {
					defer func() {
						if recover() == nil {
							t.Fatal("fixture did not panic")
						}
					}()
				}
				err := a.withOperation("setup", func() error {
					if mode == "panic" {
						panic("fixture")
					}
					if mode == "ordinary-error" {
						return context.Canceled
					}
					return nil
				})
				if mode == "invalid-guard" && !errors.Is(err, errCleanupRequired) {
					t.Fatalf("invalid guard did not fail closed: %v", err)
				}
				if mode == "ordinary-error" && !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
				if (mode == "success" || mode == "previous-boot") && err != nil {
					t.Fatal(err)
				}
			}()
			_, err := os.Stat(guard)
			wantGuard := mode == "panic" || mode == "invalid-guard"
			if (err == nil) != wantGuard {
				t.Fatalf("guard stat=%v, wantGuard=%v", err, wantGuard)
			}
		})
	}
}

func TestOperationLockHeldUntilCancelledProcessGroupStops(t *testing.T) {
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	a, _ := testApp(t)
	a.Runner = OSRunner{Out: io.Discard, Err: io.Discard}
	dir := t.TempDir()
	pidFile, signaledFile := filepath.Join(dir, "child.pid"), filepath.Join(dir, "signaled")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := Command{Name: "/bin/sh", Args: []string{"-c", `trap 'echo signaled > "$2"; exit 0' TERM; /bin/sh -c 'trap "" TERM; echo $$ > "$1"; exec sleep 30' sh "$1" >/dev/null 2>&1 & wait`, "sh", pidFile, signaledFile}}
	done := make(chan error, 1)
	go func() {
		done <- a.withOperation("setup", func() error { return a.Runner.Run(ctx, command) })
	}()
	pidData := waitForProcessTestFile(t, pidFile)
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil || pid <= 0 {
		t.Fatalf("invalid fixture PID %q", pidData)
	}
	identity := processTestStartTime(pid)
	if identity == "" {
		t.Fatal("fixture process identity unavailable")
	}
	completed := false
	t.Cleanup(func() {
		if !completed && processTestStartTime(pid) == identity {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	cancel()
	waitForProcessTestFile(t, signaledFile)
	lock, err := acquireLock(a.Paths)
	if err == nil {
		releaseLock(lock)
		t.Fatal("operation released its lock while the descendant was still running")
	}
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("unexpected lock error: %v", err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled operation succeeded")
		}
		completed = true
	case <-time.After(15 * time.Second):
		t.Fatal("cancelled operation did not complete")
	}
	lock, err = acquireLock(a.Paths)
	if err != nil {
		t.Fatalf("lock remained held after process cleanup: %v", err)
	}
	releaseLock(lock)
}

func processTestStartTime(pid int) string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return ""
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return ""
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) <= 19 {
		return ""
	}
	return fields[19]
}

func waitForProcessTestFile(t *testing.T, path string) []byte {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return data
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fixture did not write %s", filepath.Base(path))
	return nil
}
