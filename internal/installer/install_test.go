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

const releaseVersion = "v0.4.0"
const releaseCommit = "0123456789abcdef0123456789abcdef01234567"

const launcherFixture = `#!/bin/bash
set -e
printf 'launcher %s\n' "$*" >> "$TEST_LOG"
case "$1" in
  install)
    mkdir -p "$HOME/.local/bin"
    cp "$0" "$HOME/.local/bin/ptcgl-omarchy"
    ;;
  setup) [[ ${TEST_SETUP_FAIL:-0} != 1 ]] ;;
esac
`

func fixture(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func releaseFixtures(t *testing.T, root, mode, script string) {
	t.Helper()
	launcher := launcherFixture
	manifest := map[string]any{
		"schema": 1, "repository": "hucKOder/omarchy-ptcgl", "release": releaseVersion,
		"source_commit": releaseCommit, "platform": "linux-amd64", "launcher": "ptcgl-omarchy-linux-amd64",
		"launcher_sha256":  fmt.Sprintf("%x", sha256.Sum256([]byte(launcher))),
		"installer_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(script))),
	}
	switch mode {
	case "manifest-version":
		manifest["release"] = "v9.9.9"
	case "manifest-commit":
		manifest["source_commit"] = strings.Repeat("f", 40)
	case "manifest-platform":
		manifest["platform"] = "linux-arm64"
	case "manifest-schema":
		manifest["schema"] = 2
	case "manifest-path":
		manifest["launcher"] = "../../other-binary"
	case "manifest-hash":
		manifest["launcher_sha256"] = "not-a-sha256"
	}
	marshal := func(value any) string {
		t.Helper()
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	manifestData := marshal(manifest)
	if mode == "manifest-malformed" {
		manifestData = "{invalid JSON"
	}
	// This fixture models the verifier's authenticated identity and digest output.
	// Production signature verification is delegated to the real GitHub CLI.
	bundle := map[string]any{
		"signature_valid": true, "repository": "hucKOder/omarchy-ptcgl",
		"identity":   "https://github.com/hucKOder/omarchy-ptcgl/.github/workflows/release.yml@refs/tags/" + releaseVersion,
		"source_ref": "refs/tags/" + releaseVersion, "source_commit": releaseCommit, "signer_commit": releaseCommit,
		"issuer": "https://token.actions.githubusercontent.com", "runner": "github-hosted",
		"predicate": "https://slsa.dev/provenance/v1",
		"subjects": []string{
			fmt.Sprintf("%x", sha256.Sum256([]byte(manifestData))),
			fmt.Sprintf("%x", sha256.Sum256([]byte(script))),
		},
	}
	switch mode {
	case "signature":
		bundle["signature_valid"] = false
	case "wrong-repository":
		bundle["repository"] = "attacker/omarchy-ptcgl"
	case "wrong-workflow":
		bundle["identity"] = "https://github.com/hucKOder/omarchy-ptcgl/.github/workflows/ci.yml@refs/tags/" + releaseVersion
	case "wrong-ref":
		bundle["source_ref"] = "refs/heads/main"
	case "wrong-commit":
		bundle["source_commit"] = strings.Repeat("f", 40)
	case "wrong-signer":
		bundle["signer_commit"] = strings.Repeat("f", 40)
	case "wrong-issuer":
		bundle["issuer"] = "https://attacker.example"
	case "self-hosted":
		bundle["runner"] = "self-hosted"
	case "wrong-predicate":
		bundle["predicate"] = "https://example.com/not-provenance"
	case "checksum", "forged-pair":
		launcher += "\n# tampered after attestation\n"
		if mode == "forged-pair" {
			manifest["launcher_sha256"] = fmt.Sprintf("%x", sha256.Sum256([]byte(launcher)))
			manifestData = marshal(manifest)
		}
	}
	bundleData := marshal(bundle)
	if mode == "bundle-malformed" {
		bundleData = "invalid attestation bundle"
	}
	fixture(t, filepath.Join(root, "release.json"), manifestData, 0600)
	fixture(t, filepath.Join(root, "release.sigstore.json"), bundleData, 0600)
	fixture(t, filepath.Join(root, "release-launcher"), launcher, 0600)
}

func TestInstaller(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the installer requires a non-root desktop user")
	}
	script, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	quickScript, err := os.ReadFile("../../bootstrap.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, downloadMode   string
		args                 []string
		local, piped         bool
		bootstrap            bool
		quick, discover      bool
		noConfig, noVerifier bool
		noSelection, noGH    bool
		oldGH                bool
		pkgFail, setupFail   bool
		wantError            bool
		want, absent         []string
	}{
		{
			name: "one-command latest installation needs no AUR verifier", quick: true, discover: true, noVerifier: true,
			want: []string{"omarchy pkg add github-cli curl jq coreutils ca-certificates", "git/ref/tags/" + releaseVersion, "launcher install", "launcher setup"},
		},
		{
			name: "one-command pinned launcher upgrade", quick: true, noVerifier: true, args: []string{"--launcher-only"},
			want: []string{"launcher install"}, absent: []string{"api.github.com", "launcher setup"},
		},
		{
			name: "one-command follows annotated tag", quick: true, discover: true, downloadMode: "api-annotated",
			want: []string{"git/tags/" + releaseCommit, "launcher setup"},
		},
		{
			name: "one-command rejects old release", quick: true, discover: true, downloadMode: "api-old-release", wantError: true,
			absent: []string{"launcher install"},
		},
		{
			name: "one-command rejects non-commit tag", quick: true, discover: true, downloadMode: "api-invalid-ref", wantError: true,
		},
		{
			name: "one-command authenticates installer before execution", quick: true, downloadMode: "bootstrap-tampered", wantError: true,
			absent: []string{"unverified bootstrap ran", "launcher install"},
		},
		{
			name: "one-command rejects wrong source", quick: true, discover: true, downloadMode: "wrong-commit", wantError: true,
		},
		{
			name: "one-command rejects unavailable attestations", quick: true, downloadMode: "bundle-missing", wantError: true,
		},
		{
			name: "one-command checks GitHub CLI capabilities", quick: true, oldGH: true, wantError: true,
			absent: []string{"curl --"},
		},
		{
			name: "one-command package failure stops", quick: true, pkgFail: true, wantError: true,
			absent: []string{"curl --", "gh ", "launcher "},
		},
		{
			name: "release prepares game", piped: true,
			want:   []string{"omarchy pkg add umu-launcher", "omarchy install gaming gpu-lib32", "launcher install", "launcher setup"},
			absent: []string{"omarchy pkg add make go", "git clone", "make -C"},
		},
		{
			name: "generated bootstrap verifies installer before execution", bootstrap: true,
			want: []string{"gh attestation verify", "launcher install", "launcher setup"},
		},
		{
			name: "tampered bootstrap installer never executes", bootstrap: true, downloadMode: "bootstrap-tampered", wantError: true,
			absent: []string{"unverified bootstrap ran", "omarchy ", "launcher install"},
		},
		{
			name: "generated bootstrap launcher upgrade", bootstrap: true, args: []string{"--launcher-only"},
			want: []string{"launcher install"}, absent: []string{"launcher setup"},
		},
		{
			name: "generated bootstrap rejects wrong source commit", bootstrap: true, downloadMode: "wrong-commit", wantError: true,
		},
		{
			name: "generated bootstrap needs no external MSI verifier", bootstrap: true, noVerifier: true,
			want: []string{"launcher install", "launcher setup"},
		},
		{
			name: "generated bootstrap requires GitHub CLI before downloading", bootstrap: true, noGH: true, wantError: true,
			absent: []string{"curl "},
		},
		{
			name: "old GitHub CLI stops before downloading", oldGH: true, wantError: true,
			absent: []string{"curl "},
		},
		{
			name: "generated bootstrap checks GitHub CLI capabilities", bootstrap: true, oldGH: true, wantError: true,
			absent: []string{"curl "},
		},
		{
			name: "launcher only", args: []string{"--launcher-only"},
			want: []string{"launcher install"}, absent: []string{"launcher setup", "make -C"},
		},
		{
			name: "no release stops without building main", downloadMode: "notfound", wantError: true,
			absent: []string{"omarchy ", "git clone", "make -C", "launcher install"},
		},
		{
			name: "explicit local checkout needs no release verifier", local: true, noGH: true,
			want: []string{"omarchy pkg add make go", "make -C", "launcher install", "launcher setup"}, absent: []string{"curl --", "git clone"},
		},
		{
			name: "checksum failure stops before install", downloadMode: "checksum", wantError: true,
			absent: []string{"launcher install", "launcher setup", "git clone", "make -C"},
		},
		{name: "binary and checksum replacement rejected", downloadMode: "forged-pair", wantError: true},
		{name: "invalid signature rejected", downloadMode: "signature", wantError: true},
		{name: "wrong repository rejected", downloadMode: "wrong-repository", wantError: true},
		{name: "wrong workflow rejected", downloadMode: "wrong-workflow", wantError: true},
		{name: "wrong source ref rejected", downloadMode: "wrong-ref", wantError: true},
		{name: "wrong source commit rejected", downloadMode: "wrong-commit", wantError: true},
		{name: "wrong signer digest rejected", downloadMode: "wrong-signer", wantError: true},
		{name: "wrong issuer rejected", downloadMode: "wrong-issuer", wantError: true},
		{name: "self-hosted runner rejected", downloadMode: "self-hosted", wantError: true},
		{name: "wrong predicate rejected", downloadMode: "wrong-predicate", wantError: true},
		{name: "missing attestation rejected", downloadMode: "bundle-missing", wantError: true},
		{name: "malformed attestation rejected", downloadMode: "bundle-malformed", wantError: true},
		{name: "authenticated manifest wrong version rejected", downloadMode: "manifest-version", wantError: true},
		{name: "authenticated manifest wrong commit rejected", downloadMode: "manifest-commit", wantError: true},
		{name: "authenticated manifest wrong platform rejected", downloadMode: "manifest-platform", wantError: true},
		{name: "authenticated manifest wrong schema rejected", downloadMode: "manifest-schema", wantError: true},
		{name: "authenticated manifest path rejected", downloadMode: "manifest-path", wantError: true},
		{name: "authenticated manifest invalid hash rejected", downloadMode: "manifest-hash", wantError: true},
		{name: "authenticated malformed manifest rejected", downloadMode: "manifest-malformed", wantError: true},
		{name: "missing release selection rejected", noSelection: true, wantError: true},
		{name: "invalid release rejected", noSelection: true, args: []string{"--release", "../main", "--source-commit", releaseCommit}, wantError: true},
		{name: "short commit rejected", noSelection: true, args: []string{"--release", releaseVersion, "--source-commit", "0123456"}, wantError: true},
		{name: "missing release option value rejected", noSelection: true, args: []string{"--release"}, wantError: true},
		{name: "missing commit option value rejected", noSelection: true, args: []string{"--release", releaseVersion, "--source-commit"}, wantError: true},
		{name: "mixed local and release modes rejected", args: []string{"--local"}, wantError: true},
		{name: "local mode requires checkout", noSelection: true, args: []string{"--local"}, wantError: true},
		{name: "missing GitHub CLI stops before changes", noGH: true, wantError: true},
		{
			name: "network failure does not fall back", downloadMode: "network", wantError: true,
			absent: []string{"launcher install", "launcher setup", "git clone", "make -C"},
		},
		{
			name: "missing Lua config stops before packages", noConfig: true, wantError: true,
			absent: []string{"omarchy", "curl --", "launcher install"},
		},
		{
			name: "release installation needs no external MSI verifier", piped: true, noVerifier: true,
			want: []string{"launcher install", "launcher setup"},
		},
		{
			name: "local installation needs no external MSI verifier", local: true, noVerifier: true,
			want: []string{"launcher install", "launcher setup"},
		},
		{
			name: "launcher-only installation needs no external MSI verifier", args: []string{"--launcher-only"}, noVerifier: true,
			want: []string{"launcher install"}, absent: []string{"launcher setup"},
		},
		{
			name: "package failure stops installation", pkgFail: true, wantError: true,
			want:   []string{"gh attestation verify", "omarchy pkg add umu-launcher"},
			absent: []string{"launcher install", "launcher setup"},
		},
		{
			name: "setup failure retains installed launcher", setupFail: true, wantError: true,
			want: []string{"launcher install", "launcher setup"},
		},
		{
			name: "help requires no system changes", args: []string{"--help"}, noConfig: true, noVerifier: true,
			absent: []string{"omarchy", "curl --", "launcher install"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), "desktop user with spaces")
			home := filepath.Join(root, "home")
			bin := filepath.Join(root, "bin")
			tmp := filepath.Join(root, "tmp")
			config := filepath.Join(root, "custom config")
			for _, dir := range []string{home, bin, tmp} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			// Keep host tools off the child PATH so missing dependencies cannot be
			// supplied by the workstation and package commands cannot reach it.
			for _, name := range []string{"bash", "chmod", "cp", "dirname", "jq", "mkdir", "mktemp", "realpath", "rm", "sha256sum"} {
				path, err := exec.LookPath(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path, filepath.Join(bin, name)); err != nil {
					t.Fatal(err)
				}
			}
			if !tt.noVerifier {
				fixture(t, filepath.Join(bin, "osslsigncode"), "#!/bin/bash\nexit 0\n", 0755)
			}
			for _, name := range []string{"yay", "paru", "makepkg", "pacman", "sudo"} {
				fixture(t, filepath.Join(bin, name), `#!/bin/bash
printf 'forbidden %s %s\n' "${0##*/}" "$*" >> "$TEST_LOG"
exit 1
`, 0755)
			}
			if !tt.noConfig {
				fixture(t, filepath.Join(config, "hypr/hyprland.lua"), "-- existing config\n", 0600)
			}
			installScript := filepath.Join(root, "checkout/install.sh")
			fixture(t, installScript, string(script), 0600)
			quickPath := filepath.Join(root, "bootstrap.sh")
			fixture(t, quickPath, string(quickScript), 0600)
			installedPath := filepath.Join(home, ".local/bin/ptcgl-omarchy")
			fixture(t, installedPath, "existing launcher\n", 0755)
			if tt.local {
				fixture(t, filepath.Join(root, "checkout/go.mod"), "module ptcgl-omarchy\n", 0600)
				fixture(t, filepath.Join(root, "checkout/cmd/ptcgl-omarchy/main.go"), "package main\n", 0600)
			}
			fixture(t, filepath.Join(root, "launcher"), launcherFixture, 0755)
			releaseFixtures(t, root, tt.downloadMode, string(script))
			selectedRelease := releaseVersion
			if tt.downloadMode == "api-old-release" {
				selectedRelease = "v0.3.0"
			}
			fixture(t, filepath.Join(root, "api-release.json"), fmt.Sprintf(`{"tag_name":%q}`, selectedRelease), 0600)
			objectType := "commit"
			if tt.downloadMode == "api-annotated" {
				objectType = "tag"
			} else if tt.downloadMode == "api-invalid-ref" {
				objectType = "tree"
			}
			fixture(t, filepath.Join(root, "api-ref.json"), fmt.Sprintf(`{"object":{"sha":%q,"type":%q}}`, releaseCommit, objectType), 0600)
			fixture(t, filepath.Join(root, "api-tag.json"), fmt.Sprintf(`{"object":{"sha":%q,"type":"commit"}}`, releaseCommit), 0600)
			downloadedInstaller := string(script)
			if tt.downloadMode == "bootstrap-tampered" {
				downloadedInstaller = "#!/bin/bash\nprintf 'unverified bootstrap ran\\n' >> \"$TEST_LOG\"\n"
			}
			fixture(t, filepath.Join(root, "downloaded-installer"), downloadedInstaller, 0600)
			fixture(t, filepath.Join(bin, "uname"), "#!/bin/bash\ncase $1 in -s) printf 'Linux\\n';; -m) printf 'x86_64\\n';; esac\n", 0755)
			fixture(t, filepath.Join(bin, "omarchy"), `#!/bin/bash
printf 'omarchy %s\n' "$*" >> "$TEST_LOG"
[[ ${TEST_PKG_FAIL:-0} != 1 ]]
`, 0755)
			fixture(t, filepath.Join(bin, "curl"), `#!/bin/bash
set -e
printf 'curl %s\n' "$*" >> "$TEST_LOG"
while (( $# )); do
  case "$1" in
    --output) dest=$2; shift ;;
    --proto) proto=$2; shift ;;
    --proto-redir) proto_redir=$2; shift ;;
    https://*) url=$1 ;;
  esac
  shift
done
[[ $proto == '=https' && $proto_redir == '=https' ]]
[[ $TEST_DOWNLOAD_MODE != notfound && $TEST_DOWNLOAD_MODE != network ]] || exit 22
base="https://github.com/hucKOder/omarchy-ptcgl/releases/download/$TEST_RELEASE"
case "$url" in
  "https://api.github.com/repos/hucKOder/omarchy-ptcgl/releases/latest") cp "$TEST_FIXTURES/api-release.json" "$dest" ;;
  "https://api.github.com/repos/hucKOder/omarchy-ptcgl/git/ref/tags/$TEST_RELEASE") cp "$TEST_FIXTURES/api-ref.json" "$dest" ;;
  "https://api.github.com/repos/hucKOder/omarchy-ptcgl/git/tags/$TEST_SOURCE_COMMIT") cp "$TEST_FIXTURES/api-tag.json" "$dest" ;;
  "$base/install.sh") cp "$TEST_FIXTURES/downloaded-installer" "$dest" ;;
  "$base/release.json") cp "$TEST_FIXTURES/release.json" "$dest" ;;
  "$base/release.sigstore.json")
    [[ $TEST_DOWNLOAD_MODE != bundle-missing ]] || exit 22
    cp "$TEST_FIXTURES/release.sigstore.json" "$dest"
    ;;
  "$base/ptcgl-omarchy-linux-amd64") cp "$TEST_FIXTURES/release-launcher" "$dest" ;;
  *) exit 1 ;;
esac
`, 0755)
			if !tt.noGH {
				fixture(t, filepath.Join(bin, "gh"), `#!/bin/bash
set -euo pipefail
printf 'gh %s\n' "$*" >> "$TEST_LOG"
[[ $1 == attestation && $2 == verify ]]
shift 2
if [[ ${1:-} == --help ]]; then
  if [[ $TEST_OLD_GH == 1 ]]; then
    printf '%s\n' '--bundle --repo'
  else
    printf '%s\n' '--bundle --hostname --repo --cert-identity --cert-oidc-issuer --source-ref --source-digest --signer-digest --deny-self-hosted-runners --predicate-type'
  fi
  exit 0
fi
artifact=$1
shift
declare -A policy=()
deny_self_hosted=false
while (( $# )); do
  case "$1" in
    --deny-self-hosted-runners) deny_self_hosted=true; shift ;;
    --bundle|--hostname|--repo|--cert-identity|--cert-oidc-issuer|--source-ref|--source-digest|--signer-digest|--predicate-type)
      policy[$1]=$2; shift 2 ;;
    *) exit 1 ;;
  esac
done
[[ $deny_self_hosted == true && ${policy[--hostname]} == github.com ]]
[[ ${policy[--repo]} == hucKOder/omarchy-ptcgl ]]
[[ ${policy[--cert-identity]} == "https://github.com/hucKOder/omarchy-ptcgl/.github/workflows/release.yml@refs/tags/$TEST_RELEASE" ]]
[[ ${policy[--cert-oidc-issuer]} == https://token.actions.githubusercontent.com ]]
[[ ${policy[--source-ref]} == "refs/tags/$TEST_RELEASE" ]]
[[ ${policy[--source-digest]} == "$TEST_SOURCE_COMMIT" && ${policy[--signer-digest]} == "$TEST_SOURCE_COMMIT" ]]
[[ ${policy[--predicate-type]} == https://slsa.dev/provenance/v1 ]]
hash=$(sha256sum "$artifact")
jq -e --arg hash "${hash%% *}" --arg repo "${policy[--repo]}" \
  --arg identity "${policy[--cert-identity]}" --arg issuer "${policy[--cert-oidc-issuer]}" \
  --arg ref "${policy[--source-ref]}" --arg commit "${policy[--source-digest]}" \
  --arg signer "${policy[--signer-digest]}" --arg predicate "${policy[--predicate-type]}" '
  .signature_valid == true and (.subjects | index($hash)) != null and .repository == $repo and
  .identity == $identity and .issuer == $issuer and .source_ref == $ref and
  .source_commit == $commit and .signer_commit == $signer and
  .runner == "github-hosted" and .predicate == $predicate
' "${policy[--bundle]}" >/dev/null
`, 0755)
			}
			fixture(t, filepath.Join(bin, "git"), `#!/bin/bash
printf 'git %s\n' "$*" >> "$TEST_LOG"
mkdir -p "${@: -1}"
`, 0755)
			fixture(t, filepath.Join(bin, "make"), `#!/bin/bash
set -e
printf 'make %s\n' "$*" >> "$TEST_LOG"
[[ $1 == -C && $3 == build ]]
mkdir -p "$2/build"
cp "$TEST_FIXTURES/launcher" "$2/build/ptcgl-omarchy"
`, 0755)
			args := []string{"--release", releaseVersion, "--source-commit", releaseCommit}
			if tt.local {
				args = []string{"--local"}
			}
			if tt.noSelection {
				args = nil
			}
			args = append(args, tt.args...)
			cmd := exec.Command("bash", append([]string{installScript}, args...)...)
			if tt.piped {
				cmd = exec.Command("bash", append([]string{"-s", "--"}, args...)...)
				cmd.Stdin = strings.NewReader(string(script))
			}
			if tt.bootstrap {
				block := installBlock(t, generatedNotes(t, releaseVersion, releaseCommit))
				if len(tt.args) != 0 {
					invocation := `bash "$work_dir/install.sh" --release "$release" --source-commit "$source_commit"`
					block = strings.Replace(block, invocation, invocation+" "+strings.Join(tt.args, " "), 1)
				}
				cmd = exec.Command("bash")
				cmd.Stdin = strings.NewReader(block)
			}
			if tt.quick {
				if tt.discover {
					args = tt.args
				}
				cmd = exec.Command("bash", append([]string{quickPath}, args...)...)
			}
			logPath := filepath.Join(root, "commands.log")
			cmd.Env = append(os.Environ(),
				"HOME="+home, "XDG_CONFIG_HOME="+config, "TMPDIR="+tmp,
				"PATH="+bin, "TEST_LOG="+logPath,
				"TEST_FIXTURES="+root, "TEST_DOWNLOAD_MODE="+tt.downloadMode,
				"TEST_RELEASE="+releaseVersion, "TEST_SOURCE_COMMIT="+releaseCommit,
				fmt.Sprintf("TEST_PKG_FAIL=%d", boolInt(tt.pkgFail)),
				fmt.Sprintf("TEST_SETUP_FAIL=%d", boolInt(tt.setupFail)),
				fmt.Sprintf("TEST_OLD_GH=%d", boolInt(tt.oldGH)),
			)
			output, err := cmd.CombinedOutput()
			if (err != nil) != tt.wantError {
				t.Fatalf("installer error = %v, wantError = %v\n%s", err, tt.wantError, output)
			}
			log, _ := os.ReadFile(logPath)
			if !tt.local && !tt.wantError && !strings.Contains(strings.Join(tt.args, " "), "--help") {
				verified := strings.Index(string(log), " --bundle ")
				packages := strings.Index(string(log), "omarchy pkg add umu-launcher")
				if verified < 0 || packages < 0 || verified > packages {
					t.Errorf("release authentication did not precede package installation:\n%s", log)
				}
				if tt.bootstrap || tt.quick {
					installerAuth := strings.Index(string(log), "install.sh --bundle ")
					metadataAuth := strings.Index(string(log), "release.json --bundle ")
					if installerAuth < 0 || metadataAuth < installerAuth || metadataAuth > packages {
						t.Errorf("bootstrap did not authenticate installer, then metadata, before installation:\n%s", log)
					}
				}
			}
			forbiddenCommands := []string{"omarchy pkg aur", "forbidden ", "git clone"}
			if !tt.quick {
				forbiddenCommands = append(forbiddenCommands, "releases/latest")
			}
			for _, forbidden := range forbiddenCommands {
				if strings.Contains(string(log), forbidden) {
					t.Errorf("unexpected package build/install command %q in:\n%s", forbidden, log)
				}
			}
			if tt.wantError && !tt.pkgFail && !tt.setupFail {
				packageMutation := "omarchy "
				if tt.quick {
					packageMutation = "omarchy pkg add umu-launcher"
				}
				for _, mutation := range []string{packageMutation, "make -C", "launcher "} {
					if strings.Contains(string(log), mutation) {
						t.Errorf("preflight/authentication failure allowed %q in:\n%s", mutation, log)
					}
				}
			}
			if tt.oldGH && !strings.Contains(string(output), "Update github-cli") {
				t.Errorf("old GitHub CLI has no recovery guidance: %s", output)
			}
			for _, want := range tt.want {
				if !strings.Contains(string(log), want) {
					t.Errorf("missing command %q in:\n%s", want, log)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(string(log), absent) {
					t.Errorf("unexpected command %q in:\n%s", absent, log)
				}
			}
			leftovers, err := os.ReadDir(tmp)
			if err != nil || len(leftovers) != 0 {
				t.Errorf("temporary installation files retained: %v (%v)", leftovers, err)
			}
			if tt.setupFail {
				if _, err := os.Stat(installedPath); err != nil {
					t.Errorf("launcher unavailable for resuming setup: %v", err)
				}
				if !strings.Contains(string(output), "Resume with:") || strings.Contains(string(output), "Installed!") {
					t.Errorf("incorrect setup failure message: %s", output)
				}
			} else if tt.wantError {
				installed, err := os.ReadFile(installedPath)
				if err != nil || string(installed) != "existing launcher\n" {
					t.Errorf("failed installation replaced the existing launcher: %s (%v)", installed, err)
				}
			}
		})
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
