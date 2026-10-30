package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type folder struct {
	id, label, path, missing string
}

func (a *App) folders() []folder {
	return []folder{
		{"game", "Game installation", filepath.Dir(a.gamePath()), "Run ptcgl-omarchy setup to install the game."},
		{"config", "Launcher configuration", a.Paths.Config, "This optional folder is used for config.json; create it when adding launch overrides."},
		{"cache", "Download cache", a.Paths.Cache, "This folder is created when setup or maintenance downloads files."},
	}
}

func (a *App) OpenFolder(ctx context.Context, target string) error {
	for _, f := range a.folders() {
		if f.id != target {
			continue
		}
		info, err := os.Stat(f.path)
		if os.IsNotExist(err) {
			return fmt.Errorf("%s folder does not exist at %q. %s", f.label, f.path, f.missing)
		}
		if err != nil {
			return fmt.Errorf("read %s folder: %w", f.label, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory: %q", f.label, f.path)
		}
		if _, err := a.LookPath("xdg-open"); err != nil {
			return fmt.Errorf("missing xdg-open; install it with: omarchy pkg add xdg-utils")
		}
		if err := a.Runner.Run(ctx, Command{Name: "xdg-open", Args: []string{f.path}}); err != nil {
			return fmt.Errorf("open %s folder: %w", f.label, err)
		}
		return nil
	}
	return fmt.Errorf("unknown folder %q; choose game, config, or cache", target)
}

func (a *App) ChooseFolder(ctx context.Context) error {
	if err := a.require("zenity"); err != nil {
		return err
	}
	args := []string{
		"--list", "--title=Pokémon TCG Live — Open folders", "--text=Choose a folder to open in your file manager.",
		"--column=ID", "--column=Folder", "--column=Location", "--hide-column=1", "--print-column=1", "--ok-label=Open", "--width=900", "--height=300",
	}
	for _, f := range a.folders() {
		args = append(args, f.id, f.label, f.path)
	}
	// GTK warnings on stderr must not become part of the selected ID.
	b, err := exec.CommandContext(ctx, "zenity", args...).Output()
	if dialogDismissed(err) && ctx.Err() == nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("choose folder: %w", err)
	}
	target := strings.TrimSpace(string(b))
	if target == "" {
		return nil
	}
	return a.OpenFolder(ctx, target)
}
