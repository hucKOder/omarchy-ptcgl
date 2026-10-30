package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"ptcgl-omarchy/internal/process"
)

// Simulate the compositor, not a second geometry observer. Every mutation must
// carry an exact address, and only a successful resize changes the reported size.
func layoutDesktop(r *fakeRunner, windows *[]Window) {
	r.output = func(c Command) ([]byte, error) {
		if reflect.DeepEqual(c.Args, []string{"clients", "-j"}) {
			return json.Marshal(*windows)
		}
		for i := range *windows {
			w := &(*windows)[i]
			if !strings.Contains(c.Args[1], "address:"+w.Address+"\"") {
				continue
			}
			switch {
			case strings.Contains(c.Args[1], "window.float("):
				w.Floating = true
			case strings.Contains(c.Args[1], "window.fullscreen_state("):
				w.Fullscreen = 0
			case strings.Contains(c.Args[1], "window.resize("):
				w.Size = [2]int{1280, 721} // XWayland may round by a pixel.
			}
		}
		return []byte("ok\n"), nil
	}
}

func layoutDispatches(r *fakeRunner) []Command {
	var commands []Command
	for _, c := range r.commands {
		if len(c.Args) == 2 && c.Args[0] == "dispatch" {
			commands = append(commands, c)
		}
	}
	return commands
}

