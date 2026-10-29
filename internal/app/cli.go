package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

const help = `ptcgl-omarchy — Pokémon TCG Live for Omarchy

Usage: ptcgl-omarchy <command> [options]

  app                 First-launch setup dialogs, then play (default)
  setup [--gui]       Install the selected runtime and official game client
  play [-- <args>]    Launch or focus the game in windowed mode
  update [--check]    Refresh the official installer; preserve the game prefix
  repair [--gui]      Reapply installer files without resetting game data
  runtime-update     Select the compatibility profile bundled with this release
  install            Install this binary and Omarchy integration for your user
  integrate          Register app menu, login callback, and tiled window rules
  doctor [--json]     Check runtime, installation, and desktop integration
  doctor --save FILE  Save a privacy-safe support report
  status [--json]     Read installation readiness and current activity
  updates [--gui]     Explain component versions and explicit update actions
  fix [--gui] <id>    Confirm/run a supported recovery action
  open <folder>       Open game, config, or cache in your file manager
  open --gui          Choose a folder graphically
  uninstall          Remove binary and integration; retain installed game data
  version            Print launcher version
  licenses           Print bundled software licenses

setup, update, repair, runtime-update, doctor, and open accept --gui.
Game patches run through the built-in updater at launch. update --check compares
the official MSI with the last applied MSI; it does not check in-game patches.
`

func RunCLI(ctx context.Context, args []string, out, errOut io.Writer) int {
	command := "app"
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}
	if command == "help" || command == "--help" || command == "-h" {
		fmt.Fprint(out, help)
		return 0
	}
	if command == "version" || command == "--version" {
		fmt.Fprintf(out, "%s %s\n", Name, Version)
		return 0
	}
	if command == "licenses" {
		fmt.Fprint(out, thirdPartyNotices)
		return 0
	}
	// Private worker: independent of launcher configuration and desktop state.
	if command == "__verify-installer" {
		if len(args) != 1 {
			fmt.Fprintln(errOut, "installer verification requires one MSI path")
			return 2
		}
		if err := verifyMSI(ctx, args[0], nil); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		return 0
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	gui := fs.Bool("gui", false, "show graphical dialogs")
	checkOnly := fs.Bool("check", false, "compare the latest official installer without applying it")
	jsonOutput := fs.Bool("json", false, "print structured diagnostics")
	saveReport := fs.String("save", "", "save a privacy-safe support report (doctor only)")
	// Never send callback URLs through flag parsing or its error output.
	if command != "callback" {
		if err := fs.Parse(args); err != nil {
			if err == flag.ErrHelp {
				return 0
			}
			return 2
		}
	}
	if command != "play" && command != "open" && command != "fix" && len(fs.Args()) > 0 {
		fmt.Fprintln(errOut, "unexpected positional arguments")
		return 2
	}
	if *checkOnly && command != "update" {
		fmt.Fprintln(errOut, "--check is only valid with update")
		return 2
	}
	if *jsonOutput && command != "doctor" && command != "status" {
		fmt.Fprintln(errOut, "--json is only valid with doctor or status")
		return 2
	}
	if *saveReport != "" && (command != "doctor" || *jsonOutput || *gui) {
		fmt.Fprintln(errOut, "--save is only valid with terminal doctor, without --json")
		return 2
	}
	reportError := func(err error) int {
		message := strings.TrimSpace(err.Error())
		fmt.Fprintln(errOut, message)
		if *gui || command == "app" {
			ShowError(message)
		}
		return 1
	}
	if command == "callback" && len(args) != 1 {
		return reportError(ErrCallback)
	}
	p, err := UserPaths()
	if err != nil {
		if command == "status" {
			return outputStatus(out, unavailableStatus(), *jsonOutput)
		}
		return reportError(err)
	}
	var a *App
	if command == "open" {
		// Folder access also works when configuration/runtime state needs repair.
		a = &App{Paths: p, Runner: OSRunner{out, errOut}, LookPath: exec.LookPath}
	} else {
		a, err = New(p, out, errOut)
		if err != nil {
			if command == "status" {
				return outputStatus(out, unavailableStatus(), *jsonOutput)
			}
			return reportError(err)
		}
	}
	operation := func(fn func(context.Context) error) error {
		if *gui {
			return a.WithProgress(ctx, fn)
		}
		return fn(ctx)
	}
	switch command {
	case "callback":
		err = a.Callback(ctx, args[0])
	case "setup":
		err = operation(a.Setup)
	case "play":
		err = a.play(ctx, fs.Args(), *gui)
	case "app":
		err = a.Desktop(ctx)
	case "update", "repair":
		repair := command == "repair"
		if *gui {
			err = a.UpdateGUI(ctx, repair, *checkOnly)
		} else {
			err = a.Update(ctx, repair, *checkOnly)
		}
	case "runtime-update":
		err = operation(a.UpdateRuntime)
	case "install":
		err = a.InstallSelf(ctx)
	case "integrate":
		err = a.Integrate(ctx)
	case "uninstall":
		err = a.Uninstall(ctx)
	case "open":
		if len(fs.Args()) > 1 || (len(fs.Args()) == 0 && !*gui) {
			fmt.Fprintln(errOut, "usage: ptcgl-omarchy open <game|config|cache> or ptcgl-omarchy open --gui")
			return 2
		}
		if len(fs.Args()) == 0 {
			err = a.ChooseFolder(ctx)
		} else {
			err = a.OpenFolder(ctx, fs.Arg(0))
		}
	case "doctor":
		checks := a.Doctor(ctx)
		if *gui {
			err = a.DiagnosticsGUI(ctx)
			checks = a.Doctor(ctx)
		} else if *saveReport != "" {
			err = AtomicWrite(*saveReport, []byte(a.SupportReport(checks)), 0600)
			if err == nil {
				fmt.Fprintln(out, "Saved privacy-safe support report.")
			}
		} else if *jsonOutput {
			encoder := json.NewEncoder(out)
			encoder.SetIndent("", "  ")
			err = encoder.Encode(checks)
		} else {
			fmt.Fprint(out, FormatChecks(checks))
		}
		if err == nil {
			for _, check := range checks {
				if !check.OK {
					return 1
				}
			}
			return 0
		}
	case "status":
		return outputStatus(out, a.Status(ctx), *jsonOutput)
	case "updates":
		if *gui {
			err = a.UpdatesGUI(ctx)
		} else {
			fmt.Fprintln(out, a.UpdatesSummary())
		}
	case "fix":
		if len(fs.Args()) != 1 {
			fmt.Fprintln(errOut, "usage: ptcgl-omarchy fix [--gui] <integration|setup|repair|runtime-update>")
			return 2
		}
		err = a.Fix(ctx, fs.Arg(0), *gui)
	default:
		fmt.Fprintf(errOut, "unknown command %q\n%s", command, help)
		return 2
	}
	if err != nil {
		return reportError(err)
	}
	return 0
}

func outputStatus(out io.Writer, status Status, jsonOutput bool) int {
	if jsonOutput {
		if err := json.NewEncoder(out).Encode(status); err != nil {
			return 1
		}
	} else {
		fmt.Fprintf(out, "Installation: %s\nActivity: %s\n%s\n", status.Readiness, status.Activity, status.Message)
	}
	// An unavailable component is data, not a malformed protocol response.
	return 0
}
