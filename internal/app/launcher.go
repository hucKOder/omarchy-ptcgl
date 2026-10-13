package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func (a *App) Play(ctx context.Context, args []string) error {
	if a.focusExisting(ctx) {
		a.progress("Focused the running game.")
		return nil
	}
	return a.withLock(func() error {
		if err := a.setup(ctx); err != nil {
			return err
		}
		a.progress("Launching Pokémon TCG Live…")
		launchArgs := []string{a.gamePath(), "-screen-fullscreen", "0", "-nolog"}
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
		if err := a.Runner.Run(ctx, c); err != nil {
			return fmt.Errorf("game exited: %w", err)
		}
		return nil
	})
}

var ErrCallback = errors.New("invalid Pokémon authentication callback")

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
	c := a.umu(session.Game, raw, "-nolog")
	c.Dir, c.Quiet = filepath.Dir(session.Game), true
	c.Env = environment(map[string]string{
		"GAMEID": "0", "WINEPREFIX": session.Prefix, "PROTONPATH": session.Proton,
		"PROTON_VERB": "runinprefix", "UMU_CONTAINER_NSENTER": "1",
		"UMU_LOG": "error", "PROTON_LOG": "0", "WINEDEBUG": "-all", "STEAM_LINUX_RUNTIME_LOG": "0",
	})
	if ctx.Err() != nil {
		return fmt.Errorf("authentication callback was cancelled")
	}
	if err := a.Runner.Start(c); err != nil {
		// Do not expose exec errors or arguments containing authorization material.
		return fmt.Errorf("authentication callback could not be delivered")
	}
	return nil
}

type Window struct {
	Address    string `json:"address"`
	Title      string `json:"title"`
	PID        int    `json:"pid"`
	XWayland   bool   `json:"xwayland"`
	Floating   bool   `json:"floating"`
	Fullscreen int    `json:"fullscreen"`
}

func gameTitle(title string) bool {
	return title == "Pokemon TCG Live" || title == "Pokémon TCG Live" || title == "Pokémon Trading Card Game Live" || title == "Pokemon Trading Card Game Live"
}

func (a *App) ownWindow(w Window) bool {
	if !gameTitle(w.Title) || !w.XWayland {
		return false
	}
	// A second PTCGL installation can have the same title/class. Focus ours only.
	env, _ := os.ReadFile(filepath.Join("/proc", strconv.Itoa(w.PID), "environ"))
	for _, item := range strings.Split(string(env), "\x00") {
		key, value, _ := strings.Cut(item, "=")
		if (key == "WINEPREFIX" || key == "STEAM_COMPAT_DATA_PATH") && samePrefix(value, a.Paths.Prefix) {
			return true
		}
	}
	cmdline, _ := os.ReadFile(filepath.Join("/proc", strconv.Itoa(w.PID), "cmdline"))
	return strings.Contains(string(cmdline), a.gamePath())
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
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	b, err := a.Runner.Output(ctx, Command{Name: "hyprctl", Args: []string{"clients", "-j"}})
	if err != nil {
		return false
	}
	var windows []Window
	if json.Unmarshal(b, &windows) != nil {
		return false
	}
	for _, w := range windows {
		if a.ownWindow(w) {
			dispatch := "hl.dsp.focus({ window = " + luaQuote("address:"+w.Address) + " })"
			return a.Runner.Run(ctx, Command{Name: "hyprctl", Args: []string{"dispatch", dispatch}, Quiet: true}) == nil
		}
	}
	return false
}
