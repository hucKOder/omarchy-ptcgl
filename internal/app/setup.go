package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const gameRelative = "drive_c/users/steamuser/The Pokémon Company International/Pokémon Trading Card Game Live/Pokemon TCG Live.exe"

func (a *App) gamePath() string { return filepath.Join(a.Paths.Prefix, gameRelative) }

func (a *App) umu(args ...string) Command {
	return Command{Name: "umu-run", Args: args, Env: environment(map[string]string{
		"GAMEID": "0", "WINEPREFIX": a.Paths.Prefix, "PROTONPATH": a.protonPath(),
		"UMU_CONTAINER_NSENTER": "1", "PROTON_VERB": "waitforexitandrun",
	})}
}

func (a *App) prefixReady() bool {
	for _, name := range []string{"system.reg", "user.reg", "userdef.reg", "drive_c/windows/system32/msiexec.exe"} {
		if !exists(filepath.Join(a.Paths.Prefix, name)) {
			return false
		}
	}
	return true
}

func (a *App) bootstrap(ctx context.Context) error {
	if err := a.require("umu-run"); err != nil {
		return err
	}
	if err := a.ensureProton(ctx); err != nil {
		return err
	}
	if !a.prefixReady() {
		a.progress("Initializing the game prefix…")
		if err := os.MkdirAll(filepath.Dir(a.Paths.Prefix), 0700); err != nil {
			return err
		}
		if err := a.Runner.Run(ctx, a.umu("")); err != nil {
			return fmt.Errorf("initialize prefix: %w", err)
		}
		if !a.prefixReady() {
			return fmt.Errorf("prefix initialization is incomplete; run setup again")
		}
	}
	if !a.vcReady() {
		a.progress("Installing the Visual C++ runtime…")
		if err := a.Runner.Run(ctx, a.umu("winetricks", "-q", "vcrun2019")); err != nil {
			return fmt.Errorf("install Visual C++: %w", err)
		}
		if !a.vcReady() {
			return fmt.Errorf("Visual C++ runtime installation is incomplete")
		}
	}
	return nil
}

func (a *App) vcReady() bool {
	for _, dll := range []string{"concrt140.dll", "msvcp140.dll", "vcruntime140.dll", "vcruntime140_1.dll"} {
		if !exists(filepath.Join(a.Paths.Prefix, "drive_c/windows/system32", dll)) {
			return false
		}
	}
	return true
}

func (a *App) setup(ctx context.Context) error {
	if !exists(a.gamePath()) {
		if err := a.require("osslsigncode"); err != nil {
			return err
		}
	}
	if err := a.bootstrap(ctx); err != nil {
		return err
	}
	if exists(a.gamePath()) {
		a.progress("Game is installed. Normal patches are handled by the game's updater.")
		return nil
	}
	_, err := a.install(ctx, "setup")
	return err
}

type Installation struct {
	InstallerSHA256 string    `json:"installer_sha256"`
	AppliedAt       time.Time `json:"applied_at"`
	Operation       string    `json:"operation"`
	ProtonRelease   string    `json:"proton_release"`
}

func (a *App) installation() Installation {
	var state Installation
	b, _ := os.ReadFile(filepath.Join(a.Paths.State, "installation.json"))
	_ = json.Unmarshal(b, &state)
	return state
}

// Update always retrieves the current MSI: Pokémon's endpoint is mutable.
// It does not update Proton or reset the prefix. Same-MSI updates are no-ops;
// repair explicitly reinstalls files even when the MSI hash is unchanged.
func (a *App) Update(ctx context.Context, repair, checkOnly bool) error {
	_, err := a.update(ctx, repair, checkOnly)
	return err
}

type installerOutcome int

const (
	installerUnchanged installerOutcome = iota + 1
	installerAvailable
	installerApplied
	installerRepaired
)

func (a *App) update(ctx context.Context, repair, checkOnly bool) (installerOutcome, error) {
	var outcome installerOutcome
	err := a.withLock(func() error {
		if !exists(a.gamePath()) && !repair {
			return fmt.Errorf("game is not installed; run ptcgl-omarchy setup first")
		}
		if err := a.require("umu-run", "osslsigncode"); err != nil {
			return err
		}
		if !checkOnly {
			if err := a.bootstrap(ctx); err != nil {
				return err
			}
		}
		op := "update"
		if repair {
			op = "repair"
		}
		if checkOnly {
			op = "check"
		}
		var err error
		outcome, err = a.install(ctx, op)
		return err
	})
	return outcome, err
}

