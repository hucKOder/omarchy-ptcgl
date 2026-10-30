package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"ptcgl-omarchy/internal/process"
)

type callbackRecordingRunner struct {
	*fakeRunner
	runCalls   int
	startCalls int
	runFn      func(context.Context, Command) error
}

func (r *callbackRecordingRunner) Run(ctx context.Context, c Command) error {
	r.runCalls++
	if r.runFn != nil {
		return r.runFn(ctx, c)
	}
	return r.fakeRunner.Run(ctx, c)
}

func (r *callbackRecordingRunner) Start(c Command) error {
	r.startCalls++
	return r.fakeRunner.Start(c)
}

// activeCallbackSession holds the Play lock and publishes the session marker
// that Callback requires, without launching a game process.
func activeCallbackSession(t *testing.T, a *App, fn func() error) error {
	t.Helper()
	return a.withOperation("play", func() error {
		if err := writeJSON(filepath.Join(a.Paths.State, "play-session.json"), PlaySession{a.protonPath(), a.gamePath(), a.Paths.Prefix}); err != nil {
			return err
		}
		return fn()
	})
}

func TestCallbackUsesBoundedSupervisedRun(t *testing.T) {
	a, base := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))

	runner := &callbackRecordingRunner{fakeRunner: base}
	var got Command
	var remaining time.Duration
	runner.runFn = func(ctx context.Context, c Command) error {
		got = c
		if deadline, ok := ctx.Deadline(); ok {
			remaining = time.Until(deadline)
		}
		return nil
	}
	a.Runner = runner

	err := activeCallbackSession(t, a, func() error {
		return a.Callback(context.Background(), "tpcitcgapp://callback?code=SECRET&state=SECRET")
	})
	if err != nil {
		t.Fatal(err)
	}
	if runner.runCalls != 1 || runner.startCalls != 0 {
		t.Fatalf("callback used run=%d start=%d", runner.runCalls, runner.startCalls)
	}
	if remaining < 20*time.Second || remaining > 31*time.Second {
		t.Fatalf("callback deadline = %s, want approximately %s", remaining, defaultCallbackHelperTimeout)
	}
	if !got.Quiet {
		t.Fatal("callback helper was not quiet")
	}
	if !reflect.DeepEqual(got.Args[2:], gameWindowArgs()) {
		t.Fatal("callback helper did not request the managed landscape window")
	}
	if got.Dir != filepath.Dir(a.gamePath()) {
		t.Fatal("callback helper has the wrong working directory")
	}
	if !contains(got.Env, "PROTON_VERB=runinprefix") || !contains(got.Env, "PROTON_LOG=0") {
		t.Fatal("callback helper was not isolated from logs or the running prefix")
	}
}

func TestCallbackHelperTimeoutIsTreatedAsHandoff(t *testing.T) {
	a, base := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	a.callbackTimeout = 20 * time.Millisecond

	runner := &callbackRecordingRunner{fakeRunner: base}
	runner.runFn = func(ctx context.Context, _ Command) error {
		<-ctx.Done()
		return ctx.Err()
	}
	a.Runner = runner

	started := time.Now()
	err := activeCallbackSession(t, a, func() error {
		return a.Callback(context.Background(), "tpcitcgapp://callback?code=SECRET")
	})
	if err != nil {
		t.Fatalf("clean helper timeout was reported as delivery failure: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("helper timeout took too long: %s", elapsed)
	}
}

func TestCallbackHelperExitStatusIsNotAuthenticationFailure(t *testing.T) {
	a, base := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))

	runner := &callbackRecordingRunner{fakeRunner: base}
	runner.runFn = func(context.Context, Command) error { return &exec.ExitError{} }
	a.Runner = runner

	err := activeCallbackSession(t, a, func() error {
		return a.Callback(context.Background(), "tpcitcgapp://callback?code=SECRET")
	})
	if err != nil {
		t.Fatalf("helper exit status was treated as delivery failure: %v", err)
	}
}

func TestCallbackCleanupFailureIsReportedWithoutSecrets(t *testing.T) {
	a, base := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))

	cleanup := &process.CleanupError{PGID: 123, Err: errors.New("fixture")}
	runner := &callbackRecordingRunner{fakeRunner: base}
	runner.runFn = func(context.Context, Command) error { return cleanup }
	a.Runner = runner

	err := activeCallbackSession(t, a, func() error {
		return a.Callback(context.Background(), "tpcitcgapp://callback?code=SECRET")
	})
	var got *process.CleanupError
	if !errors.As(err, &got) || got != cleanup {
		t.Fatalf("cleanup failure was not preserved: %v", err)
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Fatal("callback failure exposed authorization material")
	}
}

func TestCallbackCancellationDoesNotStartHelper(t *testing.T) {
	a, base := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))

	runner := &callbackRecordingRunner{fakeRunner: base}
	a.Runner = runner
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := activeCallbackSession(t, a, func() error {
		return a.Callback(ctx, "tpcitcgapp://callback?code=SECRET")
	})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("cancelled callback error = %v", err)
	}
	if runner.runCalls != 0 || runner.startCalls != 0 {
		t.Fatal("cancelled callback started a helper")
	}
}

// This uses the real process runner so timeout cleanup is exercised against a
// helper that ignores SIGTERM, not just a fake Runner.
func TestCallbackKillsHelperThatIgnoresTerm(t *testing.T) {
	a, _ := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))

	bin := t.TempDir()
	pidFile := filepath.Join(bin, "helper.pid")
	script := fmt.Sprintf("#!/bin/sh\ntrap '' TERM\necho $$ > %s\nwhile :; do sleep 1; done\n", strconv.Quote(pidFile))
	umu := filepath.Join(bin, "umu-run")
	if err := os.WriteFile(umu, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	a.Runner = OSRunner{Out: io.Discard, Err: io.Discard}
	a.callbackTimeout = 50 * time.Millisecond

	started := time.Now()
	err := activeCallbackSession(t, a, func() error {
		return a.Callback(context.Background(), "tpcitcgapp://callback?code=SECRET")
	})
	if err != nil {
		t.Fatalf("helper cleanup failed: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("helper cleanup took too long: %s", elapsed)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("helper did not record its PID: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("invalid helper PID: %v", err)
	}
	t.Cleanup(func() {
		if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err == nil {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}
	})
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); !os.IsNotExist(err) {
		t.Fatalf("helper process %d survived timeout cleanup: %v", pid, err)
	}
}
