package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func doctorApp(t *testing.T) (*App, *fakeRunner) {
	t.Helper()
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test-session")
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	for name, content := range a.desktopFiles() {
		writeTestFile(t, filepath.Join(a.Paths.Applications, name), content)
	}
	writeTestFile(t, filepath.Join(a.Paths.Hypr, "hyprland.lua"), []byte(managedBlock(filepath.Join(a.Paths.Hypr, Name+".lua"))))
	rules, _ := assets.ReadFile("assets/ptcgl-omarchy.lua")
	writeTestFile(t, filepath.Join(a.Paths.Hypr, Name+".lua"), rules)
	r.output = func(c Command) ([]byte, error) {
		if c.Name == "xdg-mime" {
			return []byte(callbackID + "\n"), nil
		}
		return nil, nil
	}
	return a, r
}

func TestDoctorHealthyInstallation(t *testing.T) {
	a, r := doctorApp(t)
	checks := a.Doctor(context.Background())
	for _, check := range checks {
		if !check.OK || check.Fix != "" || check.Detail == "" {
			t.Fatalf("unexpected diagnostic: %+v", check)
		}
	}
	if strings.Contains(FormatChecks(checks), "Fix:") {
		t.Fatal("healthy report suggests repairs")
	}
	for _, command := range r.commands {
		if (command.Name != "xdg-mime" || strings.Join(command.Args, " ") != "query default "+callbackMIME) &&
			(command.Name != "hyprctl" || strings.Join(command.Args, " ") != "configerrors") {
			t.Fatalf("diagnostics invoked a modifying command: %+v", command)
		}
	}
}

func TestDoctorExplainsFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, *App, *fakeRunner)
		check  string
		detail string
		fix    string
	}{
		{
			name: "missing UMU",
			change: func(_ *testing.T, a *App, _ *fakeRunner) {
				a.LookPath = func(name string) (string, error) {
					if name == "umu-run" {
						return "", errors.New("umu-run not found in PATH")
					}
					return "/usr/bin/" + name, nil
				}
			},
			check: "umu-run", detail: "not found in PATH", fix: "omarchy pkg add umu-launcher",
		},
		{
			name: "missing managed runtime",
			change: func(_ *testing.T, a *App, _ *fakeRunner) {
				a.Profile.ProtonDirectory = "missing-runtime"
			},
			check: "Proton", detail: "missing proton", fix: "ptcgl-omarchy setup",
		},
		{
			name: "incomplete managed runtime",
			change: func(t *testing.T, a *App, _ *fakeRunner) {
				if err := os.Remove(filepath.Join(a.protonPath(), "version")); err != nil {
					t.Fatal(err)
				}
			},
			check: "Proton", detail: "missing version", fix: "Move the incomplete runtime",
		},
		{
			name: "missing custom runtime",
			change: func(_ *testing.T, a *App, _ *fakeRunner) {
				a.Config.ProtonPath = filepath.Join(a.Paths.Home, "custom runtime")
			},
			check: "Proton", detail: "custom runtime", fix: "Correct proton_path",
		},
		{
			name: "missing callback desktop file",
			change: func(t *testing.T, a *App, _ *fakeRunner) {
				if err := os.Remove(filepath.Join(a.Paths.Applications, callbackID)); err != nil {
					t.Fatal(err)
				}
			},
			check: "callback entry", detail: callbackID, fix: "ptcgl-omarchy integrate",
		},
		{
			name: "different callback handler",
			change: func(_ *testing.T, _ *App, r *fakeRunner) {
				r.output = func(Command) ([]byte, error) { return []byte("other.desktop\n"), nil }
			},
			check: "browser callback", detail: "Default handler is other.desktop; expected " + callbackID, fix: "ptcgl-omarchy integrate",
		},
		{
			name: "no callback handler",
			change: func(_ *testing.T, _ *App, r *fakeRunner) {
				r.output = func(Command) ([]byte, error) { return nil, nil }
			},
			check: "browser callback", detail: "No default handler", fix: "ptcgl-omarchy integrate",
		},
		{
			name: "callback query error",
			change: func(_ *testing.T, _ *App, r *fakeRunner) {
				r.output = func(Command) ([]byte, error) { return []byte("query diagnostic"), errors.New("query failed") }
			},
			check: "browser callback", detail: "query failed; query diagnostic", fix: "ptcgl-omarchy integrate",
		},
		{
			name: "unreadable Hyprland config",
			change: func(t *testing.T, a *App, _ *fakeRunner) {
				path := filepath.Join(a.Paths.Hypr, "hyprland.lua")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			},
			check: "Hyprland rules", detail: "is a directory", fix: "ptcgl-omarchy integrate",
		},
		{
			name: "Hyprland command error without output",
			change: func(_ *testing.T, _ *App, r *fakeRunner) {
				r.output = func(Command) ([]byte, error) { return nil, errors.New("socket unavailable") }
			},
			check: "Hyprland config", detail: "socket unavailable", fix: "hyprctl configerrors",
		},
		{
			name: "Hyprland configuration error",
			change: func(_ *testing.T, _ *App, r *fakeRunner) {
				r.output = func(Command) ([]byte, error) { return []byte("invalid rule on line 12\n"), nil }
			},
			check: "Hyprland config", detail: "invalid rule on line 12", fix: "hyprctl reload",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, r := doctorApp(t)
			tc.change(t, a, r)
			for _, check := range a.Doctor(context.Background()) {
				if check.Name != tc.check {
					continue
				}
				if check.OK || !strings.Contains(check.Detail, tc.detail) || !strings.Contains(check.Fix, tc.fix) {
					t.Fatalf("unexpected diagnostic: %+v", check)
				}
				report := FormatChecks([]Check{check})
				if !strings.HasPrefix(report, "FAIL") || !strings.Contains(report, "Fix: "+check.Fix) {
					t.Fatalf("failure guidance missing from report: %s", report)
				}
				return
			}
			t.Fatalf("missing check %q", tc.check)
		})
	}
}

func TestDoctorWithoutDesktopSession(t *testing.T) {
	a, r := doctorApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	checks := a.Doctor(context.Background())
	for _, check := range checks {
		if check.Name == "Hyprland session" && (check.OK || !strings.Contains(check.Fix, "inside your Omarchy desktop session")) {
			t.Fatalf("missing session guidance: %+v", check)
		}
	}
	for _, command := range r.commands {
		if command.Name == "hyprctl" {
			t.Fatal("queried Hyprland without a session")
		}
	}

	b, err := json.Marshal(checks)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, check := range decoded {
		_, hasFix := check["fix"]
		wantFix := !check["ok"].(bool)
		if hasFix != wantFix || check["name"] == nil || check["detail"] == nil {
			t.Fatalf("JSON report lost diagnostic fields or included an irrelevant fix: %s", b)
		}
	}
}
