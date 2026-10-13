package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type Command struct {
	Name      string
	Args, Env []string
	Dir       string
	Quiet     bool
}

type Runner interface {
	Run(context.Context, Command) error
	Output(context.Context, Command) ([]byte, error)
	Start(Command) error
}

type OSRunner struct{ Out, Err io.Writer }

func (r OSRunner) command(ctx context.Context, c Command) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	cmd.Env, cmd.Dir = c.Env, c.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second
	return cmd
}

func (r OSRunner) Run(ctx context.Context, c Command) error {
	cmd := r.command(ctx, c)
	if !c.Quiet {
		cmd.Stdout, cmd.Stderr = r.Out, r.Err
	}
	// Callback arguments can contain credentials: never wrap exec errors with arguments.
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (r OSRunner) Output(ctx context.Context, c Command) ([]byte, error) {
	return r.command(ctx, c).CombinedOutput()
}

// Login delivery can outlive the URI-handler process. Do not attach it to a
// timeout: a game that keeps its callback process alive must not be terminated.
func (r OSRunner) Start(c Command) error {
	cmd := exec.Command(c.Name, c.Args...)
	cmd.Env, cmd.Dir = c.Env, c.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if !c.Quiet {
		cmd.Stdout, cmd.Stderr = r.Out, r.Err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func environment(values map[string]string) []string {
	env := make([]string, 0, len(os.Environ())+len(values))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replace := values[key]; !replace {
			env = append(env, entry)
		}
	}
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}

var ErrBusy = errors.New("game or maintenance operation is running; close the game and retry")

// A single lock covers setup, game lifetime, repair, and updates. Callback delivery
// deliberately does not acquire this lock: it must reach a game that holds it.
func acquireLock(p Paths) (*os.File, error) {
	if err := os.MkdirAll(p.State, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(p.State, "session.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("lock session: %w", err)
	}
	return f, nil
}

func releaseLock(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }
