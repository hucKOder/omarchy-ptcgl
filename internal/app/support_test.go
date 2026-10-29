package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSupportReportUsesAnAllowlistInsteadOfRawDiagnostics(t *testing.T) {
	a, _ := testApp(t)
	checks := []Check{
		{Name: "game", OK: false, Detail: a.Paths.Home + " tpcitcgapp://callback?code=SECRET", Fix: "echo SECRET", Action: "repair"},
		{Name: "browser callback", OK: false, Detail: "SECRET", Action: "arbitrary SECRET command"},
		{Name: "unrecognized SECRET check", OK: false},
	}
	report := a.SupportReport(checks)
	for _, forbidden := range []string{"SECRET", a.Paths.Home, "tpcitcgapp://", "echo ", "unrecognized"} {
		if strings.Contains(report, forbidden) {
			t.Fatalf("shared report leaked %q: %s", forbidden, report)
		}
	}
	for _, required := range []string{"Launcher:", "Runtime:", "FAIL  game", "Reapply the verified official game installer", "Game patches:"} {
		if !strings.Contains(report, required) {
			t.Fatalf("missing report context %q", required)
		}
	}
}

func TestFixRejectsUnlistedCommandsAndCancelledConfirmation(t *testing.T) {
	a, r := testApp(t)
	log := stubZenity(t)
	t.Setenv("TEST_QUESTION_EXIT", "2")
	if err := a.Fix(context.Background(), "integration; echo SECRET", true); err == nil {
		t.Fatal("arbitrary fix accepted")
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("unlisted fix reached UI")
	}
	// Confirmation failures must not authorize a mutation.
	if err := a.Fix(context.Background(), "integration", true); err == nil {
		t.Fatal("failed confirmation was ignored")
	}
	if len(r.commands) != 0 {
		t.Fatal("unconfirmed recovery modified integration")
	}
	t.Setenv("TEST_QUESTION_EXIT", "1")
	if err := a.Fix(context.Background(), "integration", true); err != nil {
		t.Fatalf("cancelled confirmation was treated as failure: %v", err)
	}
	if len(r.commands) != 0 {
		t.Fatal("cancelled recovery changed integration")
	}
}

func TestDiagnosticsSaveAndCancelUseOnlyTheShareableReport(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "save", true: "cancel"}[cancel], func(t *testing.T) {
			a, r := testApp(t)
			stubZenity(t)
			t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
			t.Setenv("TEST_TEXT_ACTION", "Save report")
			path := filepath.Join(t.TempDir(), "support report.txt")
			t.Setenv("TEST_REPORT_FILE", path)
			if cancel {
				t.Setenv("TEST_FILE_EXIT", "1")
			}
			if err := a.DiagnosticsGUI(context.Background()); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(path)
			if cancel {
				if !os.IsNotExist(err) {
					t.Fatal("cancelled save wrote a report")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), a.Paths.Home) || !strings.Contains(string(b), "support report") {
				t.Fatalf("unsafe report saved: %s", b)
			}
			for _, c := range r.commands {
				if c.Name == "umu-run" {
					t.Fatal("saving diagnostics started maintenance")
				}
			}
		})
	}
}

