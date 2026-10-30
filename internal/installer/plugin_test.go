package installer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestCompanionInstallation(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the companion installer requires a non-root desktop user")
	}
	helper, err := os.ReadFile("../../scripts/plugin-install.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, state, bootstrap string
		args                   []string
		wantCode               int
		wantRun                bool
		locked                 bool
	}{
		{name: "launcher only from a checkout with spaces", wantRun: true},
		{name: "bootstrap failure is preserved", bootstrap: "exit 23\n", wantRun: true, wantCode: 23},
		{name: "help does not install", args: []string{"--help"}},
		{name: "unexpected arguments do not install", args: []string{"--local"}, wantCode: 2},
		{name: "relative XDG state is rejected", state: "relative", wantCode: 1},
		{name: "concurrent companion installation is blocked", locked: true, wantCode: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			script := filepath.Join(root, "a checkout '$ with spaces", "scripts", "plugin-install.sh")
			fixture(t, script, string(helper), 0o600)
			log := filepath.Join(root, "arguments")
			fixture(t, filepath.Join(filepath.Dir(filepath.Dir(script)), "bootstrap.sh"),
				"#!/bin/bash\nprintf '%s\\n' \"$@\" > \"$TEST_LOG\"\n"+tt.bootstrap, 0o600)
			state := tt.state
			if state == "" {
				state = filepath.Join(root, "custom state")
			}
			if tt.locked {
				lockPath := filepath.Join(state, "ptcgl-omarchy", "companion-install.lock")
				fixture(t, lockPath, "", 0o600)
				lock, err := os.OpenFile(lockPath, os.O_RDWR, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				defer lock.Close()
				if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", append([]string{script}, tt.args...)...)
			cmd.Env = append(os.Environ(), "HOME="+root, "XDG_STATE_HOME="+state, "TEST_LOG="+log)
			output, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					code = exit.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if code != tt.wantCode {
				t.Fatalf("exit %d, want %d: %s", code, tt.wantCode, output)
			}
			arguments, err := os.ReadFile(log)
			if tt.wantRun {
				if err != nil || string(arguments) != "--launcher-only\n" {
					t.Fatalf("bootstrap arguments %q: %v", arguments, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("bootstrap unexpectedly ran: %q, %v", arguments, err)
			}
			if tt.wantCode != 0 && strings.Contains(string(output), "Launcher installed.") {
				t.Fatalf("failed installation reported success: %s", output)
			}
		})
	}
}
