package app

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var labelPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+\-]{0,99}$`)

func safeLabel(label string) string {
	if !labelPattern.MatchString(label) {
		return "Unknown"
	}
	return label
}

// Reports deliberately use a field allowlist, not best-effort token redaction.
// Raw diagnostic details, paths, config, environment, and process output stay
// local. A future check must explicitly opt into the shareable format.
func (a *App) SupportReport(checks []Check) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Pokémon TCG Live — support report\nLauncher: %s\nPlatform: linux/amd64\n", safeLabel(Version))
	runtime := safeLabel(a.Profile.ProtonRelease)
	if a.Config.ProtonPath != "" {
		runtime = "Custom runtime (user-managed)"
	}
	fmt.Fprintf(&b, "Runtime: %s\n", runtime)
	i := a.installerStatus()
	if i.LastApplied != "" {
		fmt.Fprintf(&b, "Last installer application: %s\n", i.LastApplied)
	}
	fmt.Fprintf(&b, "Installer: %s\n", i.Freshness)
	if i.CheckedAt != "" {
		fmt.Fprintf(&b, "Installer last checked: %s\n", i.CheckedAt)
	}
	b.WriteString("Game patches: checked in-game at launch; not inspected by this report.\n\nChecks (personal paths and raw error output omitted):\n")
	known := map[string]bool{}
	for _, name := range []string{"omarchy", "hyprctl", "umu-run", "zenity", "xdg-mime", "Hyprland session", "Proton", "prefix", "Visual C++ runtime", "game", "app-menu entry", "callback entry", "browser callback", "Hyprland rules", "Hyprland config"} {
		known[name] = true
	}
	for _, check := range checks {
		if !known[check.Name] {
			continue
		}
		state := "OK"
		if !check.OK {
			state = "FAIL"
		}
		fmt.Fprintf(&b, "%s  %s\n", state, check.Name)
		if !check.OK {
			if description, ok := fixDescription(check.Action); ok {
				fmt.Fprintf(&b, "    Next action: %s\n", description)
			}
		}
	}
	return b.String()
}

func fixDescription(id string) (string, bool) {
	switch id {
	case "integration":
		return "Refresh app-menu entries, browser login handler, and managed window rules. Unrelated desktop settings are retained.", true
	case "setup":
		return "Retry runtime and game setup. Incomplete managed runtime files may be replaced; the existing game prefix is retained.", true
	case "repair":
		return "Reapply the verified official game installer in the existing prefix. The launcher does not reset the prefix or settings.", true
	case "runtime-update":
		return "Select this launcher's bundled runtime profile. Previous managed versions and the existing prefix are retained. Wine may update the prefix on next launch. Custom runtimes are not changed.", true
	}
	return "", false
}

func (a *App) Fix(ctx context.Context, id string, gui bool) error {
	description, ok := fixDescription(id)
	if !ok {
		return fmt.Errorf("unsupported recovery action; use integration, setup, repair, or runtime-update")
	}
	if gui {
		err := exec.CommandContext(ctx, "zenity", "--question", "--no-markup", "--title=Confirm recovery action", "--text="+description+"\n\nClose the game before continuing.", "--width=500").Run()
		if dialogDismissed(err) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	operation := func(ctx context.Context) error {
		switch id {
		case "integration":
			return a.Integrate(ctx)
		case "setup":
			return a.Setup(ctx)
		case "repair":
			return a.Update(ctx, true, false)
		case "runtime-update":
			return a.UpdateRuntime(ctx)
		}
		return fmt.Errorf("unsupported recovery action")
	}
	if !gui {
		return operation(ctx)
	}
	return a.withProgress(ctx, "Pokémon TCG Live — recovery", description, operation)
}

func (a *App) DiagnosticsGUI(ctx context.Context) error {
	for {
		checks := a.Doctor(ctx)
		report := a.SupportReport(checks)
		args := []string{"--text-info", "--title=Pokémon TCG Live diagnostics", "--width=850", "--height=550", "--extra-button=Save report"}
		if _, err := a.LookPath("wl-copy"); err == nil {
			args = append(args, "--extra-button=Copy report")
		}
		allowed := make(map[string]string)
		if a.Status(ctx).Capabilities.Maintain {
			for _, check := range checks {
				if check.OK {
					continue
				}
				label := map[string]string{"integration": "Fix integration", "setup": "Retry setup", "repair": "Repair game"}[check.Action]
				if label != "" && allowed[label] == "" {
					args = append(args, "--extra-button="+label)
					allowed[label] = check.Action
				}
			}
		}
		cmd := exec.CommandContext(ctx, "zenity", args...)
		cmd.Stdin = strings.NewReader(report + "\nLocal diagnostic details (not included in shared reports):\n\n" + FormatChecks(checks))
		output, err := cmd.Output()
		choice := strings.TrimSpace(string(output))
		if choice == "" {
			if dialogDismissed(err) {
				return nil
			}
			return err
		}
		switch choice {
		case "Save report":
			if err := saveReportGUI(ctx, report); err != nil {
				return err
			}
		case "Copy report":
			cmd := exec.CommandContext(ctx, "wl-copy")
			cmd.Stdin = strings.NewReader(report)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("copy support report: %w", err)
			}
		default:
			if id := allowed[choice]; id != "" {
				if err := a.Fix(ctx, id, true); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("unsupported diagnostic action")
			}
		}
	}
}

func saveReportGUI(ctx context.Context, report string) error {
	cmd := exec.CommandContext(ctx, "zenity", "--file-selection", "--save", "--confirm-overwrite", "--filename=ptcgl-support.txt", "--title=Save privacy-safe support report")
	output, err := cmd.Output()
	if dialogDismissed(err) {
		return nil
	}
	if err != nil {
		return err
	}
	path := strings.TrimSuffix(string(output), "\n")
	if path == "" {
		return nil
	}
	return AtomicWrite(path, []byte(report), 0o600)
}

func (a *App) UpdatesSummary() string {
	i := a.installerStatus()
	runtime := safeLabel(a.Profile.ProtonRelease)
	if a.Config.ProtonPath != "" {
		runtime = "Custom runtime (user-managed)"
	}
	last := "Not recorded"
	if i.LastApplied != "" {
		last = i.LastApplied
	}
	fresh := "Not checked. Choose Check installer to compare the official MSI."
	if i.CheckedAt != "" {
		fresh = "Installer differed at last check: " + i.CheckedAt
		if i.Freshness == "unchanged-at-check" {
			fresh = "Installer matched at last check: " + i.CheckedAt
		}
	}
	return fmt.Sprintf("Launcher: %s (installed version; online freshness not checked)\nUpgrade the launcher explicitly from the companion or the verified release installation.\n\nCompatibility runtime: %s\nBundled profile: %s\nRuntime selection is separate from launcher and game updates.\n\nOfficial installer last applied: %s\n%s\n\nGame patches: checked in-game at launch, not by the installer comparison.\n\nEach update changes only its selected component. No automatic upgrades.", safeLabel(Version), runtime, safeLabel(DefaultProfile().ProtonRelease), last, fresh)
}

func (a *App) UpdatesGUI(ctx context.Context) error {
	for {
		args := []string{"--text-info", "--title=Pokémon TCG Live updates", "--width=700", "--height=480"}
		canMaintain := a.Status(ctx).Capabilities.Maintain
		if canMaintain {
			args = append(args, "--extra-button=Check installer", "--extra-button=Update installer")
			if a.Config.ProtonPath == "" {
				args = append(args, "--extra-button=Select bundled runtime")
			}
		}
		text := a.UpdatesSummary()
		if !canMaintain {
			text += "\n\nMaintenance unavailable: close the game and other operations, or open Diagnostics if status cannot be read."
		}
		cmd := exec.CommandContext(ctx, "zenity", args...)
		cmd.Stdin = strings.NewReader(text)
		out, err := cmd.Output()
		choice := strings.TrimSpace(string(out))
		if choice == "" {
			if dialogDismissed(err) {
				return nil
			}
			return err
		}
		if !canMaintain {
			return fmt.Errorf("maintenance unavailable")
		}
		switch choice {
		case "Check installer":
			err = a.UpdateGUI(ctx, false, true)
		case "Update installer":
			err = exec.CommandContext(ctx, "zenity", "--question", "--no-markup", "--title=Update official installer", "--text=Apply the verified official installer in the existing prefix? The launcher does not reset settings. This does not upgrade the launcher or select another runtime.").Run()
			if dialogDismissed(err) {
				continue
			}
			if err == nil {
				err = a.UpdateGUI(ctx, false, false)
			}
		case "Select bundled runtime":
			err = a.Fix(ctx, "runtime-update", true)
		default:
			return fmt.Errorf("unsupported update action")
		}
		if err != nil {
			return err
		}
	}
}
