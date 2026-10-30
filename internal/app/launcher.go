package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ptcgl-omarchy/internal/desktop"
	"ptcgl-omarchy/internal/process"
)

func (a *App) Play(ctx context.Context, args []string) error {
	return a.play(ctx, args, false)
}

func (a *App) play(ctx context.Context, args []string, feedback bool) error {
	window, err := a.existingWindow(ctx)
	if err != nil {
		return fmt.Errorf("cannot inspect game windows; open diagnostics before launching")
	}
	if window != nil {
		if err := a.focusWindow(ctx, *window); err != nil {
			return fmt.Errorf("the game is already running, but its window could not be focused; no second game was started")
		}
		a.progress("Focused the running game.")
		return nil
	}
	err = a.withOperation("play", func() error {
		if err := a.setup(ctx); err != nil {
			return err
		}
		a.phase("starting", "Starting Pokémon TCG Live…")
		launchArgs := append([]string{a.gamePath()}, gameWindowArgs()...)
		launchArgs = append(launchArgs, a.Config.ExtraArgs...)
		launchArgs = append(launchArgs, args...)
		c := a.umu(launchArgs...)
		// The updater uses relative paths: it must inherit the game directory.
		c.Dir = filepath.Dir(a.gamePath())
		marker := filepath.Join(a.Paths.State, "play-session.json")
		if err := writeJSON(marker, PlaySession{a.protonPath(), a.gamePath(), a.Paths.Prefix}); err != nil {
			return err
		}
		defer os.Remove(marker)
		if feedback {
			stop := a.startupFeedback(ctx)
			defer stop()
		}
		stopLayout := a.startLandscape(ctx)
		runErr := a.Runner.Run(ctx, c)
		layoutErr := stopLayout()
		if runErr != nil {
			runErr = fmt.Errorf("game exited: %w", runErr)
		}
		return errors.Join(runErr, layoutErr)
	})
	if errors.Is(err, ErrBusy) {
		if a.focusExisting(ctx) {
			return nil
		}
		if op, ok := readOperation(a.Paths); ok {
			if op.Kind == "play" && (op.Progress.Phase == "starting" || (a.gameReady() && a.prefixReady() && a.vcReady() && validateProton(a.protonPath()) == nil)) {
				return a.waitForExisting(ctx, 8*time.Second)
			}
			return fmt.Errorf("setup or maintenance is in progress; wait for it to finish: %w", err)
		}
	}
	return err
}

func (a *App) waitForExisting(ctx context.Context, limit time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if a.focusExisting(ctx) {
			return nil
		}
		if op, ok := readOperation(a.Paths); !ok || op.Kind != "play" {
			return fmt.Errorf("the previous startup ended; try Play again")
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("the game is still starting; wait briefly, then choose Focus game or open diagnostics")
		case <-ticker.C:
		}
	}
}

var ErrCallback = errors.New("invalid Pokémon authentication callback")

// A callback helper only needs to hand the deep link to the running client.
// It is expected to exit immediately after that handoff; this bound prevents a
// Unity/Wine shutdown hang from leaving an invisible second game running.
const defaultCallbackHelperTimeout = 30 * time.Second

type PlaySession struct{ Proton, Game, Prefix string }

func ValidateCallback(raw string) error {
	if len(raw) == 0 || len(raw) > 16384 {
		return ErrCallback
	}
	for _, r := range raw {
		if unicode.IsControl(r) {
			return ErrCallback
		}
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "tpcitcgapp") || !strings.EqualFold(u.Host, "callback") || u.User != nil {
		return ErrCallback
	}
	if _, err := url.ParseQuery(u.RawQuery); err != nil {
		return ErrCallback
	}
	return nil
}

