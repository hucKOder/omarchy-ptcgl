package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ptcgl-omarchy/internal/verify"
)

func TestVerificationWorkerNeedsNoLauncherConfiguration(t *testing.T) {
	for _, args := range [][]string{{"__verify-installer"}, {"__verify-installer", "missing.msi", "extra"}, {"__verify-installer", "missing.msi"}} {
		var out bytes.Buffer
		if code := RunCLI(context.Background(), args, io.Discard, &out); code == 0 {
			t.Fatalf("worker accepted invalid invocation: %v", args)
		}
	}
	path := filepath.Join(t.TempDir(), "malformed.msi")
	writeTestFile(t, path, []byte("not an MSI"))
	if err := verify.MSI(context.Background(), path); err == nil {
		t.Fatal("accepted malformed MSI")
	}
}

func TestMSIVerificationUsesBundledWorker(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "worker-test-placeholder")
	a, r := testApp(t)
	path := filepath.Join(t.TempDir(), "installer.msi")
	writeTestFile(t, path, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
	a.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	if err := a.verifyInstaller(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	self, _ := os.Executable()
	if len(r.commands) != 1 || r.commands[0].Name != self || strings.Join(r.commands[0].Args, "|") != "__verify-installer|"+path {
		t.Fatalf("not using the bundled verifier: %+v", r.commands)
	}
	if r.commands[0].Env == nil || contains(r.commands[0].Env, "GITHUB_TOKEN=worker-test-placeholder") {
		t.Fatal("MSI worker inherited developer credentials")
	}
}
