package installer_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestInstaller(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the installer requires a non-root desktop user")
	}
	script, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, downloadMode string
		args               []string
		local, piped       bool
		noConfig, pkgFail  bool
		setupFail          bool
		wantError          bool
		want, absent       []string
	}{
		{
			name: "release prepares game", piped: true,
			want:   []string{"omarchy pkg add umu-launcher", "omarchy pkg aur add osslsigncode", "omarchy install gaming gpu-lib32", "launcher install", "launcher setup"},
			absent: []string{"omarchy pkg add make go", "git clone", "make -C"},
		},
		{
			name: "launcher only", args: []string{"--launcher-only"},
			want: []string{"launcher install"}, absent: []string{"launcher setup", "make -C"},
		},
		{
			name: "no release builds main", downloadMode: "notfound",
			want: []string{"omarchy pkg add git", "omarchy pkg add make go", "git clone --depth 1 --branch main", "make -C", "launcher install", "launcher setup"},
		},
		{
			name: "local checkout", local: true,
			want: []string{"omarchy pkg add make go", "make -C", "launcher install", "launcher setup"}, absent: []string{"curl --", "git clone"},
		},
		{
			name: "checksum failure stops before install", downloadMode: "checksum", wantError: true,
			absent: []string{"launcher install", "launcher setup", "git clone", "make -C"},
		},
		{
			name: "network failure does not fall back", downloadMode: "network", wantError: true,
			absent: []string{"launcher install", "launcher setup", "git clone", "make -C"},
		},
		{
			name: "missing Lua config stops before packages", noConfig: true, wantError: true,
			absent: []string{"omarchy", "curl --", "launcher install"},
		},
		{
			name: "package failure stops installation", pkgFail: true, wantError: true,
			absent: []string{"curl --", "launcher install", "launcher setup"},
		},
		{
			name: "setup failure retains installed launcher", setupFail: true, wantError: true,
			want: []string{"launcher install", "launcher setup"},
		},
		{
			name: "help requires no system changes", args: []string{"--help"}, noConfig: true,
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
			if !tt.noConfig {
				fixture(t, filepath.Join(config, "hypr/hyprland.lua"), "-- existing config\n", 0600)
			}
			installScript := filepath.Join(root, "checkout/install.sh")
			fixture(t, installScript, string(script), 0600)
			installedPath := filepath.Join(home, ".local/bin/ptcgl-omarchy")
			fixture(t, installedPath, "existing launcher\n", 0755)
			if tt.local {
				fixture(t, filepath.Join(root, "checkout/go.mod"), "module ptcgl-omarchy\n", 0600)
				fixture(t, filepath.Join(root, "checkout/cmd/ptcgl-omarchy/main.go"), "package main\n", 0600)
			}
			fixture(t, filepath.Join(root, "launcher"), launcherFixture, 0755)
			fixture(t, filepath.Join(root, "checksum"), fmt.Sprintf("%x  ptcgl-omarchy-linux-amd64\n", sha256.Sum256([]byte(launcherFixture))), 0600)
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
    https://*) url=$1 ;;
  esac
  shift
done
case "$url" in
  */releases/latest)
    case "$TEST_DOWNLOAD_MODE" in
      network) printf '503'; exit 22 ;;
      notfound) printf '404'; exit 22 ;;
      *) printf '{"tag_name":"v0.1.0"}\n' > "$dest"; printf '200' ;;
    esac
    ;;
  *.sha256)
    if [[ $TEST_DOWNLOAD_MODE == checksum ]]; then
      printf '%064d  ptcgl-omarchy-linux-amd64\n' 0 > "$dest"
    else
      cp "$TEST_FIXTURES/checksum" "$dest"
    fi
    ;;
  */ptcgl-omarchy-linux-amd64) cp "$TEST_FIXTURES/launcher" "$dest" ;;
  *) exit 1 ;;
esac
`, 0755)
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
			cmd := exec.Command("bash", append([]string{installScript}, tt.args...)...)
			if tt.piped {
				cmd = exec.Command("bash")
				cmd.Stdin = strings.NewReader(string(script))
			}
			logPath := filepath.Join(root, "commands.log")
			cmd.Env = append(os.Environ(),
				"HOME="+home, "XDG_CONFIG_HOME="+config, "TMPDIR="+tmp,
				"PATH="+bin+":"+os.Getenv("PATH"), "TEST_LOG="+logPath,
				"TEST_FIXTURES="+root, "TEST_DOWNLOAD_MODE="+tt.downloadMode,
				fmt.Sprintf("TEST_PKG_FAIL=%d", boolInt(tt.pkgFail)),
				fmt.Sprintf("TEST_SETUP_FAIL=%d", boolInt(tt.setupFail)),
			)
			output, err := cmd.CombinedOutput()
			if (err != nil) != tt.wantError {
				t.Fatalf("installer error = %v, wantError = %v\n%s", err, tt.wantError, output)
			}
			log, _ := os.ReadFile(logPath)
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