func (a *App) Callback(ctx context.Context, raw string) error {
	if err := ValidateCallback(raw); err != nil {
		return err
	}
	// Require the active game session, avoiding an accidental new game process.
	l, err := acquireLock(a.Paths)
	if err == nil {
		releaseLock(l)
		return fmt.Errorf("start Pokémon TCG Live before signing in")
	}
	if !errors.Is(err, ErrBusy) {
		return fmt.Errorf("cannot find the running game session")
	}
	var session PlaySession
	b, err := os.ReadFile(filepath.Join(a.Paths.State, "play-session.json"))
	if err != nil || json.Unmarshal(b, &session) != nil || session.Prefix != a.Paths.Prefix || session.Game != a.gamePath() {
		return fmt.Errorf("start Pokémon TCG Live before signing in")
	}
	if !exists(session.Game) || validateProton(session.Proton) != nil {
		return fmt.Errorf("game runtime is unavailable")
	}
	callbackArgs := append([]string{session.Game, raw}, gameWindowArgs()...)
	c := a.umu(callbackArgs...)
	c.Dir, c.Quiet = filepath.Dir(session.Game), true
	c.Env = process.Environment(map[string]string{
		"GAMEID": "0", "WINEPREFIX": session.Prefix, "PROTONPATH": session.Proton,
		"PROTON_VERB": "runinprefix", "UMU_CONTAINER_NSENTER": "1",
		"UMU_LOG": "error", "PROTON_LOG": "0", "WINEDEBUG": "-all", "STEAM_LINUX_RUNTIME_LOG": "0",
	})
	if ctx.Err() != nil {
		return fmt.Errorf("authentication callback was cancelled")
	}
	timeout := a.callbackTimeout
	if timeout <= 0 {
		timeout = defaultCallbackHelperTimeout
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err = a.Runner.Run(timeoutCtx, c)
	if err == nil {
		return nil
	}
	var cleanupErr *process.CleanupError
	if errors.As(err, &cleanupErr) {
		return fmt.Errorf("authentication callback helper did not stop cleanly; close the game and open diagnostics: %w", cleanupErr)
	}
	if (errors.Is(err, context.DeadlineExceeded) || timeoutCtx.Err() == context.DeadlineExceeded) && ctx.Err() == nil {
		// The helper was started and had time to persist the deep link. It is
		// only failing to exit; do not turn that into a browser-visible failure.
		return nil
	}
	if errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return fmt.Errorf("authentication callback was cancelled")
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// The helper ran; its exit status is not the authentication result. The
		// game itself reports whether the callback was accepted.
		return nil
	}
	// Do not expose exec errors or arguments containing authorization material.
	return fmt.Errorf("authentication callback could not be delivered")
}

type Window struct {
	Address    string `json:"address"`
	Title      string `json:"title"`
	PID        int    `json:"pid"`
	XWayland   bool   `json:"xwayland"`
	Floating   bool   `json:"floating"`
	Fullscreen int    `json:"fullscreen"`
	Size       [2]int `json:"size"`
}

func gameTitle(title string) bool {
	return title == "Pokemon TCG Live" || title == "Pokémon TCG Live" || title == "Pokémon Trading Card Game Live" || title == "Pokemon Trading Card Game Live"
}

func (a *App) ownWindow(w Window) bool {
	if !gameTitle(w.Title) || !w.XWayland || w.PID <= 0 {
		return false
	}
	// A second PTCGL installation can have the same title/class. Focus ours only.
	env, _ := os.ReadFile(filepath.Join("/proc", strconv.Itoa(w.PID), "environ"))
	prefixFound := false
	for _, item := range strings.Split(string(env), "\x00") {
		key, value, _ := strings.Cut(item, "=")
		if key == "WINEPREFIX" || key == "STEAM_COMPAT_DATA_PATH" {
			if !samePrefix(value, a.Paths.Prefix) {
				return false
			}
			prefixFound = true
		}
	}
	if prefixFound {
		return true
	}
	cmdline, _ := os.ReadFile(filepath.Join("/proc", strconv.Itoa(w.PID), "cmdline"))
	for _, arg := range strings.Split(string(cmdline), "\x00") {
		if arg == a.gamePath() {
			return true
		}
	}
	return false
}

func samePrefix(path, prefix string) bool {
	if filepath.Clean(path) == filepath.Clean(prefix) {
		return true
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	expected, err := filepath.EvalSymlinks(prefix)
	return err == nil && resolved == expected
}

func (a *App) focusExisting(ctx context.Context) bool {
	w, err := a.existingWindow(ctx)
	if err != nil || w == nil {
		return false
	}
	return a.focusWindow(ctx, *w) == nil
}

func (a *App) focusWindow(ctx context.Context, w Window) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	dispatch := "hl.dsp.focus({ window = " + desktop.QuoteLua("address:"+w.Address) + " })"
	return a.Runner.Run(ctx, Command{Name: "hyprctl", Args: []string{"dispatch", dispatch}, Quiet: true})
}

func (a *App) existingWindow(ctx context.Context) (*Window, error) {
	windows, err := a.windows(ctx)
	if err != nil {
		return nil, err
	}
	for _, w := range windows {
		if a.ownWindow(w) {
			return &w, nil
		}
	}
	return nil, nil
}

func (a *App) windows(ctx context.Context) ([]Window, error) {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	b, err := a.Runner.Output(ctx, Command{Name: "hyprctl", Args: []string{"clients", "-j"}})
	if err != nil {
		return nil, err
	}
	var windows []Window
	if json.Unmarshal(b, &windows) != nil {
		return nil, fmt.Errorf("window status unavailable")
	}
	return windows, nil
}
