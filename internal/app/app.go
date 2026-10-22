package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

var Version = "0.4.1"

type App struct {
	Paths    Paths
	Config   Config
	Profile  Profile
	Runner   Runner
	Download Downloader
	Out      io.Writer
	Notify   func(string)
	LookPath func(string) (string, error)
}

func New(p Paths, out, errOut io.Writer) (*App, error) {
	c, err := LoadConfig(p)
	if err != nil {
		return nil, err
	}
	profile := DefaultProfile()
	if b, err := os.ReadFile(filepath.Join(p.State, "runtime.json")); err == nil {
		if err := json.Unmarshal(b, &profile); err != nil {
			return nil, fmt.Errorf("runtime.json: %w", err)
		}
		if profile.ProtonDirectory == "" || filepath.Base(profile.ProtonDirectory) != profile.ProtonDirectory || len(profile.ProtonSHA256) != 64 || profile.InstallerURL != DefaultProfile().InstallerURL {
			return nil, fmt.Errorf("invalid saved runtime profile")
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return &App{Paths: p, Config: c, Profile: profile, Runner: OSRunner{out, errOut},
		Download: NewDownloader(), Out: out, LookPath: exec.LookPath}, nil
}

func (a *App) progress(message string) {
	fmt.Fprintln(a.Out, message)
	if a.Notify != nil {
		a.Notify(message)
	}
}

func (a *App) require(names ...string) error {
	for _, name := range names {
		if _, err := a.LookPath(name); err != nil {
			return fmt.Errorf("missing %s; install dependencies with: omarchy pkg add umu-launcher zenity", name)
		}
	}
	return nil
}

func (a *App) withLock(fn func() error) error {
	l, err := acquireLock(a.Paths)
	if err != nil {
		return err
	}
	defer releaseLock(l)
	// A crash can leave a session marker, but never the advisory lock itself.
	if err := os.Remove(filepath.Join(a.Paths.State, "play-session.json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return fn()
}

func (a *App) Setup(ctx context.Context) error {
	return a.withLock(func() error { return a.setup(ctx) })
}
