package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// This observer owns only its dialog. Its cancellation never reaches the game
// context, and Play waits for the observer before releasing the session lock.
func (a *App) startupFeedback(ctx context.Context) func() {
	observe, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-observe.Done():
			return
		case <-time.After(350 * time.Millisecond):
		}
		if w, err := a.existingWindow(observe); err == nil && w != nil {
			return
		}
		cmd := exec.CommandContext(observe, "zenity", "--progress", "--pulsate", "--auto-close", "--no-cancel", "--extra-button=Diagnostics", "--title=Pokémon TCG Live", "--text=Starting Pokémon TCG Live…", "--width=480")
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.WaitDelay = 2 * time.Second
		pipe, err := cmd.StdinPipe()
		if err != nil {
			return
		}
		if cmd.Start() != nil {
			_ = pipe.Close()
			return
		}
		// Extra-button output is a fixed label, never subprocess/game output.
		closed := make(chan error, 1)
		go func() { closed <- cmd.Wait() }()
		finish := func() {
			_, _ = io.WriteString(pipe, "100\n")
			_ = pipe.Close()
			select {
			case <-closed:
			case <-time.After(2 * time.Second):
				_ = cmd.Process.Kill()
				<-closed
			}
		}
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		slow := time.NewTimer(45 * time.Second)
		defer slow.Stop()
		for {
			select {
			case <-observe.Done():
				finish()
				return
			case <-closed:
				_ = pipe.Close()
				if strings.TrimSpace(output.String()) == "Diagnostics" {
					_ = a.Runner.Start(Command{Name: a.Paths.Bin, Args: []string{"doctor", "--gui"}})
				}
				return
			case <-slow.C:
				_, _ = io.WriteString(pipe, "# Startup is taking longer than expected. Open Diagnostics from the companion or app menu. The game has not been stopped.\n")
			case <-ticker.C:
				w, err := a.existingWindow(observe)
				if err == nil && w != nil {
					finish()
					// Never dispatch focus from this observer: the user may have
					// deliberately switched to another application during startup.
					return
				}
			}
		}
	}()
	return func() { cancel(); <-done }
}

func formatMiB(n int64) string { return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20)) }

func progressText(e ProgressEvent) string {
	if e.Phase != "downloading" {
		return e.Message
	}
	text := "Downloading: " + formatMiB(e.Downloaded)
	if e.Total > 0 {
		text += " / " + formatMiB(e.Total)
	} else {
		text += " (total size unknown)"
	}
	if e.BytesPerSecond > 0 {
		text += " · " + formatMiB(int64(e.BytesPerSecond)) + "/s"
	}
	return strings.ReplaceAll(text, "\n", " ")
}
