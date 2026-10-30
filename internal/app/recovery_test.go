package app

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadProgressKnownUnknownAndCancellation(t *testing.T) {
	for _, mode := range []string{"known", "unknown", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			body := bytes.Repeat([]byte("x"), 10000)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if mode == "known" {
					w.Header().Set("Content-Length", "10000")
				} else {
					w.(http.Flusher).Flush()
				}
				_, _ = w.Write(body)
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			d := testDownloadClient(server)
			var events []ProgressEvent
			d.Progress = func(e ProgressEvent) {
				events = append(events, e)
				if mode == "cancel" && e.Downloaded > 0 {
					cancel()
				}
			}
			dest := filepath.Join(t.TempDir(), "cached")
			writeTestFile(t, dest, []byte("old verified cache"))
			verified := false
			err := d.Download(ctx, server.URL, dest, 20000, func(string) error { verified = true; return nil })
			if mode == "cancel" {
				if err == nil || verified {
					t.Fatal("cancelled download was verified/published")
				}
				b, _ := os.ReadFile(dest)
				if string(b) != "old verified cache" {
					t.Fatal("cancelled download replaced verified cache")
				}
			} else {
				if err != nil || !verified {
					t.Fatalf("download failed: %v", err)
				}
				last := events[len(events)-1]
				if last.Phase != "verifying" || last.Cancellable {
					t.Fatalf("verification was cancellable: %+v", last)
				}
				transfer := events[len(events)-2]
				if transfer.Downloaded != 10000 || (mode == "known" && transfer.Total != 10000) || (mode == "unknown" && transfer.Total != -1) || transfer.BytesPerSecond <= 0 {
					t.Fatalf("incorrect transfer: %+v", transfer)
				}
			}
			leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(dest), ".download-*"))
			if len(leftovers) != 0 {
				t.Fatal("temporary transfer retained")
			}
		})
	}
}

func TestManagedRuntimeRecoveryRestagesOnlyAfterValidation(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(map[bool]string{true: "valid replacement", false: "invalid replacement"}[valid], func(t *testing.T) {
			a, _ := testApp(t)
			a.Profile.ProtonDirectory = "GE"
			var headers []*tar.Header
			if valid {
				for _, name := range []string{"proton", "toolmanifest.vdf", "compatibilitytool.vdf", "version"} {
					headers = append(headers, &tar.Header{Name: "GE/" + name, Typeflag: tar.TypeReg, Mode: 0o755, Size: 1})
				}
				headers = append(headers, &tar.Header{Name: "GE/files", Typeflag: tar.TypeDir, Mode: 0o755})
			} else {
				headers = append(headers, &tar.Header{Name: "GE/proton", Typeflag: tar.TypeReg, Size: 1})
			}
			archive := tarFixture(t, headers)
			hash, err := fileSHA256(archive)
			if err != nil {
				t.Fatal(err)
			}
			a.Profile.ProtonSHA256 = hash
			b, _ := os.ReadFile(archive)
			writeTestFile(t, filepath.Join(a.Paths.Cache, "GE.tar.gz"), b)
			old := filepath.Join(a.protonPath(), "proton")
			writeTestFile(t, old, []byte("incomplete original"))
			err = a.ensureProton(context.Background())
			if valid {
				if err != nil || validateProton(a.protonPath()) != nil {
					t.Fatalf("runtime did not recover: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("invalid replacement published")
				}
				b, _ := os.ReadFile(old)
				if string(b) != "incomplete original" {
					t.Fatal("failed extraction changed original runtime")
				}
			}
			leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(a.protonPath()), ".proton-*"))
			if len(leftovers) != 0 {
				t.Fatal("runtime staging retained")
			}
		})
	}
}

