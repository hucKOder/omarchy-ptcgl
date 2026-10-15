package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const dialogTimeout = 5 * time.Minute

func (a *App) WithProgress(ctx context.Context, operation func(context.Context) error) error {
	return a.withProgress(ctx, "Pokémon TCG Live", "Preparing Pokémon TCG Live…", operation)
}

func (a *App) withProgress(ctx context.Context, title, text string, operation func(context.Context) error) error {
	if err := a.require("zenity"); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zenity", "--progress", "--pulsate", "--auto-close", "--no-cancel", "--title="+title, "--text="+text, "--width=480")
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	finished := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		finished <- err
		if err != nil {
			cancel()
		}
	}()
	previous := a.Notify
	a.Notify = func(message string) { _, _ = fmt.Fprintln(pipe, "# "+strings.ReplaceAll(message, "\n", " ")) }
	err = operation(ctx)
	a.Notify = previous
	_, _ = fmt.Fprintln(pipe, "100")
	_ = pipe.Close()
	dialogErr := <-finished
	if err != nil {
		return err
	}
	if dialogErr != nil {
		return fmt.Errorf("progress dialog closed: %w", dialogErr)
	}
	return ctx.Err()
}

func (a *App) UpdateGUI(ctx context.Context, repair, checkOnly bool) error {
	title, text := "Update game", "Updating the official Pokémon installer…"
	if repair {
		title, text = "Repair game", "Repairing Pokémon TCG Live…"
	}
	if checkOnly {
		title, text = "Check installer updates", "Checking the current official Pokémon installer…"
	}
	title = "Pokémon TCG Live — " + title
	var outcome installerOutcome
	if err := a.withProgress(ctx, title, text, func(ctx context.Context) error {
		var err error
		outcome, err = a.update(ctx, repair, checkOnly)
		return err
	}); err != nil {
		return err
	}
	var message string
	switch outcome {
	case installerUnchanged:
		message = "Official installer unchanged.\n\nThe current installer matches the last applied installer. No reinstall was needed."
	case installerAvailable:
		message = "Installer update available.\n\nThe current official installer differs from the last applied installer. Choose Update game from the app's desktop actions, or run ptcgl-omarchy update to apply it."
	case installerApplied:
		message = "Update completed.\n\nThe current official installer was applied successfully."
	case installerRepaired:
		message = "Repair completed.\n\nThe current official installer was reapplied successfully."
	default:
		return fmt.Errorf("maintenance completed without an installer result")
	}
	message += "\n\nIn-game patches are checked by the game at launch."
	err := exec.CommandContext(ctx, "zenity", "--info", "--no-markup", "--title="+title, "--text="+message, "--width=500").Run()
	if dialogDismissed(err) {
		return nil
	}
	return err
}

func (a *App) Desktop(ctx context.Context) error {
	if !exists(a.gamePath()) || !a.prefixReady() || !a.vcReady() || validateProton(a.protonPath()) != nil {
		if err := a.WithProgress(ctx, a.Setup); err != nil {
			return err
		}
	}
	return a.Play(ctx, nil)
}

func ShowError(message string) {
	ctx, cancel := context.WithTimeout(context.Background(), dialogTimeout)
	defer cancel()
	_ = exec.CommandContext(ctx, "zenity", "--error", "--title=Pokémon TCG Live", "--text="+message, "--width=500").Run()
}

func ShowReport(ctx context.Context, report string) error {
	cmd := exec.CommandContext(ctx, "zenity", "--text-info", "--title=Pokémon TCG Live diagnostics", "--width=850", "--height=500")
	cmd.Stdin = strings.NewReader(report)
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if dialogDismissed(err) {
		return nil
	}
	return err
}

// Zenity returns status 1 when a dialog is cancelled or closed.
func dialogDismissed(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 1
}
