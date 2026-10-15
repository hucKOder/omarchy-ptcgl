package installer_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestReleaseAssets(t *testing.T) {
	script, err := os.ReadFile("../../scripts/release-assets.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, version, commit string
		wantError             bool
	}{
		{name: "versioned artifacts", version: releaseVersion, commit: releaseCommit},
		{name: "prerelease", version: "v0.2.0-rc.1", commit: releaseCommit},
		{name: "invalid tag", version: "../main", commit: releaseCommit, wantError: true},
		{name: "short commit", version: releaseVersion, commit: "0123456", wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), "release tree with spaces")
			fixture(t, filepath.Join(root, "scripts/release-assets.sh"), string(script), 0600)
			fixture(t, filepath.Join(root, "install.sh"), "reviewed installer\n", 0600)
			fixture(t, filepath.Join(root, "build/ptcgl-omarchy"), "built launcher\n", 0755)
			cmd := exec.Command("bash", "scripts/release-assets.sh", tt.version, tt.commit)
			cmd.Dir = root
			output, err := cmd.CombinedOutput()
			if (err != nil) != tt.wantError {
				t.Fatalf("release assets error = %v, wantError = %v\n%s", err, tt.wantError, output)
			}
			if tt.wantError {
				if _, err := os.Stat(filepath.Join(root, "build/release")); !os.IsNotExist(err) {
					t.Fatalf("invalid release created output: %v", err)
				}
				return
			}
			b, err := os.ReadFile(filepath.Join(root, "build/release/release.json"))
			if err != nil {
				t.Fatal(err)
			}
			var manifest map[string]any
			if err := json.Unmarshal(b, &manifest); err != nil {
				t.Fatal(err)
			}
			for key, want := range map[string]any{
				"schema": float64(1), "repository": "hucKOder/omarchy-ptcgl", "release": tt.version,
				"source_commit": tt.commit, "platform": "linux-amd64", "launcher": "ptcgl-omarchy-linux-amd64",
			} {
				if manifest[key] != want {
					t.Errorf("manifest %s = %v, want %v", key, manifest[key], want)
				}
			}
			for name, key := range map[string]string{"install.sh": "installer_sha256", "ptcgl-omarchy-linux-amd64": "launcher_sha256"} {
				b, err := os.ReadFile(filepath.Join(root, "build/release", name))
				if err != nil {
					t.Fatal(err)
				}
				if want := fmt.Sprintf("%x", sha256.Sum256(b)); manifest[key] != want {
					t.Errorf("%s digest = %v, want %s", name, manifest[key], want)
				}
			}
		})
	}
}
