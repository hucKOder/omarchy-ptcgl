package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var Version = "0.5.0"

type App struct {
	Paths    Paths
	Config   Config
	Profile  Profile
	Runner   Runner
	Download Downloader
	Out      io.Writer
	Notify   func(string)
	LookPath func(string) (string, error)
	// Progress is installed before an operation starts; callbacks receive no URLs,
	// command arguments, or subprocess output.
	Progress       func(ProgressEvent)
	mu             sync.Mutex
	operation      *Operation
	downloadCancel context.CancelFunc
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
	return a.withOperation("maintenance", fn)
}

func (a *App) withOperation(kind string, fn func() error) error {
	l, err := acquireLock(a.Paths)
	if err != nil {
		return err
	}
	defer releaseLock(l)
	// A launcher crash can release flock while Wine's window survives. Do not
	// modify that prefix until the owned game has actually closed.
	w, windowErr := a.existingWindow(context.Background())
	if w != nil {
		return ErrBusy
	}
	if windowErr != nil {
		switch kind {
		case "play", "setup", "update", "repair", "runtime-update":
			return fmt.Errorf("cannot inspect game windows; open diagnostics before changing game files")
		}
	}
	op, err := newOperation(kind, l)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.operation = &op
	err = writeJSON(filepath.Join(a.Paths.State, "operation.json"), op)
	a.mu.Unlock()
	if err != nil {
		a.mu.Lock()
		a.operation = nil
		a.mu.Unlock()
		return err
	}
	defer func() {
		a.mu.Lock()
		a.operation = nil
		_ = os.Remove(filepath.Join(a.Paths.State, "operation.json"))
		a.mu.Unlock()
	}()
	// A crash can leave a session marker, but never the advisory lock itself.
	if err := os.Remove(filepath.Join(a.Paths.State, "play-session.json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return fn()
}

func (a *App) Setup(ctx context.Context) error {
	return a.withOperation("setup", func() error { return a.setup(ctx) })
}
