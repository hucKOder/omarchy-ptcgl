package process

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"testing/fstest"
	"time"
)

func testProcReader(files fstest.MapFS) procReader {
	name := func(path string) string {
		path = strings.TrimPrefix(strings.TrimPrefix(path, "/proc"), "/")
		if path == "" {
			return "."
		}
		return path
	}
	return procReader{
		readDir: func(path string) ([]os.DirEntry, error) {
			return fs.ReadDir(files, name(path))
		},
		readFile: func(path string) ([]byte, error) {
			return fs.ReadFile(files, name(path))
		},
	}
}

func TestZombieLeaderThreadObservation(t *testing.T) {
	for _, state := range []string{"R", "S", "D", "Z", "X", "x"} {
		t.Run(state, func(t *testing.T) {
			files := fstest.MapFS{
				"123/stat":          {Data: []byte("123 (name with ) spaces) Z 1 123")},
				"123/task/123/stat": {Data: []byte("123 (leader) Z 1 123")},
				"123/task/124/stat": {Data: []byte("124 (worker) " + state + " 1 123")},
			}
			reader := testProcReader(files)
			live, err := reader.groupHasLiveProcesses(123)
			wantLive := state == "R" || state == "S" || state == "D"
			if err != nil || live != wantLive {
				t.Fatalf("live=%v err=%v, want live=%v", live, err, wantLive)
			}
		})
	}
}

func TestThreadObservationRacesAndErrors(t *testing.T) {
	for _, mode := range []string{"vanished-thread", "vanished-process", "new-thread", "read-error", "enumeration-error", "recheck-error", "malformed-stat"} {
		t.Run(mode, func(t *testing.T) {
			files := fstest.MapFS{"123/task/123/stat": {Data: []byte("123 (leader) Z 1 123")}}
			reader := testProcReader(files)
			readFile, readDir := reader.readFile, reader.readDir
			reads := 0
			reader.readDir = func(path string) ([]os.DirEntry, error) {
				reads++
				switch {
				case mode == "vanished-process":
					return nil, os.ErrNotExist
				case mode == "enumeration-error", mode == "recheck-error" && reads > 1:
					return nil, syscall.EMFILE
				}
				return readDir(path)
			}
			reader.readFile = func(path string) ([]byte, error) {
				switch mode {
				case "vanished-thread":
					return nil, os.ErrNotExist
				case "new-thread":
					files["123/task/124/stat"] = &fstest.MapFile{Data: []byte("124 (worker) R 1 123")}
				case "read-error":
					return nil, syscall.EACCES
				case "malformed-stat":
					return []byte("broken"), nil
				}
				return readFile(path)
			}
			live, err := reader.hasLiveThreads("/proc/123")
			wantErr := strings.HasSuffix(mode, "error") || mode == "malformed-stat"
			if (err != nil) != wantErr {
				t.Fatalf("live=%v err=%v, wantErr=%v", live, err, wantErr)
			}
			if !wantErr && live != (mode == "new-thread") {
				t.Fatalf("unexpected live=%v", live)
			}
		})
	}
}

func fastGroupControl() groupControl {
	control := defaultGroupControl()
	control.grace, control.timeout, control.poll = 10*time.Millisecond, 100*time.Millisecond, time.Millisecond
	return control
}

func TestTerminationFailuresAreBounded(t *testing.T) {
	for _, mode := range []string{"enumeration", "read", "signal"} {
		t.Run(mode, func(t *testing.T) {
			control := fastGroupControl()
			var signals []syscall.Signal
			wantErr := error(syscall.EMFILE)
			control.signal = func(_ int, sig syscall.Signal) error {
				signals = append(signals, sig)
				if mode == "signal" {
					return syscall.EPERM
				}
				return nil
			}
			reader := procReader{
				readDir:  func(string) ([]os.DirEntry, error) { return nil, syscall.EMFILE },
				readFile: func(string) ([]byte, error) { return nil, syscall.EMFILE },
			}
			if mode == "read" {
				reader.readDir = func(string) ([]os.DirEntry, error) {
					return fs.ReadDir(fstest.MapFS{"123/stat": {}}, ".")
				}
			}
			control.observe = reader.groupHasLiveProcesses
			if mode == "signal" {
				control.observe = func(int) (bool, error) { return true, nil }
				wantErr = syscall.EPERM
			}
			start := time.Now()
			err := terminateGroup(123, control)
			var cleanupErr *CleanupError
			if !errors.As(err, &cleanupErr) || !errors.Is(err, wantErr) {
				t.Fatalf("got %v, want unsafe cleanup wrapping %v", err, wantErr)
			}
			if time.Since(start) > time.Second {
				t.Fatal("cleanup did not honor its hard deadline")
			}
			if len(signals) < 2 || signals[0] != syscall.SIGTERM || signals[1] != syscall.SIGKILL {
				t.Fatalf("missing escalation: %v", signals)
			}
		})
	}
}

