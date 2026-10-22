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

// Only inherit settings needed by the desktop and compatibility runtime. In
// particular, developer/cloud/model credentials and loader injection settings
// must not flow from a terminal or coding agent into downloaded executables.
// This is not a filesystem sandbox: Wine still runs with the user's privileges.
func environment(values map[string]string) []string {
	env := make([]string, 0, len(os.Environ())+len(values))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replace := values[key]; !replace && runtimeVariable(key) {
			env = append(env, entry)
		}
	}
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}

func runtimeVariable(key string) bool {
	switch key {
	case "HOME", "USER", "LOGNAME", "PATH", "SHELL", "TMPDIR", "LANG", "LANGUAGE", "TZ",
		"LC_ALL", "LC_ADDRESS", "LC_COLLATE", "LC_CTYPE", "LC_IDENTIFICATION", "LC_MEASUREMENT",
		"LC_MESSAGES", "LC_MONETARY", "LC_NAME", "LC_NUMERIC", "LC_PAPER", "LC_TELEPHONE", "LC_TIME",
		"DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY", "DBUS_SESSION_BUS_ADDRESS",
		"XDG_RUNTIME_DIR", "XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME",
		"XDG_DATA_DIRS", "XDG_CONFIG_DIRS", "XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP",
		"XDG_SESSION_TYPE", "XDG_SESSION_ID", "XDG_SEAT", "XDG_VTNR", "DESKTOP_SESSION",
		"HYPRLAND_INSTANCE_SIGNATURE", "PULSE_SERVER", "PULSE_SINK", "PULSE_SOURCE",
		"PIPEWIRE_REMOTE", "ALSA_CONFIG_PATH", "SDL_AUDIODRIVER", "SDL_VIDEODRIVER",
		"SDL_GAMECONTROLLERCONFIG", "SDL_GAMECONTROLLERCONFIG_FILE",
		"VK_ICD_FILENAMES", "VK_DRIVER_FILES", "VK_ADD_DRIVER_FILES", "VK_LAYER_PATH",
		"VK_IMPLICIT_LAYER_PATH", "VK_INSTANCE_LAYERS", "VK_LOADER_DRIVERS_SELECT",
		"VK_LOADER_DRIVERS_DISABLE", "DRI_PRIME", "MESA_LOADER_DRIVER_OVERRIDE",
		"MESA_VK_DEVICE_SELECT", "MESA_VK_DEVICE_SELECT_FORCE_DEFAULT_DEVICE",
		"__NV_PRIME_RENDER_OFFLOAD", "__GLX_VENDOR_LIBRARY_NAME", "__VK_LAYER_NV_optimus",
		"__GL_VRR_ALLOWED", "__GL_GSYNC_ALLOWED", "GBM_BACKEND", "LIBVA_DRIVER_NAME",
		"PROTON_LOG", "PROTON_NO_ESYNC", "PROTON_NO_FSYNC", "PROTON_USE_WINED3D",
		"PROTON_ENABLE_NVAPI", "PROTON_HIDE_NVIDIA_GPU", "PROTON_ENABLE_WAYLAND",
		"WINEDEBUG", "WINEESYNC", "WINEFSYNC", "UMU_LOG", "UMU_RUNTIME_UPDATE",
		"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
		"http_proxy", "https_proxy", "all_proxy", "no_proxy",
		"SSL_CERT_FILE", "SSL_CERT_DIR":
		return true
	}
	return false
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
