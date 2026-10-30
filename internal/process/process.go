// Package process runs launcher subprocesses and restricts their environment.
package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type Command struct {
	Name      string
	Args, Env []string
	Dir       string
	Quiet     bool
}

type OSRunner struct{ Out, Err io.Writer }

const (
	terminationGrace   = 5 * time.Second
	terminationTimeout = terminationGrace + 5*time.Second
	outputDrainTimeout = 5 * time.Second
)

func (r OSRunner) command(c Command) *exec.Cmd {
	cmd := exec.Command(c.Name, c.Args...)
	cmd.Env, cmd.Dir = c.Env, c.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = outputDrainTimeout
	return cmd
}

func (r OSRunner) Run(ctx context.Context, c Command) error {
	cmd := r.command(c)
	if !c.Quiet {
		cmd.Stdout, cmd.Stderr = r.Out, r.Err
	}
	// Callback arguments can contain credentials: never wrap exec errors with arguments.
	return run(ctx, cmd)
}

func (r OSRunner) Output(ctx context.Context, c Command) ([]byte, error) {
	cmd := r.command(c)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := run(ctx, cmd)
	return output.Bytes(), err
}

// run keeps the group leader unreaped during cancellation so its process-group
// ID cannot be recycled while signals are sent to remaining group members.
func run(ctx context.Context, cmd *exec.Cmd) error {
	return runWithControl(ctx, cmd, defaultGroupControl())
}

func runWithControl(ctx context.Context, cmd *exec.Cmd, control groupControl) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	streams, err := prepareOutput(cmd)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		streams.closeAll()
		return err
	}
	streams.start()
	pid := cmd.Process.Pid
	exited := make(chan error, 1)
	go func() {
		var info unix.Siginfo
		var err error
		for {
			err = unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
			if !errors.Is(err, syscall.EINTR) {
				break
			}
		}
		exited <- err
	}()

	var observeErr error
	cancelled := false
	cleanup := func() error { return terminateGroup(pid, control) }
	// On unconfirmed cleanup do not block on a still-running direct child, or
	// reap its leader while cleanup is signaling. The waiter only reaps after
	// signaling has ended and the direct child really exits.
	unsafeReturn := func(err error, observed bool) error {
		streams.closeAll()
		streams.waitClosed()
		go func() {
			if !observed {
				_ = <-exited // CleanupError already reports the unsafe outcome.
			}
			_ = cmd.Wait()
		}()
		return err
	}
	select {
	case observeErr = <-exited:
	case <-ctx.Done():
		cancelled = true
		if err := cleanup(); err != nil {
			return unsafeReturn(err, false)
		}
		observeErr = <-exited
	}
	if !cancelled && ctx.Err() != nil {
		cancelled = true
		if err := cleanup(); err != nil {
			return unsafeReturn(err, true)
		}
	}
	outputErr := streams.wait(ctx, &cancelled, cleanup)
	var cleanupErr *CleanupError
	if errors.As(outputErr, &cleanupErr) {
		return unsafeReturn(outputErr, true)
	}
	waitErr := cmd.Wait()
	if observeErr != nil {
		return fmt.Errorf("observe subprocess exit: %w", observeErr)
	}
	if waitErr != nil {
		return waitErr
	}
	if cancelled {
		return ctx.Err()
	}
	return outputErr
}

type outputPipe struct {
	read, write *os.File
	target      io.Writer
}

type outputStreams struct {
	pipes     []outputPipe
	done      chan error
	remaining int
	mu        sync.Mutex
}

