package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStatusIsReadOnlyAndIgnoresStaleMarkers(t *testing.T) {
	a, _ := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	s := a.Status(context.Background())
	if s.SchemaVersion != 1 || s.Activity != "idle" || s.Readiness != "setup-needed" || !s.Capabilities.Play {
		t.Fatalf("unexpected initial status: %+v", s)
	}
	if _, err := os.Stat(a.Paths.Home); !os.IsNotExist(err) {
		t.Fatal("status created application directories")
	}
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	marker := filepath.Join(a.Paths.State, "play-session.json")
	writeTestFile(t, marker, []byte("stale"))
	writeTestFile(t, filepath.Join(a.Paths.State, "operation.json"), []byte(`{"kind":"play","pid":1}`))
	s = a.Status(context.Background())
	if s.Activity != "idle" || s.Readiness != "ready" || !s.Capabilities.Maintain {
		t.Fatalf("stale marker became a running game: %+v", s)
	}
	b, _ := os.ReadFile(marker)
	if string(b) != "stale" {
		t.Fatal("status cleaned a marker")
	}
}

func TestStatusBindsActivityToTheActualLockOwner(t *testing.T) {
	a, _ := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	if err := a.withOperation("repair", func() error {
		a.phase("installing", "Repairing installation…")
		s := a.Status(context.Background())
		if s.Activity != "repair" || s.Phase != "installing" || s.Capabilities.Play || s.Capabilities.Maintain {
			t.Fatalf("unexpected active state: %+v", s)
		}
		op, ok := readOperation(a.Paths)
		if !ok {
			t.Fatal("actual lock owner was not recognized")
		}
		for _, change := range []func(*Operation){func(o *Operation) { o.Start = "reused-pid" }, func(o *Operation) { o.Boot = "previous-boot" }, func(o *Operation) { o.LockFD = 999999 }, func(o *Operation) { o.Kind = "unknown-operation" }} {
			bad := op
			change(&bad)
			if err := writeJSON(filepath.Join(a.Paths.State, "operation.json"), bad); err != nil {
				t.Fatal(err)
			}
			s = a.Status(context.Background())
			if s.Activity != "unavailable" || s.Capabilities.Maintain || s.Capabilities.Play {
				t.Fatalf("invalid owner trusted: %+v", s)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if s := a.Status(context.Background()); s.Activity != "idle" {
		t.Fatalf("completed operation still active: %+v", s)
	}
	if exists(filepath.Join(a.Paths.State, "operation.json")) {
		t.Fatal("operation marker retained")
	}
}

func TestUnknownBusySessionDoesNotEnableActions(t *testing.T) {
	a, _ := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	lock, err := acquireLock(a.Paths)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseLock(lock)
	s := a.Status(context.Background())
	if s.Activity != "unavailable" || s.Capabilities.Play || s.Capabilities.Maintain {
		t.Fatalf("unknown owner treated as idle: %+v", s)
	}
}

func ownedWindowFixture(t *testing.T, a *App) Window {
	t.Helper()
	cmd := exec.Command("/usr/bin/sleep", "30")
	cmd.Env = environment(map[string]string{"WINEPREFIX": a.Paths.Prefix})
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	w := Window{Address: "0x123", Title: "Pokemon TCG Live", PID: cmd.Process.Pid, XWayland: true}
	deadline := time.Now().Add(2 * time.Second)
	for !a.ownWindow(w) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !a.ownWindow(w) {
		t.Fatal("fixture did not expose its prefix")
	}
	return w
}

func TestOrphanedGameWindowBlocksMaintenance(t *testing.T) {
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test")
	w := ownedWindowFixture(t, a)
	r.output = func(Command) ([]byte, error) { return json.Marshal([]Window{w}) }
	s := a.Status(context.Background())
	if s.Activity != "running" || !s.Capabilities.Play || s.Capabilities.Maintain {
		t.Fatalf("orphaned window ignored: %+v", s)
	}
	if err := a.Update(context.Background(), false, false); !errors.Is(err, ErrBusy) {
		t.Fatalf("maintenance allowed against surviving game: %v", err)
	}
	for _, c := range r.commands {
		if c.Name == "umu-run" {
			t.Fatal("maintenance started Windows Installer")
		}
	}
}

func TestRepeatLaunchDuringStartupWaitsAndFocusesSameGame(t *testing.T) {
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test")
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	w := ownedWindowFixture(t, a)
	queries := 0
	r.output = func(Command) ([]byte, error) {
		queries++
		if queries < 6 {
			return []byte("[]"), nil
		}
		return json.Marshal([]Window{w})
	}
	r.run = func(c Command) error {
		if c.Name != "umu-run" {
			return nil
		}
		other, err := New(a.Paths, &bytes.Buffer{}, &bytes.Buffer{})
		if err != nil {
			return err
		}
		other.Runner, other.LookPath = r, a.LookPath
		if err := other.Play(context.Background(), nil); err != nil {
			return fmt.Errorf("repeat launch: %w", err)
		}
		return nil
	}
	if err := a.Play(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	launches, focuses := 0, 0
	for _, c := range r.commands {
		if c.Name == "umu-run" {
			launches++
		}
		if c.Name == "hyprctl" && c.Args[0] == "dispatch" {
			focuses++
		}
	}
	if launches != 1 || focuses != 1 {
		t.Fatalf("launches=%d focuses=%d", launches, focuses)
	}
}

func TestFailedFocusNeverStartsAnotherGame(t *testing.T) {
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
	w := ownedWindowFixture(t, a)
	r.output = func(Command) ([]byte, error) { return json.Marshal([]Window{w}) }
	r.run = func(Command) error { return errors.New("focus dispatcher unavailable") }
	if err := a.Play(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "no second game") {
		t.Fatalf("focus failure was ignored: %v", err)
	}
	for _, c := range r.commands {
		if c.Name == "umu-run" {
			t.Fatal("failed focus spawned another game")
		}
	}
	if _, err := os.Stat(a.Paths.Prefix); !os.IsNotExist(err) {
		t.Fatal("failed focus changed the game prefix")
	}
}

func TestUnknownWindowActivityBlocksPrefixChanges(t *testing.T) {
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
	r.output = func(Command) ([]byte, error) { return nil, errors.New("compositor unavailable") }
	for _, op := range []func() error{func() error { return a.Play(context.Background(), nil) }, func() error { return a.Setup(context.Background()) }, func() error { return a.Update(context.Background(), true, false) }} {
		if err := op(); err == nil || !strings.Contains(err.Error(), "cannot inspect game windows") {
			t.Fatalf("unknown activity allowed a prefix change: %v", err)
		}
	}
	if _, err := os.Stat(a.Paths.Prefix); !os.IsNotExist(err) {
		t.Fatal("unknown activity changed prefix")
	}
	for _, c := range r.commands {
		if c.Name == "umu-run" {
			t.Fatal("unknown activity launched Windows runtime")
		}
	}
}

func TestCLIStatusReturnsStructuredUnavailableWithoutLeakingConfig(t *testing.T) {
	a, _ := testApp(t)
	useTestPaths(t, a.Paths)
	log := stubZenity(t)
	writeTestFile(t, filepath.Join(a.Paths.Config, "config.json"), []byte(`{"proton_path":"tpcitcgapp://callback?code=SECRET"}`))
	var out, errOut bytes.Buffer
	if code := RunCLI(context.Background(), []string{"status", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("status exit %d: %s", code, &errOut)
	}
	var s Status
	if err := json.Unmarshal(out.Bytes(), &s); err != nil || s.Readiness != "unavailable" || s.SchemaVersion != 1 {
		t.Fatalf("invalid protocol: %s (%v)", &out, err)
	}
	if strings.Contains(out.String()+errOut.String(), "SECRET") {
		t.Fatal("status exposed config input")
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("status opened a graphical dialog")
	}
}
