// Package cli parses launcher command-line options without initializing the app.
package cli

import (
	"flag"
	"fmt"
	"io"
)

const Help = `ptcgl-omarchy — Pokémon TCG Live for Omarchy

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

// Options contains parsed flags and positional arguments for one command.
type Options struct {
	GUI, Check, JSON bool
	Save             string
	Args             []string
}

// Parse returns whether execution should continue and the exit code otherwise.
// Callback arguments bypass flag parsing so credentials never enter its output.
func Parse(command string, args []string, errOut io.Writer) (Options, bool, int) {
	var opts Options
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.BoolVar(&opts.GUI, "gui", false, "show graphical dialogs")
	fs.BoolVar(&opts.Check, "check", false, "compare the latest official installer without applying it")
	fs.BoolVar(&opts.JSON, "json", false, "print structured diagnostics")
	fs.StringVar(&opts.Save, "save", "", "save a privacy-safe support report (doctor only)")
	if command != "callback" {
		if err := fs.Parse(args); err != nil {
			if err == flag.ErrHelp {
				return opts, false, 0
			}
			return opts, false, 2
		}
	}
	opts.Args = fs.Args()
	if command != "play" && command != "open" && command != "fix" && len(opts.Args) > 0 {
		fmt.Fprintln(errOut, "unexpected positional arguments")
		return opts, false, 2
	}
	if opts.Check && command != "update" {
		fmt.Fprintln(errOut, "--check is only valid with update")
		return opts, false, 2
	}
	if opts.JSON && command != "doctor" && command != "status" {
		fmt.Fprintln(errOut, "--json is only valid with doctor or status")
		return opts, false, 2
	}
	if opts.Save != "" && (command != "doctor" || opts.JSON || opts.GUI) {
		fmt.Fprintln(errOut, "--save is only valid with terminal doctor, without --json")
		return opts, false, 2
	}
	return opts, true, 0
}
