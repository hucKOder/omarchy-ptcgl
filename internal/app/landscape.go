package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"ptcgl-omarchy/internal/desktop"
	"ptcgl-omarchy/internal/process"
)

// The desktop client has landscape menu/match scenes, but not their portrait
// variants. Unity's requested size alone cannot override a narrow compositor tile.
func gameWindowArgs() []string {
	return []string{"-screen-fullscreen", "0", "-screen-width", "1280", "-screen-height", "720", "-nolog"}
}

const landscapeWarning = "Landscape protection unavailable. Keep the game wider than tall before signing in or entering a match; open Diagnostics. The game has not been stopped."

// Own only this observer: stopping it must never cancel the game or its detached
// callback worker. Join before releasing the session lock, including cleanup errors.
func (a *App) startLandscape(ctx context.Context) func() error {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return func() error { return nil }
	}
	observe, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		done <- a.watchLandscape(observe, ticker.C)
	}()
	return func() error { cancel(); return <-done }
}

// A tick channel makes lifetime, cancellation and recovery tests independent of
// wall-clock sleeps. Continue after the first window: callbacks can open another
// window, and later menu/match transitions also select scenes by current aspect.
func (a *App) watchLandscape(ctx context.Context, ticks <-chan time.Time) error {
	var lastErr error
	for {
		if ctx.Err() != nil {
			return lastErr
		}
		err := a.keepLandscape(ctx)
		var cleanupErr *process.CleanupError
		if errors.As(err, &cleanupErr) {
			a.emit(ProgressEvent{Phase: "layout-warning", Message: landscapeWarning, Total: -1})
			return err // Do not issue more subprocesses after unconfirmed cleanup.
		}
		if ctx.Err() != nil {
			return lastErr
		}
		if err != nil && lastErr == nil {
			a.emit(ProgressEvent{Phase: "layout-warning", Message: landscapeWarning, Total: -1})
		} else if err == nil && lastErr != nil {
			a.emit(ProgressEvent{Phase: "starting", Message: "Landscape protection restored.", Total: -1})
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return lastErr
		case _, ok := <-ticks:
			if !ok {
				return lastErr
			}
		}
	}
}

func portrait(w Window) bool { return w.Size[0] > 0 && w.Size[1] > w.Size[0] }

func windowAddress(address string) bool {
	if !strings.HasPrefix(address, "0x") {
		return false
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(address, "0x"), 16, 64)
	return err == nil && n != 0
}

func (a *App) keepLandscape(ctx context.Context) error {
	windows, err := a.windows(ctx)
	if err != nil {
		return fmt.Errorf("inspect landscape layout: %w", err)
	}
	for _, w := range windows {
		if !portrait(w) || !a.ownWindow(w) {
			continue
		}
		if !windowAddress(w.Address) || w.PID <= 0 {
			return fmt.Errorf("landscape window identity unavailable")
		}
		if err := a.correctLandscape(ctx, w); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) correctLandscape(ctx context.Context, w Window) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	target := desktop.QuoteLua("address:" + w.Address)
	// Idempotent set/enable actions, never toggle or target the active window.
	steps := []string{}
	if w.Fullscreen != 0 {
		steps = append(steps, "hl.dsp.window.fullscreen_state({ internal = 0, client = 0, action = 'set', window = "+target+" })")
	}
	if !w.Floating {
		steps = append(steps, "hl.dsp.window.float({ action = 'enable', window = "+target+" })")
	}
	steps = append(steps,
		"hl.dsp.window.resize({ x = 1280, y = 720, relative = false, window = "+target+" })",
		"hl.dsp.window.set_prop({ prop = 'keep_aspect_ratio', value = '1', window = "+target+" })")
	for _, step := range steps {
		// Re-query before every mutation: a closed/replaced callback window must
		// not turn a stale address into a command against a different application.
		current, err := a.sameWindow(ctx, w)
		if err != nil || current == nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		output, err := a.Runner.Output(ctx, Command{Name: "hyprctl", Args: []string{"dispatch", step}, Quiet: true})
		if err != nil {
			return fmt.Errorf("correct landscape layout: %w", err)
		}
		if strings.TrimSpace(string(output)) != "ok" {
			// Never persist compositor output: other window titles may be private.
			return fmt.Errorf("compositor rejected landscape correction")
		}
	}
	current, err := a.sameWindow(ctx, w)
	if err != nil {
		return err
	}
	if current != nil && portrait(*current) {
		return fmt.Errorf("compositor did not apply landscape size")
	}
	return nil
}

func (a *App) sameWindow(ctx context.Context, expected Window) (*Window, error) {
	windows, err := a.windows(ctx)
	if err != nil {
		return nil, err
	}
	for _, w := range windows {
		if w.Address == expected.Address && w.PID == expected.PID && a.ownWindow(w) {
			return &w, nil
		}
	}
	return nil, nil
}