func TestTerminationRetriesTransientFailures(t *testing.T) {
	control := fastGroupControl()
	observations := 0
	control.observe = func(int) (bool, error) {
		observations++
		if observations < 3 {
			return true, syscall.EMFILE
		}
		return false, nil
	}
	control.signal = func(int, syscall.Signal) error { return syscall.ESRCH }
	if err := terminateGroup(123, control); err != nil {
		t.Fatal(err)
	}
}

type readyWriter struct {
	once  sync.Once
	ready chan struct{}
}

func (w *readyWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.ready) })
	return len(p), nil
}

func TestUnconfirmedCleanupReturnsAndReapsDirectChild(t *testing.T) {
	for _, mode := range []string{"observation", "signal"} {
		t.Run(mode, func(t *testing.T) {
			control := fastGroupControl()
			wantErr := error(syscall.EMFILE)
			control.observe = func(int) (bool, error) { return true, syscall.EMFILE }
			if mode == "signal" {
				control = fastGroupControl()
				control.signal = func(int, syscall.Signal) error { return syscall.EPERM }
				wantErr = syscall.EPERM
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cmd := exec.Command("/bin/sh", "-c", "printf ready; exec sleep 30")
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			ready := &readyWriter{ready: make(chan struct{})}
			cmd.Stdout = ready
			done := make(chan error, 1)
			go func() { done <- runWithControl(ctx, cmd, control) }()
			select {
			case <-ready.ready:
			case <-time.After(5 * time.Second):
				t.Fatal("fixture did not start")
			}
			pid := cmd.Process.Pid
			identity := processStartTime(pid)
			if identity == "" {
				t.Fatal("fixture identity unavailable")
			}
			kill := func() {
				if processStartTime(pid) == identity {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			}
			t.Cleanup(kill)
			cancel()
			select {
			case err := <-done:
				var cleanupErr *CleanupError
				if !errors.As(err, &cleanupErr) || !errors.Is(err, wantErr) {
					t.Fatalf("got %v, want unconfirmed cleanup wrapping %v", err, wantErr)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("persistent cleanup error hung Run")
			}
			if mode == "signal" && !processIsLive(pid) {
				t.Fatal("fixture did not exercise returning with a still-live child")
			}
			kill()
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Stat("/proc/" + strconv.Itoa(pid)); errors.Is(err, os.ErrNotExist) {
					return
				}
				time.Sleep(time.Millisecond)
			}
			t.Fatal("direct child remained unreaped after unsafe cleanup error")
		})
	}
}

func TestUnconfirmedCleanupAfterLeaderExit(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	cmd := exec.Command("/bin/sh", "-c", `printf ready; /bin/sh -c 'trap "" TERM; echo $$ > "$1"; exec sleep 30' sh "$1" & exit 0`, "sh", pidFile)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	ready := &readyWriter{ready: make(chan struct{})}
	cmd.Stdout = ready
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	control := fastGroupControl()
	control.signal = func(int, syscall.Signal) error { return syscall.EPERM }
	done := make(chan error, 1)
	go func() { done <- runWithControl(ctx, cmd, control) }()
	select {
	case <-ready.ready:
	case <-time.After(5 * time.Second):
		t.Fatal("fixture did not start")
	}
	var pid int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(pidFile)
		pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if pid <= 0 {
		t.Fatal("descendant did not start")
	}
	identity := processStartTime(pid)
	if identity == "" {
		t.Fatal("descendant identity unavailable")
	}
	t.Cleanup(func() {
		if processStartTime(pid) == identity {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	for processIsLive(cmd.Process.Pid) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if processIsLive(cmd.Process.Pid) {
		t.Fatal("fixture leader did not exit")
	}
	cancel()
	select {
	case err := <-done:
		var cleanupErr *CleanupError
		if !errors.As(err, &cleanupErr) || !errors.Is(err, syscall.EPERM) {
			t.Fatalf("got %v, want unsafe output-drain cleanup failure", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("failed cleanup during output draining hung Run")
	}
	if !processIsLive(pid) {
		t.Fatal("fixture did not exercise a surviving descendant")
	}
}
