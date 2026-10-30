package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Run on the test binary's initial OS thread, before testing starts goroutines.
// SYS_EXIT (not exit_group) models pthread_exit without requiring a C compiler.
func init() {
	if os.Getenv("PTCGL_TEST_ZOMBIE_LEADER") != "1" {
		return
	}
	runtime.LockOSThread()
	signal.Ignore(syscall.SIGTERM)
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		close(ready)
		// Bound even a failed test's fixture lifetime.
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()
		<-timer.C
		os.Exit(0)
	}()
	<-ready
	if err := os.WriteFile(os.Getenv("PTCGL_TEST_PID_FILE"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		os.Exit(2)
	}
	// Syscall releases the runtime's P before exiting this thread; RawSyscall
	// would strand it and could prevent the worker goroutine from progressing.
	_, _, _ = unix.Syscall(unix.SYS_EXIT, 0, 0, 0)
	os.Exit(3)
}

type interfaceFieldWriter struct {
	value any
	out   *bytes.Buffer
}

func (w interfaceFieldWriter) Write(p []byte) (int, error) { return w.out.Write(p) }

func TestRunWithValueNonComparableWriter(t *testing.T) {
	var output bytes.Buffer
	writer := interfaceFieldWriter{value: []byte("not comparable"), out: &output}
	err := (OSRunner{Out: writer, Err: writer}).Run(context.Background(), Command{
		Name: "/bin/sh", Args: []string{"-c", "printf stdout; printf stderr >&2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "stdoutstderr" && got != "stderrstdout" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestCancellationStopsWorkersWithZombieLeader(t *testing.T) {
	for _, descendant := range []bool{false, true} {
		t.Run(fmt.Sprint("descendant=", descendant), func(t *testing.T) {
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			pidFile := filepath.Join(t.TempDir(), "worker.pid")
			command := Command{Name: executable, Env: append(os.Environ(), "PTCGL_TEST_ZOMBIE_LEADER=1", "PTCGL_TEST_PID_FILE="+pidFile)}
			if descendant {
				command.Name = "/bin/sh"
				command.Args = []string{"-c", `"$1" >/dev/null 2>&1 & wait`, "sh", executable}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- (OSRunner{}).Run(ctx, command) }()
			var pid int
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				data, _ := os.ReadFile(pidFile)
				pid, _ = strconv.Atoi(string(data))
				if pid > 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid <= 0 {
				t.Fatal("fixture did not start")
			}
			identity := processStartTime(pid)
			if identity == "" {
				t.Fatal("fixture identity unavailable")
			}
			t.Cleanup(func() {
				if processStartTime(pid) == identity {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			})
			sawZombie := false
			for time.Now().Before(deadline) {
				data, _ := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
				end := bytes.LastIndexByte(data, ')')
				if end >= 0 && strings.HasPrefix(string(data[end+1:]), " Z ") {
					sawZombie = true
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !sawZombie || !processIsLive(pid) {
				t.Fatal("fixture has no live worker threads")
			}
			cancel()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("cancelled command succeeded")
				}
				if processIsLive(pid) {
					t.Fatal("runner returned while zombie leader's workers survived")
				}
			case <-time.After(8 * time.Second):
				t.Fatal("cancellation did not escalate against zombie leader's workers")
			}
		})
	}
}

func TestEnvironmentFiltersSecretsAndOverrides(t *testing.T) {
	t.Setenv("HOME", "/original")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("PROTON_LOG", "1")
	got := Environment(map[string]string{"HOME": "/override", "WINEPREFIX": "/prefix"})
	joined := strings.Join(got, "\n")
	for _, want := range []string{"HOME=/override", "WINEPREFIX=/prefix", "PROTON_LOG=1"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, unwanted := range []string{"AWS_SECRET_ACCESS_KEY", "HOME=/original"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("unexpected environment entry %q", unwanted)
		}
	}
}

func TestRunHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := OSRunner{Out: io.Discard, Err: io.Discard}
	start := time.Now()
	err := r.Run(ctx, Command{Name: "sleep", Args: []string{"30"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context cancellation", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancelled command did not return promptly")
	}
}

func TestOutputPreservesCombinedOutputAndExitError(t *testing.T) {
	runner := OSRunner{}
	output, err := runner.Output(context.Background(), Command{Name: "/bin/sh", Args: []string{"-c", "printf stdout; printf stderr >&2; exit 3"}})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
		t.Fatalf("got %v, want exit code 3", err)
	}
	if string(output) != "stdoutstderr" {
		t.Fatalf("unexpected combined output: %q", output)
	}
}

func TestGracefulCancellationReturnsWithoutWaitingForEscalation(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- (OSRunner{Out: io.Discard, Err: io.Discard}).Run(ctx, Command{
			Name: "/bin/sh", Args: []string{"-c", `echo ready > "$1"; exec sleep 30`, "sh", ready},
		})
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(ready); err == nil && len(data) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatal("process did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled command succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("graceful shutdown waited for escalation")
	}
}

func TestCancellationStopsTermIgnoringDescendant(t *testing.T) {
	for _, mode := range []string{"run", "output"} {
		t.Run(mode, func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "child.pid")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runner := OSRunner{Out: io.Discard, Err: io.Discard}
			// The child ignores SIGTERM and closes inherited output descriptors. Its
			// parent exits on SIGTERM, exposing cleanup that waits only for the parent.
			command := Command{Name: "/bin/sh", Args: []string{"-c", `/bin/sh -c 'trap "" TERM; echo $$ > "$1"; exec sleep 30' sh "$1" >/dev/null 2>&1 & wait`, "sh", pidFile}}
			done := make(chan error, 1)
			go func() {
				if mode == "output" {
					_, err := runner.Output(ctx, command)
					done <- err
					return
				}
				done <- runner.Run(ctx, command)
			}()
			var pid int
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				data, err := os.ReadFile(pidFile)
				if err == nil {
					pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
					if err == nil && pid > 0 {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid <= 0 {
				t.Fatal("descendant did not start")
			}
			identity := processStartTime(pid)
			if identity == "" {
				t.Fatal("descendant identity unavailable")
			}
			t.Cleanup(func() {
				if processIsLive(pid) && processStartTime(pid) == identity {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			})
			cancel()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("cancelled command succeeded")
				}
			case <-time.After(15 * time.Second):
				t.Fatal("cancelled command did not return")
			}
			if processIsLive(pid) {
				t.Fatal("runner returned while descendant was still running")
			}
		})
	}
}

func TestCancellationAfterParentExitStopsDescendantHoldingOutput(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	parentFile := filepath.Join(filepath.Dir(pidFile), "parent.pid")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := OSRunner{}
	command := Command{Name: "/bin/sh", Args: []string{"-c", `echo $$ > "$2"; /bin/sh -c 'trap "" TERM; echo $$ > "$1"; exec sleep 30' sh "$1" & exit 0`, "sh", pidFile, parentFile}}
	done := make(chan error, 1)
	go func() {
		_, err := runner.Output(ctx, command)
		done <- err
	}()
	var pid int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
			if err == nil && pid > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid <= 0 {
		t.Fatal("descendant did not start")
	}
	identity := processStartTime(pid)
	if identity == "" {
		t.Fatal("descendant identity unavailable")
	}
	t.Cleanup(func() {
		if processIsLive(pid) && processStartTime(pid) == identity {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	// Wait until the direct parent has exited, while its child still holds stdout.
	parentData, err := os.ReadFile(parentFile)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := strconv.Atoi(strings.TrimSpace(string(parentData)))
	if err != nil || parent <= 0 {
		t.Fatalf("invalid parent PID %q", parentData)
	}
	deadline = time.Now().Add(5 * time.Second)
	for processIsLive(parent) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processIsLive(parent) {
		t.Fatal("fixture parent did not exit")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled output succeeded")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("cancelled output did not return")
	}
	if processIsLive(pid) {
		t.Fatal("runner returned while descendant was still running")
	}
}

func processIsLive(pid int) bool {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return !errors.Is(err, os.ErrNotExist)
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return true
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) == 0 || (fields[0] != "Z" && fields[0] != "X" && fields[0] != "x") {
		return true
	}
	tasks, err := os.ReadDir(filepath.Join("/proc", strconv.Itoa(pid), "task"))
	if err != nil {
		return !errors.Is(err, os.ErrNotExist)
	}
	for _, task := range tasks {
		data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "task", task.Name(), "stat"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return true
		}
		end := strings.LastIndexByte(string(data), ')')
		if end < 0 {
			return true
		}
		fields := strings.Fields(string(data[end+1:]))
		if len(fields) == 0 || (fields[0] != "Z" && fields[0] != "X" && fields[0] != "x") {
			return true
		}
	}
	return false
}

func processStartTime(pid int) string {
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