// Own the output pipes instead of letting Cmd.Wait reap the leader before
// inherited pipes close. This keeps the group ID pinned if cancellation arrives
// after the leader exits but a descendant still holds an output descriptor.
func prepareOutput(cmd *exec.Cmd) (*outputStreams, error) {
	streams := &outputStreams{done: make(chan error, 2)}
	for _, target := range []*io.Writer{&cmd.Stdout, &cmd.Stderr} {
		if *target == nil {
			continue
		}
		if _, ok := (*target).(*os.File); ok {
			continue
		}
		if len(streams.pipes) != 0 && reflect.ValueOf(*target).Comparable() && streams.pipes[0].target == *target {
			// Match exec.Cmd: identical writers share one pipe, preserving order.
			*target = streams.pipes[0].write
			continue
		}
		read, write, err := os.Pipe()
		if err != nil {
			streams.closeAll()
			return nil, err
		}
		streams.pipes = append(streams.pipes, outputPipe{read: read, write: write, target: *target})
		*target = write
	}
	return streams, nil
}

func (s *outputStreams) closeAll() {
	for _, pipe := range s.pipes {
		_ = pipe.read.Close()
		_ = pipe.write.Close()
	}
}

type synchronizedWriter struct {
	mu     *sync.Mutex
	target io.Writer
}

func (w synchronizedWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.target.Write(data)
}

func (s *outputStreams) start() {
	s.remaining = len(s.pipes)
	for _, pipe := range s.pipes {
		_ = pipe.write.Close()
		go func(pipe outputPipe) {
			_, err := io.Copy(synchronizedWriter{mu: &s.mu, target: pipe.target}, pipe.read)
			_ = pipe.read.Close()
			s.done <- err
		}(pipe)
	}
}

// Join the copy goroutines before returning output, including unsafe cleanup
// failures. Closing pipes alone does not synchronize a final in-flight Write.
func (s *outputStreams) waitClosed() {
	for s.remaining > 0 {
		_ = <-s.done
		s.remaining--
	}
}

func (s *outputStreams) wait(ctx context.Context, cancelled *bool, cleanup func() error) error {
	if len(s.pipes) == 0 {
		return nil
	}
	timer := time.NewTimer(outputDrainTimeout)
	defer timer.Stop()
	var firstErr error
	ctxDone := ctx.Done()
	if *cancelled {
		ctxDone = nil
	}
	for s.remaining > 0 {
		select {
		case err := <-s.done:
			if firstErr == nil {
				firstErr = err
			}
			s.remaining--
		case <-ctxDone:
			*cancelled = true
			ctxDone = nil
			if err := cleanup(); err != nil {
				return err
			}
		case <-timer.C:
			for _, pipe := range s.pipes {
				_ = pipe.read.Close()
			}
			firstErr = exec.ErrWaitDelay
		}
	}
	return firstErr
}

// CleanupError means cancellation could not confirm that the group stopped.
// Callers that serialize access to shared files MUST retain a persistent safety
// guard before releasing their lock, even if the direct child has exited.
type CleanupError struct {
	PGID int
	Err  error
}

func (e *CleanupError) Error() string {
	return fmt.Sprintf("cannot confirm subprocess group %d stopped; block further operations until reboot: %v", e.PGID, e.Err)
}

func (e *CleanupError) Unwrap() error { return e.Err }

type groupControl struct {
	observe func(int) (bool, error)
	signal  func(int, syscall.Signal) error
	grace   time.Duration
	timeout time.Duration
	poll    time.Duration
}

func defaultGroupControl() groupControl {
	return groupControl{
		observe: groupHasLiveProcesses,
		signal: func(pgid int, sig syscall.Signal) error {
			return syscall.Kill(-pgid, sig)
		},
		grace: terminationGrace, timeout: terminationTimeout, poll: 50 * time.Millisecond,
	}
}