func TestCustomRuntimeIsNeverRestaged(t *testing.T) {
	a, _ := testApp(t)
	a.Config.ProtonPath = filepath.Join(a.Paths.Home, "custom")
	path := filepath.Join(a.Config.ProtonPath, "proton")
	writeTestFile(t, path, []byte("custom runtime"))
	if err := a.ensureProton(context.Background()); err == nil {
		t.Fatal("incomplete custom runtime accepted")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "custom runtime" {
		t.Fatal("custom runtime was changed")
	}
}

func TestExecutableWithoutSuccessfulReceiptIsNotReady(t *testing.T) {
	a, r, _ := maintenanceApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	if err := os.Remove(filepath.Join(a.Paths.State, "installation.json")); err != nil {
		t.Fatal(err)
	}
	if a.gameReady() {
		t.Fatal("executable without receipt accepted")
	}
	if s := a.Status(context.Background()); s.Readiness != "repair-needed" || s.Capabilities.Play {
		t.Fatalf("missing receipt marked ready: %+v", s)
	}
	if err := a.Setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !a.gameReady() || a.installation().Operation != "repair" {
		t.Fatal("setup failed to repair an unrecorded installation")
	}
	found := false
	for _, c := range r.commands {
		if c.Name == "umu-run" && contains(c.Args, "REINSTALL=ALL") {
			found = true
		}
	}
	if !found {
		t.Fatal("unrecorded files were not reapplied")
	}
}

func TestInterruptedMSIWithExecutableRequiresAndCompletesRepair(t *testing.T) {
	a, r, _ := maintenanceApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	registry := filepath.Join(a.Paths.Prefix, "user.reg")
	writeTestFile(t, registry, []byte("saved account and settings"))
	old := Installation{InstallerSHA256: "previous successful receipt"}
	if err := writeJSON(filepath.Join(a.Paths.State, "installation.json"), old); err != nil {
		t.Fatal(err)
	}
	r.run = func(Command) error { return errors.New("Windows Installer interrupted") }
	if err := a.Update(context.Background(), false, false); err == nil || !strings.Contains(err.Error(), "ptcgl-omarchy repair") {
		t.Fatalf("missing recovery action: %v", err)
	}
	if !exists(a.gamePath()) || a.gameReady() || !a.installPending() {
		t.Fatal("partial executable mistaken for completed setup")
	}
	if a.installation().InstallerSHA256 != old.InstallerSHA256 {
		t.Fatal("failed MSI advanced successful receipt")
	}
	if s := a.Status(context.Background()); s.Readiness != "repair-needed" || s.Capabilities.Play {
		t.Fatalf("interrupted MSI marked ready: %+v", s)
	}
	found := false
	for _, check := range a.Doctor(context.Background()) {
		if check.Name == "game" && !check.OK && check.Action == "repair" {
			found = true
		}
	}
	if !found {
		t.Fatal("diagnostics did not recommend repair")
	}
	r.run = nil
	if err := a.Setup(context.Background()); err != nil {
		t.Fatalf("retry did not repair: %v", err)
	}
	if !a.gameReady() || a.installPending() || a.installation().Operation != "repair" {
		t.Fatal("repair did not complete installation state")
	}
	b, _ := os.ReadFile(registry)
	if string(b) != "saved account and settings" {
		t.Fatal("recovery reset account/settings")
	}
}

func TestDownloadCancellationDoesNotReachVerificationOrInstaller(t *testing.T) {
	a, r, _ := maintenanceApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	a.Progress = func(e ProgressEvent) {
		if e.Cancellable {
			a.cancelCurrentDownload()
		}
	}
	if err := a.Update(context.Background(), false, false); err == nil {
		t.Fatal("cancellation was ignored")
	}
	if a.installPending() {
		t.Fatal("cancelled transfer marked MSI as started")
	}
	for _, c := range r.commands {
		if c.Name == "umu-run" || strings.Contains(c.Name, "__verify") {
			t.Fatal("installer started after cancellation")
		}
	}
	if a.cancelCurrentDownload() {
		t.Fatal("cancel callback retained after transfer")
	}
}

func TestVerificationAndInstallerAreNotUserCancellable(t *testing.T) {
	a, r, _ := maintenanceApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	a.Progress = func(e ProgressEvent) {
		if e.Phase == "verifying" || e.Phase == "installing" {
			if a.cancelCurrentDownload() {
				t.Fatal("unsafe phase accepted cancellation")
			}
		}
	}
	r.run = func(Command) error {
		if a.cancelCurrentDownload() {
			t.Fatal("Windows Installer accepted download cancellation")
		}
		return nil
	}
	if err := a.Update(context.Background(), false, false); err != nil {
		t.Fatal(err)
	}
}