func TestDiagnosticRecoveryConfirmsAndRefreshesChecks(t *testing.T) {
	a, r := testApp(t)
	log := stubZenity(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	t.Setenv("TEST_TEXT_ACTION", "Fix integration")
	writeTestFile(t, a.Paths.Bin, []byte("launcher fixture"))
	writeTestFile(t, filepath.Join(a.Paths.Hypr, "hyprland.lua"), []byte("-- existing config\n"))
	r.output = func(Command) ([]byte, error) { return []byte("previous.desktop"), nil }
	if err := a.DiagnosticsGUI(context.Background()); err != nil {
		t.Fatal(err)
	}
	dialogs := readDialogLog(t, log)
	if !strings.Contains(dialogs, "BEGIN --question") || strings.Count(dialogs, "BEGIN --text-info") != 2 {
		t.Fatalf("recovery did not confirm and refresh: %s", dialogs)
	}
	if !exists(filepath.Join(a.Paths.Applications, desktopID)) {
		t.Fatal("confirmed integration fix did not run")
	}
}

func TestUpdateOverviewDoesNotClaimOnlineFreshness(t *testing.T) {
	a, _ := testApp(t)
	before := a.UpdatesSummary()
	if !strings.Contains(before, "online freshness not checked") || !strings.Contains(before, "Not checked.") || !strings.Contains(before, "Game patches: checked in-game") {
		t.Fatalf("misleading overview: %s", before)
	}
	now := time.Now().UTC()
	if err := writeJSON(filepath.Join(a.Paths.State, "installation.json"), Installation{InstallerSHA256: strings.Repeat("a", 64), AppliedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(a.Paths.State, "installer-check.json"), InstallerCheck{SHA256: strings.Repeat("a", 64), CheckedAt: now}); err != nil {
		t.Fatal(err)
	}
	after := a.UpdatesSummary()
	if !strings.Contains(after, "Installer matched at last check:") || !strings.Contains(after, now.Format(time.RFC3339)) {
		t.Fatalf("check context omitted: %s", after)
	}
}

func TestUpdateOverviewCheckChangesOnlyTheCheckReceipt(t *testing.T) {
	a, r, _ := maintenanceApp(t)
	stubZenity(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	t.Setenv("TEST_TEXT_ACTION", "Check installer")
	receipt := filepath.Join(a.Paths.State, "installation.json")
	before, _ := os.ReadFile(receipt)
	if err := a.UpdatesGUI(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(receipt)
	if !bytes.Equal(before, after) {
		t.Fatal("installer check changed successful installation receipt")
	}
	if !exists(filepath.Join(a.Paths.State, "installer-check.json")) {
		t.Fatal("explicit check was not recorded")
	}
	if exists(filepath.Join(a.Paths.State, "runtime.json")) {
		t.Fatal("installer check selected a runtime")
	}
	for _, c := range r.commands {
		if c.Name == "umu-run" {
			t.Fatal("installer check invoked Windows Installer")
		}
	}
}

func TestUpdatesDisableMutationsForBusyOrCustomRuntime(t *testing.T) {
	a, r := testApp(t)
	log := stubZenity(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	t.Setenv("TEST_TEXT_ACTION", "Update installer")
	if err := a.withOperation("repair", func() error {
		if err := a.UpdatesGUI(context.Background()); err == nil {
			t.Fatal("busy update view accepted a modifying action")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(r.commands) != 0 {
		t.Fatal("busy update view launched a subprocess")
	}
	if strings.Contains(readDialogLog(t, log), "--extra-button=Update installer") {
		t.Fatal("busy view offered update action")
	}
	t.Setenv("TEST_TEXT_ACTION", "")
	a.Config.ProtonPath = filepath.Join(a.Paths.Home, "custom")
	if err := a.UpdatesGUI(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(readDialogLog(t, log), "--extra-button=Select bundled runtime") {
		t.Fatal("custom runtime offered automatic replacement")
	}
}

func TestCopyReportExcludesLocalDiagnosticDetails(t *testing.T) {
	a, _ := testApp(t)
	stubZenity(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	t.Setenv("TEST_TEXT_ACTION", "Copy report")
	path := filepath.Join(t.TempDir(), "clipboard.txt")
	copyPath := filepath.Join(os.Getenv("PATH"), "wl-copy")
	script := "#!/bin/sh\nwhile IFS= read -r line; do printf '%s\\n' \"$line\"; done > '" + path + "'\n"
	if err := os.WriteFile(copyPath, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := a.DiagnosticsGUI(context.Background()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "support report") || strings.Contains(string(b), a.Paths.Home) || strings.Contains(string(b), "Local diagnostic details") {
		t.Fatalf("clipboard received local/private details: %s", b)
	}
}

func TestCLISavesPrivateSupportReportEvenWhenChecksFail(t *testing.T) {
	a, _ := testApp(t)
	useTestPaths(t, a.Paths)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	t.Setenv("PATH", t.TempDir())
	path := filepath.Join(t.TempDir(), "a report with spaces.txt")
	var out, errOut bytes.Buffer
	if code := RunCLI(context.Background(), []string{"doctor", "--save", path}, &out, &errOut); code != 1 {
		t.Fatalf("failed checks should retain exit 1, got %d: %s", code, &errOut)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "support report") || strings.Contains(string(b), a.Paths.Home) {
		t.Fatalf("unsafe or empty export: %s", b)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("report is not private")
	}
}
