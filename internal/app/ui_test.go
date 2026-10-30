package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Keep dialog tests off the real desktop.
func stubZenity(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(bin, "dialogs.log")
	script := `#!/bin/sh
exec 3>> "$TEST_DIALOG_LOG"
printf 'BEGIN %s\n' "$1" >&3
printf '%s\n' "$@" >&3
case "$1" in
  --progress)
    no_cancel=0
    for arg in "$@"; do
      [ "$arg" = --no-cancel ] && no_cancel=1
    done
    if [ "${TEST_CANCEL_DOWNLOAD:-0}" = 1 ] && [ "$no_cancel" = 0 ]; then exit 1; fi
    while IFS= read -r line; do
      printf 'progress: %s\n' "$line" >&3
    done
    printf 'END --progress\n' >&3
    exit "${TEST_PROGRESS_EXIT:-0}"
    ;;
  --info) exit "${TEST_INFO_EXIT:-0}" ;;
  --list)
    printf '%s' "$TEST_FOLDER_STDERR" >&2
    printf '%s\n' "$TEST_FOLDER_SELECTION"
    exit "$TEST_FOLDER_EXIT"
    ;;
  --error) exit 0 ;;
  --question) exit "${TEST_QUESTION_EXIT:-0}" ;;
  --file-selection)
    printf '%s\n' "$TEST_REPORT_FILE"
    exit "${TEST_FILE_EXIT:-0}"
    ;;
  --text-info)
    while IFS= read -r line; do printf 'report: %s\n' "$line" >&3; done
    if [ -n "$TEST_TEXT_ACTION" ] && [ ! -e "$TEST_DIALOG_LOG.action" ]; then
      : > "$TEST_DIALOG_LOG.action"
      printf '%s\n' "$TEST_TEXT_ACTION"
    fi
    exit 0
    ;;
  *) exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "zenity"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("TEST_DIALOG_LOG", log)
	t.Setenv("TEST_PROGRESS_EXIT", "0")
	t.Setenv("TEST_INFO_EXIT", "0")
	t.Setenv("TEST_FOLDER_STDERR", "")
	t.Setenv("TEST_FOLDER_SELECTION", "")
	t.Setenv("TEST_FOLDER_EXIT", "0")
	t.Setenv("TEST_QUESTION_EXIT", "0")
	t.Setenv("TEST_REPORT_FILE", "")
	t.Setenv("TEST_TEXT_ACTION", "")
	t.Setenv("TEST_FILE_EXIT", "0")
	t.Setenv("TEST_CANCEL_DOWNLOAD", "0")
	return log
}

func maintenanceApp(t *testing.T) (*App, *fakeRunner, string) {
	t.Helper()
	a, r := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	body := []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(server.Close)
	a.Download = testDownloadClient(server)
	a.Profile.InstallerURL = server.URL
	return a, r, fmt.Sprintf("%x", sha256.Sum256(body))
}

func readDialogLog(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGraphicalMaintenanceResults(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		repair, check, sameInstaller bool
		title, text, result          string
		installerRuns                int
	}{
		{
			name:          "check unchanged",
			check:         true,
			sameInstaller: true,
			title:         "Check installer updates",
			text:          "Checking the current official",
			result:        "Official installer unchanged.",
		},
		{
			name:   "check changed",
			check:  true,
			title:  "Check installer updates",
			text:   "Checking the current official",
			result: "Installer update available.",
		},
		{
			name:          "update unchanged",
			sameInstaller: true,
			title:         "Update game",
			text:          "Updating the official",
			result:        "Official installer unchanged.",
		},
		{
			name:          "update changed",
			title:         "Update game",
			text:          "Updating the official",
			result:        "Update completed.",
			installerRuns: 1,
		},
		{
			name:          "repair unchanged",
			repair:        true,
			sameInstaller: true,
			title:         "Repair game",
			text:          "Repairing Pokémon TCG Live",
			result:        "Repair completed.",
			installerRuns: 1,
		},
		{
			name:          "repair changed",
			repair:        true,
			title:         "Repair game",
			text:          "Repairing Pokémon TCG Live",
			result:        "Repair completed.",
			installerRuns: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, r, hash := maintenanceApp(t)
			log := stubZenity(t)
			if !tc.sameInstaller {
				hash = "previous installer"
			}
			receipt := filepath.Join(a.Paths.State, "installation.json")
			if err := writeJSON(receipt, Installation{InstallerSHA256: hash, AppliedAt: time.Now().UTC(), Operation: "setup"}); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(receipt)
			if err := a.UpdateGUI(context.Background(), tc.repair, tc.check); err != nil {
				t.Fatal(err)
			}
			dialogs := readDialogLog(t, log)
			for _, text := range []string{
				"--title=Pokémon TCG Live — " + tc.title,
				"--text=" + tc.text,
				"--text=" + tc.result,
				"In-game patches are checked by the game at launch.",
			} {
				if !strings.Contains(dialogs, text) {
					t.Fatalf("missing %q in dialog output:\n%s", text, dialogs)
				}
			}
			if strings.Count(dialogs, "BEGIN --info") != 1 || strings.Index(dialogs, "END --progress") > strings.Index(dialogs, "BEGIN --info") {
				t.Fatalf("result was not shown once after progress closed:\n%s", dialogs)
			}
			installerRuns := 0
			for _, c := range r.commands {
				if c.Name == "umu-run" {
					installerRuns++
					if contains(c.Args, "REINSTALL=ALL") != (tc.repair && tc.sameInstaller) {
						t.Fatalf("wrong installer repair mode: %+v", c)
					}
				}
			}
			if installerRuns != tc.installerRuns {
				t.Fatalf("installer ran %d times, expected %d", installerRuns, tc.installerRuns)
			}
			if tc.installerRuns == 0 {
				after, _ := os.ReadFile(receipt)
				if !bytes.Equal(before, after) {
					t.Fatal("check or unchanged update advanced the installation receipt")
				}
			}
		})
	}
}

func TestGraphicalMaintenanceFailureHasNoSuccessDialog(t *testing.T) {
	for _, mode := range []string{"installer", "signature", "progress"} {
		t.Run(mode, func(t *testing.T) {
			a, r, _ := maintenanceApp(t)
			log := stubZenity(t)
			if err := writeJSON(filepath.Join(a.Paths.State, "installation.json"), Installation{InstallerSHA256: "previous"}); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "installer":
				r.run = func(Command) error { return errors.New("installer failed") }
			case "signature":
				r.output = func(Command) ([]byte, error) { return nil, errors.New("invalid signature") }
			case "progress":
				t.Setenv("TEST_PROGRESS_EXIT", "1")
			}
			if err := a.UpdateGUI(context.Background(), false, false); err == nil {
				t.Fatal("failure was not returned to the caller")
			}
			if dialogs := readDialogLog(t, log); strings.Contains(dialogs, "BEGIN --info") {
				t.Fatalf("failure showed a success dialog:\n%s", dialogs)
			}
			if mode != "progress" && a.installation().InstallerSHA256 != "previous" {
				t.Fatal("failed maintenance advanced the installation receipt")
			}
		})
	}
}

func TestGraphicalResultCanBeDismissed(t *testing.T) {
	a, _, _ := maintenanceApp(t)
	stubZenity(t)
	t.Setenv("TEST_INFO_EXIT", "1")
	if err := a.UpdateGUI(context.Background(), false, true); err != nil {
		t.Fatalf("dismissing the result was treated as maintenance failure: %v", err)
	}
}

func TestGraphicalMaintenanceReportsBusySession(t *testing.T) {
	for _, args := range [][]string{{"update", "--check", "--gui"}, {"update", "--gui"}, {"repair", "--gui"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			a, _ := testApp(t)
			log := stubZenity(t)
			useTestPaths(t, a.Paths)
			lock, err := acquireLock(a.Paths)
			if err != nil {
				t.Fatal(err)
			}
			defer releaseLock(lock)
			marker := filepath.Join(a.Paths.State, "play-session.json")
			writeTestFile(t, marker, []byte("active session"))
			var out, errOut bytes.Buffer
			if code := RunCLI(context.Background(), args, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), ErrBusy.Error()) {
				t.Fatalf("busy-session error not reported: code=%d, stderr=%s", code, &errOut)
			}
			dialogs := readDialogLog(t, log)
			if !strings.Contains(dialogs, "BEGIN --error") || !strings.Contains(dialogs, ErrBusy.Error()) || strings.Contains(dialogs, "BEGIN --info") {
				t.Fatalf("busy session did not produce only an error result:\n%s", dialogs)
			}
			b, _ := os.ReadFile(marker)
			if string(b) != "active session" {
				t.Fatal("blocked maintenance changed the active session")
			}
		})
	}
}
