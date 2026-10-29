package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Status schema 1 is independent of doctor's existing JSON array protocol.
// Readiness is installation health, not online freshness or game activity.
type Status struct {
	SchemaVersion   int             `json:"schema_version"`
	LauncherVersion string          `json:"launcher_version"`
	Readiness       string          `json:"readiness"`
	Activity        string          `json:"activity"`
	Phase           string          `json:"phase"`
	Message         string          `json:"message"`
	Progress        *ProgressEvent  `json:"progress,omitempty"`
	Capabilities    Capabilities    `json:"capabilities"`
	Runtime         string          `json:"runtime"`
	CustomRuntime   bool            `json:"custom_runtime"`
	Installer       InstallerStatus `json:"installer"`
}

type Capabilities struct {
	Play        bool `json:"play"`
	Maintain    bool `json:"maintain"`
	Diagnostics bool `json:"diagnostics"`
	Folders     bool `json:"folders"`
}

type InstallerStatus struct {
	LastApplied string `json:"last_applied,omitempty"`
	Freshness   string `json:"freshness"`
	CheckedAt   string `json:"checked_at,omitempty"`
}

type InstallerCheck struct {
	SHA256    string    `json:"sha256"`
	CheckedAt time.Time `json:"checked_at"`
}

type ProgressEvent struct {
	Phase          string  `json:"phase"`
	Message        string  `json:"message"`
	Downloaded     int64   `json:"downloaded"`
	Total          int64   `json:"total"` // -1 means unknown; never invent a percentage.
	BytesPerSecond float64 `json:"bytes_per_second"`
	Cancellable    bool    `json:"cancellable"`
}

// The lock FD, process start ticks, and boot ID bind a snapshot to the actual
// flock owner. A recycled PID or an abandoned JSON file cannot imply activity.
type Operation struct {
	Kind     string        `json:"kind"`
	PID      int           `json:"pid"`
	Start    string        `json:"start"`
	Boot     string        `json:"boot"`
	LockFD   int           `json:"lock_fd"`
	Progress ProgressEvent `json:"progress"`
}

func processStart(pid int) (string, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	end := strings.LastIndexByte(string(b), ')')
	if end < 0 {
		return "", fmt.Errorf("invalid process identity")
	}
	fields := strings.Fields(string(b[end+1:]))
	if len(fields) < 20 {
		return "", fmt.Errorf("incomplete process identity")
	}
	return fields[19], nil
}

func newOperation(kind string, lock *os.File) (Operation, error) {
	start, err := processStart(os.Getpid())
	if err != nil {
		return Operation{}, err
	}
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return Operation{}, err
	}
	return Operation{Kind: kind, PID: os.Getpid(), Start: start, Boot: strings.TrimSpace(string(boot)), LockFD: int(lock.Fd()),
		Progress: ProgressEvent{Phase: "preparing", Message: "Preparing operation…", Total: -1}}, nil
}

func (o Operation) valid(p Paths) bool {
	if o.PID <= 0 || o.LockFD < 0 || o.Start == "" || o.Boot == "" {
		return false
	}
	start, err := processStart(o.PID)
	if err != nil || start != o.Start {
		return false
	}
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil || strings.TrimSpace(string(boot)) != o.Boot {
		return false
	}
	lock, err := os.Stat(filepath.Join(p.State, "session.lock"))
	if err != nil {
		return false
	}
	fd, err := os.Stat(fmt.Sprintf("/proc/%d/fd/%d", o.PID, o.LockFD))
	if err != nil || !os.SameFile(lock, fd) {
		return false
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/fdinfo/%d", o.PID, o.LockFD))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) >= 7 && f[0] == "lock:" && f[2] == "FLOCK" && f[4] == "WRITE" && f[5] == strconv.Itoa(o.PID) {
			return true
		}
	}
	return false
}

