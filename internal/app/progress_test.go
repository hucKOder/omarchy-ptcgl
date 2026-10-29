package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func waitForDialogText(t *testing.T, path, text string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b, _ := os.ReadFile(path)
		if strings.Contains(string(b), text) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Errorf("progress dialog never displayed %q", text)
}

func TestGraphicalProgressUsesRealPercentageAndSafePhases(t *testing.T) {
	a, _ := testApp(t)
	log := stubZenity(t)
	err := a.WithProgress(context.Background(), func(context.Context) error {
		a.emit(ProgressEvent{Phase: "downloading", Message: "Downloading", Downloaded: 1 << 20, Total: 2 << 20, BytesPerSecond: 1 << 20, Cancellable: true})
		waitForDialogText(t, log, "progress: 50")
		a.phase("verifying", "Verifying downloaded files…")
		waitForDialogText(t, log, "progress: # Verifying downloaded files…")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	dialogs := readDialogLog(t, log)
	if !strings.Contains(dialogs, "1.0 MiB / 2.0 MiB") || !strings.Contains(dialogs, "MiB/s") {
		t.Fatalf("transfer details missing: %s", dialogs)
	}
	if strings.Count(dialogs, "--no-cancel") < 2 || strings.Count(dialogs, "--pulsate") < 2 {
		t.Fatalf("verification was not indeterminate and protected: %s", dialogs)
	}
}

func TestGraphicalDownloadCancellationPreservesCache(t *testing.T) {
	a, r := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	stubZenity(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	t.Setenv("TEST_CANCEL_DOWNLOAD", "1")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer server.Close()
	a.Profile.InstallerURL = server.URL
	a.Download = testDownloadClient(server)
	cache := filepath.Join(a.Paths.Cache, "PokemonTCGLiveInstaller.msi")
	writeTestFile(t, cache, []byte("previous verified cache"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.UpdateGUI(ctx, false, false); err == nil {
		t.Fatal("GUI cancellation was ignored")
	}
	b, _ := os.ReadFile(cache)
	if string(b) != "previous verified cache" {
		t.Fatal("GUI cancellation destroyed cache")
	}
	for _, c := range r.commands {
		if c.Name == "umu-run" {
			t.Fatal("GUI cancellation reached Windows Installer")
		}
	}
}

func TestClosingStartupFeedbackDoesNotCancelTheGame(t *testing.T) {
	a, _ := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	bin := t.TempDir()
	finished := filepath.Join(bin, "game-finished")
	writeTestFile(t, filepath.Join(bin, "zenity"), []byte("#!/bin/sh\nexit 1\n"))
	writeTestFile(t, filepath.Join(bin, "umu-run"), []byte("#!/bin/sh\n/usr/bin/sleep 1\nprintf done > '"+finished+"'\n"))
	for _, name := range []string{"zenity", "umu-run"} {
		if err := os.Chmod(filepath.Join(bin, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	a.Runner = OSRunner{Out: io.Discard, Err: io.Discard}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := a.play(ctx, nil, true); err != nil {
		t.Fatalf("closing feedback killed the game: %v", err)
	}
	if b, _ := os.ReadFile(finished); string(b) != "done" {
		t.Fatal("game process did not complete")
	}
	if exists(filepath.Join(a.Paths.State, "play-session.json")) {
		t.Fatal("startup/session cleanup incomplete")
	}
}
