package installer_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func releaseTree(t *testing.T) string {
	t.Helper()
	script, err := os.ReadFile("../../scripts/release-assets.sh")
	if err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile("../../scripts/release-notes.md.in")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "release tree with spaces")
	fixture(t, filepath.Join(root, "scripts/release-assets.sh"), string(script), 0o600)
	fixture(t, filepath.Join(root, "scripts/release-notes.md.in"), string(template), 0o600)
	fixture(t, filepath.Join(root, "install.sh"), "reviewed installer\n", 0o600)
	fixture(t, filepath.Join(root, "build/ptcgl-omarchy"), "built launcher\n", 0o755)
	return root
}

func generatedNotes(t *testing.T, version, commit string) string {
	t.Helper()
	root := releaseTree(t)
	cmd := exec.Command("bash", "scripts/release-assets.sh", version, commit)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate release notes: %v\n%s", err, output)
	}
	b, err := os.ReadFile(filepath.Join(root, "build/release/notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func installBlock(t *testing.T, notes string) string {
	t.Helper()
	if _, manual, ok := strings.Cut(notes, "### Manual, independently pinned installation\n"); ok {
		notes = manual
	}
	_, block, ok := strings.Cut(notes, "```bash\n")
	if !ok {
		t.Fatal("installation block is missing")
	}
	block, _, ok = strings.Cut(block, "\n```")
	if !ok {
		t.Fatal("installation block is incomplete")
	}
	return block
}

func TestReleaseAssets(t *testing.T) {
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
			root := releaseTree(t)
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
			notes, err := os.ReadFile(filepath.Join(root, "build/release/notes.md"))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"Release: **" + tt.version + "**", "Source commit: `" + tt.commit + "`", "release='" + tt.version + "'", "source_commit='" + tt.commit + "'"} {
				if !strings.Contains(string(notes), want) {
					t.Errorf("release instructions missing %q", want)
				}
			}
			if strings.Contains(string(notes), "@RELEASE@") || strings.Contains(string(notes), "@SOURCE_COMMIT@") {
				t.Fatal("release instructions contain unexpanded placeholders")
			}
			check := exec.Command("bash", "-n")
			check.Stdin = strings.NewReader(installBlock(t, string(notes)))
			if output, err := check.CombinedOutput(); err != nil {
				t.Fatalf("invalid generated bootstrap: %v\n%s", err, output)
			}
		})
	}
}

func TestLegacyPublishedBootstrapRetainsRequiredVerifier(t *testing.T) {
	const publishedCommit = "5e3e96bff7a5bcff0b093dc64dc12538d813148e"
	doc, err := os.ReadFile("../../docs/RELEASES.md")
	if err != nil {
		t.Fatal(err)
	}
	legacy := installBlock(t, string(doc))
	check := `  command -v osslsigncode >/dev/null || fail "osslsigncode is required. See https://github.com/$repository/blob/$source_commit/docs/AUR-DEPENDENCIES.md"` + "\n"
	if !strings.Contains(legacy, check) {
		t.Fatal("historical v0.2.0 instructions lost their required external verifier")
	}
	if strings.Replace(legacy, check, "", 1) != installBlock(t, generatedNotes(t, "v0.2.0", publishedCommit)) {
		t.Fatal("published-release fallback differs from the generated installation procedure")
	}
}

func TestReleasePublicationClassifiesPrereleases(t *testing.T) {
	workflow, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, step, ok := strings.Cut(string(workflow), "      - name: Publish authenticated release\n")
	if !ok {
		t.Fatal("release publication step is missing")
	}
	_, script, ok := strings.Cut(step, "        run: |\n")
	if !ok {
		t.Fatal("release publication script is missing")
	}
	lines := strings.Split(script, "\n")
	for i := range lines {
		lines[i] = strings.TrimPrefix(lines[i], "          ")
	}
	script = strings.Join(lines, "\n")
	for _, version := range []string{"v1.2.3", "v1.2.3-rc.1", "v1.2.3-beta.2"} {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			bin := filepath.Join(root, "bin")
			fixture(t, filepath.Join(bin, "gh"), `#!/bin/bash
set -euo pipefail
if [[ $1 == attestation && $2 == verify ]]; then
  [[ -f $3 ]]
  exit 0
fi
[[ $1 == release && $2 == create ]]
printf '%s\n' "$@" > "$TEST_RELEASE_ARGS"
`, 0o755)
			for _, name := range []string{"bash", "cp"} {
				path, err := exec.LookPath(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path, filepath.Join(bin, name)); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{"install.sh", "release.json", "ptcgl-omarchy-linux-amd64"} {
				fixture(t, filepath.Join(root, "release", name), "artifact\n", 0o600)
			}
			fixture(t, filepath.Join(root, "bundle.json"), "attestation bundle\n", 0o600)
			fixture(t, filepath.Join(root, "release/notes.md"), "Generated release instructions\n", 0o600)
			argsPath := filepath.Join(root, "release-args")
			cmd := exec.Command("bash", "-e", "-o", "pipefail")
			cmd.Dir = root
			cmd.Stdin = strings.NewReader(script)
			cmd.Env = append(os.Environ(), "PATH="+bin, "GITHUB_REF_NAME="+version,
				"GITHUB_SHA="+releaseCommit, "GITHUB_REPOSITORY=hucKOder/omarchy-ptcgl",
				"BUNDLE_PATH="+filepath.Join(root, "bundle.json"), "TEST_RELEASE_ARGS="+argsPath)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("release publication failed: %v\n%s", err, output)
			}
			args, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatal(err)
			}
			prerelease := strings.Contains(version, "-")
			if strings.Contains(string(args), "--prerelease\n") != prerelease || strings.Contains(string(args), "--latest=false\n") != prerelease {
				t.Fatalf("incorrect release classification:\n%s", args)
			}
			notes, err := os.ReadFile(filepath.Join(root, "release/notes.md"))
			if err != nil || string(notes) != "Generated release instructions\n" {
				t.Fatalf("publisher changed the generated notes: %q (%v)", notes, err)
			}
		})
	}
}