// Unlike acquireLock, this never creates files or cleans up stale markers.
func lockBusy(p Paths) (bool, error) {
	f, err := os.Open(filepath.Join(p.State, "session.lock"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if err == syscall.EWOULDBLOCK {
			return true, nil
		}
		return false, err
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false, nil
}

func readOperation(p Paths) (Operation, bool) {
	var op Operation
	b, err := os.ReadFile(filepath.Join(p.State, "operation.json"))
	if err != nil || json.Unmarshal(b, &op) != nil || !op.valid(p) {
		return op, false
	}
	switch op.Kind {
	case "play", "setup", "update", "repair", "check", "runtime-update", "maintenance":
	default:
		return op, false
	}
	return op, true
}

func unavailableStatus() Status {
	return Status{SchemaVersion: 1, LauncherVersion: Version, Readiness: "unavailable", Activity: "unavailable", Phase: "unknown",
		Message: "Status unavailable. Open diagnostics for details.", Runtime: "Unknown", Installer: InstallerStatus{Freshness: "not-checked"},
		Capabilities: Capabilities{Diagnostics: true, Folders: true}}
}

func (a *App) Status(ctx context.Context) Status {
	s := unavailableStatus()
	s.Readiness = "setup-needed"
	if a.gameReady() && a.prefixReady() && a.vcReady() && validateProton(a.protonPath()) == nil {
		s.Readiness = "ready"
	}
	if a.installPending() || (exists(a.gamePath()) && !a.gameReady()) {
		s.Readiness = "repair-needed"
	}
	s.Runtime = safeLabel(a.Profile.ProtonRelease)
	s.CustomRuntime = a.Config.ProtonPath != ""
	if s.CustomRuntime {
		s.Runtime = "Custom runtime"
	}
	s.Installer = a.installerStatus()
	busy, err := lockBusy(a.Paths)
	if err != nil {
		return s
	}
	window, windowErr := a.existingWindow(ctx)
	if windowErr != nil {
		return s
	}
	if busy {
		op, ok := readOperation(a.Paths)
		if !ok {
			s.Message = "Another session or maintenance operation is active; its status is unavailable."
			return s
		}
		// Recheck after reading, avoiding an already-finished operation snapshot.
		if stillBusy, err := lockBusy(a.Paths); err != nil || !stillBusy {
			return s
		}
		s.Activity, s.Phase, s.Message = op.Kind, op.Progress.Phase, op.Progress.Message
		s.Progress = &op.Progress
		if op.Kind == "play" {
			s.Activity = "starting"
			if op.Progress.Phase != "starting" && s.Readiness != "ready" {
				s.Activity = "setup"
			}
			if window != nil {
				s.Activity, s.Phase, s.Message = "running", "running", "Game is running. Focus returns to its window."
				s.Progress = nil
				s.Capabilities.Play = true
			}
		}
		return s
	}
	if window != nil {
		s.Activity, s.Phase, s.Message = "running", "running", "Game is running. Close it before maintenance."
		s.Capabilities.Play = true
		return s
	}
	s.Activity, s.Phase = "idle", "idle"
	s.Capabilities.Play, s.Capabilities.Maintain = true, true
	s.Message = "Ready to play."
	if s.Readiness == "setup-needed" {
		s.Message = "First launch prepares the runtime and installs the game."
	}
	if s.Readiness == "repair-needed" {
		s.Message = "Installation was interrupted. Repair the game before playing."
		s.Capabilities.Play = false
	}
	if a.Config.ProtonPath != "" && validateProton(a.protonPath()) != nil {
		s.Readiness, s.Message, s.Capabilities.Play = "unavailable", "The custom runtime needs attention. Open diagnostics.", false
	}
	return s
}

func (a *App) installerStatus() InstallerStatus {
	s := InstallerStatus{Freshness: "not-checked"}
	i := a.installation()
	if !i.AppliedAt.IsZero() {
		s.LastApplied = i.AppliedAt.UTC().Format(time.RFC3339)
	}
	var check InstallerCheck
	b, err := os.ReadFile(filepath.Join(a.Paths.State, "installer-check.json"))
	if err == nil && json.Unmarshal(b, &check) == nil && len(check.SHA256) == 64 && !check.CheckedAt.IsZero() {
		s.CheckedAt = check.CheckedAt.UTC().Format(time.RFC3339)
		s.Freshness = "available-at-check"
		if check.SHA256 == i.InstallerSHA256 {
			s.Freshness = "unchanged-at-check"
		}
	}
	return s
}

func (a *App) emit(event ProgressEvent) {
	a.mu.Lock()
	if a.operation != nil {
		a.operation.Progress = event
		// Status is advisory: failure to refresh it must not abort an MSI in flight.
		_ = writeJSON(filepath.Join(a.Paths.State, "operation.json"), a.operation)
	}
	a.mu.Unlock()
	if a.Progress != nil {
		a.Progress(event)
	}
}

func (a *App) phase(phase, message string) {
	a.progress(message)
	a.emit(ProgressEvent{Phase: phase, Message: message, Total: -1})
}

func (a *App) cancelCurrentDownload() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.downloadCancel == nil {
		return false
	}
	a.downloadCancel()
	return true
}