// terminateGroup waits for visible live group members, including children that
// outlive the leader. It cannot cover processes that leave the group or are
// hidden from /proc. Observation/signaling failures are retried only until the
// hard deadline; inability to prove cleanup is an unsafe, actionable failure.
func terminateGroup(pgid int, control groupControl) error {
	var signalErr error
	signal := func(sig syscall.Signal) {
		if err := control.signal(pgid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
			signalErr = fmt.Errorf("signal subprocess group: %w", err)
		}
	}
	signal(syscall.SIGTERM)
	grace := time.NewTimer(control.grace)
	defer grace.Stop()
	deadline := time.NewTimer(control.timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(control.poll)
	defer ticker.Stop()
	killed := false
	for {
		live, err := control.observe(pgid)
		if err == nil && !live {
			return nil
		}
		select {
		case <-deadline.C:
			return &CleanupError{PGID: pgid, Err: errors.Join(errors.New("cleanup deadline exceeded"), err, signalErr)}
		case <-grace.C:
			signal(syscall.SIGKILL)
			killed = true
		case <-ticker.C:
			if killed {
				// Retry if a group member was not yet terminated on the first signal.
				signal(syscall.SIGKILL)
			}
		}
	}
}

func groupHasLiveProcesses(pgid int) (bool, error) {
	return (procReader{readDir: os.ReadDir, readFile: os.ReadFile}).groupHasLiveProcesses(pgid)
}

type procReader struct {
	readDir  func(string) ([]os.DirEntry, error)
	readFile func(string) ([]byte, error)
}

func (p procReader) groupHasLiveProcesses(pgid int) (bool, error) {
	entries, err := p.readDir("/proc")
	if err != nil {
		return true, fmt.Errorf("enumerate process group: %w", err)
	}
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		dir := filepath.Join("/proc", entry.Name())
		stat, err := p.readFile(filepath.Join(dir, "stat"))
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			continue
		}
		if err != nil {
			return true, err
		}
		fields, err := statFields(stat)
		if err != nil {
			return true, err
		}
		group, err := strconv.Atoi(fields[2])
		if err != nil {
			return true, err
		}
		if group != pgid {
			continue
		}
		if liveState(fields[0]) {
			return true, nil
		}
		// A zombie group leader may have exited only its main thread (e.g.
		// pthread_exit). Its workers can still execute and use the prefix.
		live, err := p.hasLiveThreads(dir)
		if err != nil || live {
			return live, err
		}
	}
	return false, nil
}

func statFields(stat []byte) ([]string, error) {
	end := bytes.LastIndexByte(stat, ')')
	if end < 0 {
		return nil, errors.New("invalid process stat")
	}
	fields := strings.Fields(string(stat[end+1:]))
	if len(fields) < 3 {
		return nil, errors.New("incomplete process stat")
	}
	return fields, nil
}

func liveState(state string) bool { return state != "Z" && state != "X" && state != "x" }

func (p procReader) hasLiveThreads(dir string) (bool, error) {
	tasks, err := p.readDir(filepath.Join(dir, "task"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return true, fmt.Errorf("enumerate subprocess threads: %w", err)
	}
	for _, task := range tasks {
		stat, err := p.readFile(filepath.Join(dir, "task", task.Name(), "stat"))
		if errors.Is(err, os.ErrNotExist) {
			continue // The thread exited between enumeration and reading.
		}
		if err != nil {
			return true, fmt.Errorf("read subprocess thread: %w", err)
		}
		fields, err := statFields(stat)
		if err != nil {
			return true, err
		}
		if liveState(fields[0]) {
			return true, nil
		}
	}
	// A worker can create another thread and then exit while its stat is read.
	// Do not report dead if the task set changed during this scan.
	after, err := p.readDir(filepath.Join(dir, "task"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return true, fmt.Errorf("recheck subprocess threads: %w", err)
	}
	if len(after) != len(tasks) {
		return true, nil
	}
	for i, task := range tasks {
		if task.Name() != after[i].Name() {
			return true, nil
		}
	}
	return false, nil
}

// Start launches a detached GUI helper and does not supervise it. Callback
// delivery must not use Start: its game-engine helper is expected to hand off
// the deep link and exit, and app.Callback bounds and cleans up that process.
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
func Environment(values map[string]string) []string {
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
