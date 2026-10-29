package app

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
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
	events := make(chan ProgressEvent, 8)
	previous := a.Progress
	a.Progress = func(event ProgressEvent) {
		select {
		case events <- event:
		case <-ctx.Done():
		}
	}
	defer func() { a.Progress = previous }()
	dialog, err := newProgressDialog(ctx, title, text, false, false)
	if err != nil {
		return err
	}
	finished := make(chan error, 1)
	go func() { finished <- operation(ctx) }()
	var dialogErr error
	for {
		var closed <-chan error
		if dialog != nil {
			closed = dialog.done
		}
		select {
		case event := <-events:
			if dialogErr != nil {
				if event.Cancellable {
					a.cancelCurrentDownload()
				}
				continue
			}
			known := event.Phase == "downloading" && event.Total > 0
			if dialog != nil && (dialog.known != known || dialog.cancellable != event.Cancellable) {
				dialogErr = dialog.finish()
				dialog = nil
			}
			if dialogErr != nil {
				a.cancelCurrentDownload()
				continue
			}
			if dialog == nil {
				dialog, dialogErr = newProgressDialog(ctx, title, progressText(event), known, event.Cancellable)
				if dialogErr != nil {
					a.cancelCurrentDownload()
					continue
				}
			}
			_, _ = fmt.Fprintln(dialog.pipe, "# "+html.EscapeString(strings.ReplaceAll(progressText(event), "\n", " ")))
			if known {
				percent := min(99, int(float64(event.Downloaded)/float64(event.Total)*100))
				_, _ = fmt.Fprintln(dialog.pipe, max(0, percent))
			}
		case err := <-closed:
			_ = dialog.pipe.Close()
			dialog.cancel()
			dialog = nil
			if err == nil {
				err = fmt.Errorf("progress window dismissed")
			}
			dialogErr = err
			// Only an active network transfer can be cancelled. Never propagate a
			// dialog close into Windows Installer or the running game.
			a.cancelCurrentDownload()
		case err := <-finished:
			if dialog != nil {
				if closeErr := dialog.finish(); dialogErr == nil {
					dialogErr = closeErr
				}
			}
			if err != nil {
				return err
			}
			if dialogErr != nil {
				return fmt.Errorf("progress dialog closed; operation has finished, check diagnostics before retrying: %w", dialogErr)
			}
			return ctx.Err()
		}
	}
}

type progressDialog struct {
	pipe               io.WriteCloser
	done               chan error
	cancel             context.CancelFunc
	known, cancellable bool
}

func newProgressDialog(ctx context.Context, title, text string, known, cancellable bool) (*progressDialog, error) {
	ctx, cancel := context.WithCancel(ctx)
	args := []string{"--progress", "--auto-close", "--title=" + title, "--text=" + html.EscapeString(text), "--width=480"}
	if !known {
		args = append(args, "--pulsate")
	}
	if !cancellable {
		args = append(args, "--no-cancel")
	}
	cmd := exec.CommandContext(ctx, "zenity", args...)
	cmd.WaitDelay = 2 * time.Second
	pipe, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = pipe.Close()
		cancel()
		return nil, err
	}
	d := &progressDialog{pipe: pipe, done: make(chan error, 1), cancel: cancel, known: known, cancellable: cancellable}
	go func() { d.done <- cmd.Wait() }()
	return d, nil
}

func (d *progressDialog) finish() error {
	_, _ = io.WriteString(d.pipe, "100\n")
	_ = d.pipe.Close()
	defer d.cancel()
	select {
	case err := <-d.done:
		return err
	case <-time.After(2 * time.Second):
		d.cancel()
		return <-d.done
	}
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
	if a.focusExisting(ctx) {
		return nil
	}
	if !a.gameReady() || !a.prefixReady() || !a.vcReady() || validateProton(a.protonPath()) != nil {
		if err := a.WithProgress(ctx, a.Setup); err != nil {
			return err
		}
	}
	return a.play(ctx, nil, true)
}

func ShowError(message string) {
	ctx, cancel := context.WithTimeout(context.Background(), dialogTimeout)
	defer cancel()
	_ = exec.CommandContext(ctx, "zenity", "--error", "--no-markup", "--title=Pokémon TCG Live", "--text="+message, "--width=500").Run()
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
