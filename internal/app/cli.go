package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

const dialogTimeout = 5 * time.Minute

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
  doctor [--json]    Check runtime, installation, and desktop integration
  uninstall          Remove binary and integration; retain installed game data
  version            Print launcher version

setup, update, repair, runtime-update, and doctor accept --gui.
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
	p, err := UserPaths()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	a, err := New(p, out, errOut)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	// Callback URLs bypass flag parsing, preventing flag's error output from
	// accidentally repeating an authentication URL.
	if command == "callback" {
		if len(args) != 1 {
			fmt.Fprintln(errOut, ErrCallback)
			return 1
		}
		if err := a.Callback(ctx, args[0]); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		return 0
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	gui := fs.Bool("gui", false, "show progress or diagnostics dialog")
	checkOnly := fs.Bool("check", false, "compare the latest official installer without applying it")
	jsonOutput := fs.Bool("json", false, "print structured diagnostics")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if command != "play" && len(fs.Args()) > 0 {
		fmt.Fprintln(errOut, "unexpected positional arguments")
		return 2
	}
	if *checkOnly && command != "update" {
		fmt.Fprintln(errOut, "--check is only valid with update")
		return 2
	}
	if *jsonOutput && command != "doctor" {
		fmt.Fprintln(errOut, "--json is only valid with doctor")
		return 2
	}
	operation := func(fn func(context.Context) error) error {
		if *gui {
			return a.WithProgress(ctx, fn)
		}
		return fn(ctx)
	}
	switch command {
	case "setup":
		err = operation(a.Setup)
	case "play":
		err = a.Play(ctx, fs.Args())
	case "app":
		err = a.Desktop(ctx)
	case "update":
		err = operation(func(ctx context.Context) error { return a.Update(ctx, false, *checkOnly) })
	case "repair":
		err = operation(func(ctx context.Context) error { return a.Update(ctx, true, false) })
	case "runtime-update":
		err = operation(a.UpdateRuntime)
	case "install":
		err = a.InstallSelf(ctx)
	case "integrate":
		err = a.Integrate(ctx)
	case "uninstall":
		err = a.Uninstall(ctx)
	case "doctor":
		checks := a.Doctor(ctx)
		if *gui {
			err = ShowReport(ctx, FormatChecks(checks))
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
	default:
		fmt.Fprintf(errOut, "unknown command %q\n%s", command, help)
		return 2
	}
	if err != nil {
		message := strings.TrimSpace(err.Error())
		fmt.Fprintln(errOut, message)
		if *gui || command == "app" {
			ShowError(message)
		}
		return 1
	}
	return 0
}