func (a *App) install(ctx context.Context, operation string) (installerOutcome, error) {
	a.progress("Downloading the current official Pokémon installer…")
	installer := filepath.Join(a.Paths.Cache, "PokemonTCGLiveInstaller.msi")
	if err := a.Download.Download(ctx, a.Profile.InstallerURL, installer, 1<<30,
		func(path string) error { return a.verifyInstaller(ctx, path) }); err != nil {
		return 0, err
	}
	hash, err := fileSHA256(installer)
	if err != nil {
		return 0, err
	}
	if operation == "check" {
		outcome := installerAvailable
		if hash == a.installation().InstallerSHA256 {
			a.progress("The official installer matches the last applied installer.")
			outcome = installerUnchanged
		} else {
			a.progress("The official installer differs from the last applied installer. Run ptcgl-omarchy update to apply it.")
		}
		a.progress("In-game patches are checked by the game at launch.")
		return outcome, nil
	}
	if operation == "update" && hash == a.installation().InstallerSHA256 && exists(a.gamePath()) {
		a.progress("The official installer has not changed. Launch the game to apply in-game patches.")
		return installerUnchanged, nil
	}
	a.progress("Applying Pokémon TCG Live installer (" + operation + ")…")
	msiexec := filepath.Join(a.Paths.Prefix, "drive_c/windows/system32/msiexec.exe")
	args := []string{msiexec, "/i", windowsPath(installer), "/quiet", "/norestart"}
	if operation == "repair" && hash == a.installation().InstallerSHA256 && exists(a.gamePath()) {
		args = append(args, "REINSTALL=ALL", "REINSTALLMODE=vomus")
	}
	c := a.umu(args...)
	c.Dir = filepath.Dir(installer)
	if err := a.Runner.Run(ctx, c); err != nil {
		var exit *exec.ExitError
		// Wine exposes Windows success-with-reboot-required as its low byte.
		if !errors.As(err, &exit) || (exit.ExitCode() != 194 && exit.ExitCode() != 3010) {
			return 0, fmt.Errorf("installer failed; retry %s or run doctor: %w", operation, err)
		}
	}
	if !exists(a.gamePath()) {
		return 0, fmt.Errorf("installer completed without producing the game executable")
	}
	state := Installation{hash, time.Now().UTC(), operation, a.Profile.ProtonRelease}
	if err := writeJSON(filepath.Join(a.Paths.State, "installation.json"), state); err != nil {
		return 0, err
	}
	a.progress("Game installation complete. Launch it to finish any in-game updates.")
	if operation == "repair" {
		return installerRepaired, nil
	}
	return installerApplied, nil
}

func windowsPath(path string) string { return "Z:" + strings.ReplaceAll(path, "/", `\`) }

var signerSubject = regexp.MustCompile(`(?m)^Signer's certificate:\s*\n\s*-+\s*\n\s*Signer #0:\s*\n\s*Subject:\s*(.+)$`)

func validatePublisher(output []byte) error {
	text := string(output)
	if !strings.Contains(text, "Signature verification: ok") || !strings.Contains(text, "Succeeded") {
		return fmt.Errorf("installer signature verification failed")
	}
	match := signerSubject.FindStringSubmatch(text)
	if len(match) != 2 {
		return fmt.Errorf("installer publisher identity is missing")
	}
	// Split only unescaped commas in the primary signer's distinguished name.
	var fields []string
	start, escaped := 0, false
	for i, r := range match[1] {
		if r == ',' && !escaped {
			fields = append(fields, strings.TrimSpace(match[1][start:i]))
			start = i + 1
		}
		if r == '\\' {
			escaped = !escaped
		} else {
			escaped = false
		}
	}
	fields = append(fields, strings.TrimSpace(match[1][start:]))
	cn, org := false, false
	for _, field := range fields {
		cn = cn || field == `CN=The Pokemon Company International\, Inc.`
		org = org || field == `O=The Pokemon Company International\, Inc.`
	}
	if !cn || !org {
		return fmt.Errorf("installer is not signed by Pokémon's publisher")
	}
	return nil
}

func (a *App) verifyInstaller(ctx context.Context, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var magic [8]byte
	if _, err := f.Read(magic[:]); err != nil || magic != [8]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1} {
		return fmt.Errorf("download is not a Windows MSI installer")
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	output, err := a.Runner.Output(verifyCtx, Command{Name: "osslsigncode", Args: []string{"verify", "-in", path}})
	if err != nil {
		return fmt.Errorf("installer signature or certificate verification failed")
	}
	return validatePublisher(output)
}
