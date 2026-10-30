package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"ptcgl-omarchy/internal/config"
	"ptcgl-omarchy/internal/process"
)

var Version = "0.5.1"

type App struct {
	Paths    Paths
	Config   Config
	Profile  Profile
	Runner   Runner
	Download Downloader
	Out      io.Writer
	LookPath func(string) (string, error)
	// Callback helpers are expected to hand off the deep link and quit. Keep a
	// bound so a Unity/Wine shutdown hang cannot leave a second game window.
	callbackTimeout time.Duration
	// Progress is installed before an operation starts; callbacks receive no URLs,
	// command arguments, or subprocess output.
	Progress       func(ProgressEvent)
	mu             sync.Mutex
	operation      *Operation
	downloadCancel context.CancelFunc
}

func New(p Paths, out, errOut io.Writer) (*App, error) {
	c, err := config.LoadConfig(p)
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
	return &App{
		Paths: p, Config: c, Profile: profile, Runner: OSRunner{Out: out, Err: errOut},
		Download: NewDownloader(), Out: out, LookPath: exec.LookPath,
		callbackTimeout: defaultCallbackHelperTimeout,
	}, nil
}

func (a *App) progress(message string) {
	fmt.Fprintln(a.Out, message)
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

func (a *App) withOperation(kind string, fn func() error) (result error) {
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
	guard := filepath.Join(a.Paths.State, cleanupGuardName)
	if err := AtomicWrite(guard, []byte(op.Boot+"\n"), 0o600); err != nil {
		return fmt.Errorf("arm subprocess safety guard: %w", err)
	}
	// Deliberately not deferred: a panic/crash must retain the guard as well.
	result = fn()
	var cleanupErr *process.CleanupError
	if errors.As(result, &cleanupErr) {
		return fmt.Errorf("%w; safety guard retained at %s", result, guard)
	}
	if err := os.Remove(guard); err != nil {
		return errors.Join(result, fmt.Errorf("clear subprocess safety guard: %w", err))
	}
	return result
}

func (a *App) Setup(ctx context.Context) error {
	return a.withOperation("setup", func() error { return a.setup(ctx) })
}
