package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"ptcgl-omarchy/internal/cli"
	"ptcgl-omarchy/internal/config"
	"ptcgl-omarchy/internal/verify"
)

func RunCLI(ctx context.Context, args []string, out, errOut io.Writer) int {
	command := "app"
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}
	if command == "help" || command == "--help" || command == "-h" {
		fmt.Fprint(out, cli.Help)
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
		if err := verify.MSI(ctx, args[0]); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		return 0
	}
	opts, proceed, code := cli.Parse(command, args, errOut)
	if !proceed {
		return code
	}
	reportError := func(err error) int {
		message := strings.TrimSpace(err.Error())
		fmt.Fprintln(errOut, message)
		if opts.GUI || command == "app" {
			ShowError(message)
		}
		return 1
	}
	if command == "callback" && len(args) != 1 {
		return reportError(ErrCallback)
	}
	p, err := config.UserPaths()
	if err != nil {
		if command == "status" {
			return outputStatus(out, unavailableStatus(), opts.JSON)
		}
		return reportError(err)
	}
	var a *App
	if command == "open" {
		// Folder access also works when configuration/runtime state needs repair.
		a = &App{Paths: p, Runner: OSRunner{Out: out, Err: errOut}, LookPath: exec.LookPath}
	} else {
		a, err = New(p, out, errOut)
		if err != nil {
			if command == "status" {
				return outputStatus(out, unavailableStatus(), opts.JSON)
			}
			return reportError(err)
		}
	}
	operation := func(fn func(context.Context) error) error {
		if opts.GUI {
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
		err = a.play(ctx, opts.Args, opts.GUI)
	case "app":
		err = a.Desktop(ctx)
	case "update", "repair":
		repair := command == "repair"
		if opts.GUI {
			err = a.UpdateGUI(ctx, repair, opts.Check)
		} else {
			err = a.Update(ctx, repair, opts.Check)
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
		if len(opts.Args) > 1 || (len(opts.Args) == 0 && !opts.GUI) {
			fmt.Fprintln(errOut, "usage: ptcgl-omarchy open <game|config|cache> or ptcgl-omarchy open --gui")
			return 2
		}
		if len(opts.Args) == 0 {
			err = a.ChooseFolder(ctx)
		} else {
			err = a.OpenFolder(ctx, opts.Args[0])
		}
	case "doctor":
		checks := a.Doctor(ctx)
		if opts.GUI {
			err = a.DiagnosticsGUI(ctx)
			checks = a.Doctor(ctx)
		} else if opts.Save != "" {
			err = AtomicWrite(opts.Save, []byte(a.SupportReport(checks)), 0o600)
			if err == nil {
				fmt.Fprintln(out, "Saved privacy-safe support report.")
			}
		} else if opts.JSON {
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
		return outputStatus(out, a.Status(ctx), opts.JSON)
	case "updates":
		if opts.GUI {
			err = a.UpdatesGUI(ctx)
		} else {
			fmt.Fprintln(out, a.UpdatesSummary())
		}
	case "fix":
		if len(opts.Args) != 1 {
			fmt.Fprintln(errOut, "usage: ptcgl-omarchy fix [--gui] <integration|setup|repair|runtime-update>")
			return 2
		}
		err = a.Fix(ctx, opts.Args[0], opts.GUI)
	default:
		fmt.Fprintf(errOut, "unknown command %q\n%s", command, cli.Help)
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