func TestLandscapeLayoutNoOp(t *testing.T) {
	for _, size := range [][2]int{{1416, 1248}, {1280, 720}, {720, 720}, {0, 0}} {
		a, r := testApp(t)
		t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
		w := ownedWindowFixture(t, a)
		w.Size = size
		windows := []Window{w}
		layoutDesktop(r, &windows)
		if err := a.keepLandscape(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(layoutDispatches(r)) != 0 || windows[0].Floating {
			t.Fatalf("changed existing landscape/unknown layout: %v", size)
		}
	}
}

func TestLandscapeLayoutPreservesPrimaryAndCallbackWindows(t *testing.T) {
	for _, callbackFirst := range []bool{false, true} {
		t.Run(strconv.FormatBool(callbackFirst), func(t *testing.T) {
			a, r := testApp(t)
			t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
			primary := ownedWindowFixture(t, a)
			primary.Size, primary.Floating = [2]int{1280, 721}, true
			callback := ownedWindowFixture(t, a)
			callback.Address, callback.Size = "0x456", [2]int{701, 619}
			if primary.PID == callback.PID {
				t.Fatal("fixture reused the primary process")
			}
			// The live browser return left an independent tiled landscape helper
			// beside the floating primary. Its title/prefix do not prove it is
			// disposable, nor does its continued presence justify any mutation.
			windows := []Window{primary, callback}
			if callbackFirst {
				windows[0], windows[1] = windows[1], windows[0]
			}
			before := append([]Window(nil), windows...)
			layoutDesktop(r, &windows)
			for range 2 {
				if err := a.keepLandscape(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(windows, before) || len(r.commands) != 2 {
				t.Fatal("observer changed the primary or callback instead of only querying")
			}
			for _, c := range r.commands {
				if c.Name != "hyprctl" || !reflect.DeepEqual(c.Args, []string{"clients", "-j"}) {
					t.Fatal("observer issued a command other than a read-only window query")
				}
			}
		})
	}
}

func TestPortraitStartupAndLaterCallbackSceneWindows(t *testing.T) {
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
	w := ownedWindowFixture(t, a)
	w.Size = [2]int{701, 1248}
	windows := []Window{w}
	layoutDesktop(r, &windows)
	if err := a.keepLandscape(context.Background()); err != nil {
		t.Fatal(err)
	}
	if portrait(windows[0]) || !windows[0].Floating || len(layoutDispatches(r)) != 3 {
		t.Fatal("narrow startup did not float, resize and constrain mouse aspect")
	}
	// A new callback window must be handled even while the first remains open.
	secondary := ownedWindowFixture(t, a)
	secondary.Address, secondary.Floating, secondary.Size = "0x456", true, w.Size
	if secondary.PID == w.PID {
		t.Fatal("callback fixture reused the primary process")
	}
	windows = append(windows, secondary)
	if err := a.keepLandscape(context.Background()); err != nil {
		t.Fatal(err)
	}
	if portrait(windows[1]) || len(layoutDispatches(r)) != 5 {
		t.Fatal("secondary callback window was not corrected independently")
	}
	// A subsequent resize/fullscreen change before a match needs protection too.
	windows[0].Size, windows[0].Fullscreen = [2]int{600, 1000}, 2
	if err := a.keepLandscape(context.Background()); err != nil {
		t.Fatal(err)
	}
	if portrait(windows[0]) || windows[0].Fullscreen != 0 || len(layoutDispatches(r)) != 8 {
		t.Fatal("later portrait fullscreen/scene layout was ignored")
	}
	for _, c := range layoutDispatches(r) {
		if strings.Contains(c.Args[1], "toggle") || strings.Contains(c.Args[1], "focus(") || strings.Contains(c.Args[1], "workspace") {
			t.Fatal("layout correction toggled or changed focus/workspace")
		}
	}
}

func TestLandscapeOwnershipAndAddressFailures(t *testing.T) {
	for _, mode := range []string{"foreign-prefix", "wrong-title", "native", "dead-pid", "bad-address", "disappeared", "reused-address"} {
		t.Run(mode, func(t *testing.T) {
			a, r := testApp(t)
			t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
			w := ownedWindowFixture(t, a)
			w.Size = [2]int{701, 1248}
			switch mode {
			case "foreign-prefix":
				other, _ := testApp(t)
				t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
				w.PID = ownedWindowFixture(t, other).PID
			case "wrong-title":
				w.Title = "browser"
			case "native":
				w.XWayland = false
			case "dead-pid":
				w.PID = 0
			case "bad-address":
				w.Address = "all"
			}
			queries := 0
			r.output = func(Command) ([]byte, error) {
				queries++
				if queries > 1 && mode == "disappeared" {
					return []byte("[]"), nil
				}
				if queries > 1 && mode == "reused-address" {
					w.PID = os.Getpid()
				}
				return json.Marshal([]Window{w})
			}
			err := a.keepLandscape(context.Background())
			if (err != nil) != (mode == "bad-address") {
				t.Fatalf("unexpected identity result: %v", err)
			}
			if len(layoutDispatches(r)) != 0 {
				t.Fatal("mutated a window without verified ownership and address")
			}
		})
	}
}

func TestLandscapeCompositorFailures(t *testing.T) {
	for _, mode := range []string{"query", "json", "dispatch", "rejected", "clamped", "cleanup"} {
		t.Run(mode, func(t *testing.T) {
			a, r := testApp(t)
			t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
			w := ownedWindowFixture(t, a)
			w.Size = [2]int{701, 1248}
			cleanup := &process.CleanupError{PGID: 42, Err: errors.New("fixture")}
			r.output = func(c Command) ([]byte, error) {
				if c.Args[0] == "clients" {
					switch mode {
					case "query":
						return nil, errors.New("unavailable")
					case "json":
						return []byte("private malformed compositor output"), nil
					}
					return json.Marshal([]Window{w})
				}
				switch mode {
				case "dispatch":
					return nil, errors.New("unavailable")
				case "rejected":
					return []byte("private rejected dispatcher output"), nil
				case "cleanup":
					return nil, cleanup
				}
				return []byte("ok"), nil // Clamped size must not count as success.
			}
			err := a.keepLandscape(context.Background())
			if err == nil || strings.Contains(err.Error(), "private") {
				t.Fatalf("failure ignored or leaked compositor output: %v", err)
			}
			if mode == "cleanup" && !errors.Is(err, cleanup) {
				t.Fatal("lost subprocess cleanup safety error")
			}
		})
	}
}

func TestLandscapeObserverCancellationAndRecovery(t *testing.T) {
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
	ticks := make(chan time.Time, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queries := 0
	r.output = func(Command) ([]byte, error) {
		queries++
		if queries == 1 {
			return nil, errors.New("temporary compositor outage")
		}
		return []byte("[]"), nil
	}
	var phases []string
	a.Progress = func(e ProgressEvent) {
		phases = append(phases, e.Phase)
		if e.Phase == "layout-warning" {
			ticks <- time.Time{}
		} else {
			cancel()
		}
	}
	if err := a.watchLandscape(ctx, ticks); err != nil {
		t.Fatal(err)
	}
	if queries != 2 || !reflect.DeepEqual(phases, []string{"layout-warning", "starting"}) {
		t.Fatalf("observer did not recover: queries=%d phases=%v", queries, phases)
	}
	if err := a.watchLandscape(ctx, ticks); err != nil || queries != 2 {
		t.Fatal("cancelled observer issued a compositor command")
	}
}

func TestLandscapeObserverStopsOnUnconfirmedCleanup(t *testing.T) {
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
	cleanup := &process.CleanupError{PGID: 42, Err: errors.New("fixture")}
	r.output = func(Command) ([]byte, error) { return nil, cleanup }
	if err := a.watchLandscape(context.Background(), make(chan time.Time)); !errors.Is(err, cleanup) {
		t.Fatalf("observer lost cleanup error: %v", err)
	}
	if len(r.commands) != 1 {
		t.Fatal("observer continued after unconfirmed cleanup")
	}
}

func TestPlayRetainsSafetyGuardForLayoutSubprocess(t *testing.T) {
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	launched, warned := make(chan struct{}), make(chan struct{})
	cleanup := &process.CleanupError{PGID: 42, Err: errors.New("fixture")}
	r.output = func(Command) ([]byte, error) {
		select {
		case <-launched:
			return nil, cleanup
		default:
			return []byte("[]"), nil
		}
	}
	r.run = func(c Command) error {
		if c.Name == "umu-run" {
			close(launched)
			<-warned // The fake game stays alive until its observer has failed.
		}
		return nil
	}
	a.Progress = func(e ProgressEvent) {
		if e.Phase == "layout-warning" {
			close(warned)
		}
	}
	if err := a.Play(context.Background(), nil); !errors.Is(err, cleanup) {
		t.Fatalf("Play lost observer cleanup failure: %v", err)
	}
	if !exists(filepath.Join(a.Paths.State, cleanupGuardName)) {
		t.Fatal("layout subprocess cleanup failure cleared the safety guard")
	}
	if exists(filepath.Join(a.Paths.State, "play-session.json")) {
		t.Fatal("finished Play retained the callback session marker")
	}
}

type cancelLayoutRunner struct {
	*fakeRunner
	entered chan struct{}
}

func (r cancelLayoutRunner) Output(ctx context.Context, _ Command) ([]byte, error) {
	close(r.entered)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestStoppingLayoutObserverJoinsWithoutCancellingGame(t *testing.T) {
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
	entered := make(chan struct{})
	a.Runner = cancelLayoutRunner{fakeRunner: r, entered: entered}
	game, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := a.startLandscape(game)
	<-entered
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if game.Err() != nil {
		t.Fatal("stopping observer cancelled the game context")
	}
}

func TestWindowOwnershipRejectsConflictingPrefix(t *testing.T) {
	a, _ := testApp(t)
	cmd := exec.Command("bash", "-c", `exec -a "$0" sleep 30`, a.gamePath())
	cmd.Env = process.Environment(map[string]string{"WINEPREFIX": filepath.Join(t.TempDir(), "foreign")})
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	w := Window{Title: "Pokemon TCG Live", XWayland: true, PID: cmd.Process.Pid}
	if a.ownWindow(w) {
		t.Fatal("game-path argument overrode explicit foreign prefix")
	}
}

func TestLaunchAndCallbackRequestSameLandscapeWithoutLogging(t *testing.T) {
	a, r := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	r.run = func(c Command) error {
		if len(c.Args) > 1 && c.Args[1] == "-screen-fullscreen" {
			return a.Callback(context.Background(), "tpcitcgapp://callback?code=fixture")
		}
		return nil
	}
	if err := a.Play(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	launches := 0
	for _, c := range r.commands {
		if c.Name != "umu-run" {
			continue
		}
		launches++
		if !reflect.DeepEqual(c.Args[len(c.Args)-len(gameWindowArgs()):], gameWindowArgs()) {
			t.Fatal("launch/callback geometry or privacy flags differ")
		}
	}
	if launches != 2 {
		t.Fatalf("launch and callback count = %d", launches)
	}
}
