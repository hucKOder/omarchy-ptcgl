package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (a *App) WithProgress(ctx context.Context, operation func(context.Context) error) error {
	if err := a.require("zenity"); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zenity", "--progress", "--pulsate", "--auto-close", "--no-cancel", "--title=Pokémon TCG Live", "--text=Preparing Pokémon TCG Live…", "--width=480")
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
	<-finished
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
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return nil
	}
	return err
}
