package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

func (a *App) Doctor(ctx context.Context) []Check {
	var checks []Check
	add := func(name string, ok bool, detail, fix string) {
		if ok {
			fix = ""
		}
		checks = append(checks, Check{Name: name, OK: ok, Detail: detail, Fix: fix})
	}
	for _, dependency := range []struct{ name, fix string }{
		{"omarchy", "Run from your Omarchy desktop session with Omarchy's bin directory in PATH."},
		{"hyprctl", "Run from your Hyprland desktop session with hyprctl in PATH."},
		{"umu-run", "omarchy pkg add umu-launcher"},
		{"zenity", "omarchy pkg add zenity"},
		{"xdg-mime", "omarchy pkg add xdg-utils"},
	} {
		path, err := a.LookPath(dependency.name)
		if err != nil {
			path = err.Error()
		}
		add(dependency.name, err == nil, path, dependency.fix)
	}
	session := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != ""
	sessionDetail := "Hyprland session active; XWayland game window"
	if !session {
		sessionDetail = "HYPRLAND_INSTANCE_SIGNATURE is not set"
	}
	add("Hyprland session", session, sessionDetail, "Run ptcgl-omarchy doctor in a terminal inside your Omarchy desktop session.")

	proton := a.protonPath()
	protonErr := validateProton(proton)
	protonDetail, protonFix := proton, "ptcgl-omarchy setup"
	if protonErr != nil {
		protonDetail += ": " + protonErr.Error()
		if a.Config.ProtonPath != "" {
			protonFix = fmt.Sprintf("Correct proton_path in %q or repair the custom runtime at %q.", filepath.Join(a.Paths.Config, "config.json"), proton)
		} else if _, err := os.Stat(proton); !os.IsNotExist(err) {
			protonFix = fmt.Sprintf("Move the incomplete runtime at %q aside, then run: ptcgl-omarchy setup", proton)
		}
	}
	add("Proton", protonErr == nil, protonDetail, protonFix)

	for _, item := range []struct {
		name   string
		ready  bool
		detail string
		fix    string
	}{
		{"prefix", a.prefixReady(), a.Paths.Prefix + " (registry files and Windows Installer)", "ptcgl-omarchy setup"},
		{"Visual C++ runtime", a.vcReady(), "vcrun2019 DLLs in " + filepath.Join(a.Paths.Prefix, "drive_c/windows/system32"), "ptcgl-omarchy setup"},
	} {
		if !item.ready {
			item.detail += ": missing or incomplete"
		}
		add(item.name, item.ready, item.detail, item.fix)
	}
	for _, item := range []struct{ name, path, fix string }{
		{"game", a.gamePath(), "ptcgl-omarchy setup"},
		{"app-menu entry", filepath.Join(a.Paths.Applications, desktopID), "ptcgl-omarchy integrate"},
		{"callback entry", filepath.Join(a.Paths.Applications, callbackID), "ptcgl-omarchy integrate"},
	} {
		info, err := os.Stat(item.path)
		ok := err == nil && info.Mode().IsRegular()
		detail := item.path
		if err != nil {
			detail = err.Error()
		} else if !ok {
			detail += ": not a regular file"
		}
		add(item.name, ok, detail, item.fix)
	}

	b, err := a.Runner.Output(ctx, Command{Name: "xdg-mime", Args: []string{"query", "default", callbackMIME}})
	handler := strings.TrimSpace(string(b))
	callbackDetail := handler
	if err != nil {
		callbackDetail = fmt.Sprintf("xdg-mime query failed: %v", err)
		if handler != "" {
			callbackDetail += "; " + handler
		}
	} else if handler == "" {
		callbackDetail = "No default handler for " + callbackMIME
	} else if handler != callbackID {
		callbackDetail = fmt.Sprintf("Default handler is %s; expected %s", handler, callbackID)
	}
	add("browser callback", err == nil && handler == callbackID, callbackDetail, "ptcgl-omarchy integrate")

	mainPath := filepath.Join(a.Paths.Hypr, "hyprland.lua")
	rulePath := filepath.Join(a.Paths.Hypr, Name+".lua")
	main, err := os.ReadFile(mainPath)
	rulesOK, rulesDetail := false, "tiled; windowed startup; opaque"
	rulesFix := "ptcgl-omarchy integrate"
	switch {
	case err != nil:
		rulesDetail = err.Error()
		rulesFix = fmt.Sprintf("Restore a readable Omarchy Lua configuration at %q, then run: ptcgl-omarchy integrate", mainPath)
	case !strings.Contains(string(main), managedBlock(rulePath)):
		rulesDetail = "Missing managed loader block in " + mainPath
	default:
		if _, err := os.ReadFile(rulePath); err != nil {
			rulesDetail = err.Error()
		} else {
			rulesOK = true
		}
	}
	add("Hyprland rules", rulesOK, rulesDetail, rulesFix)
	if session {
		b, err := a.Runner.Output(ctx, Command{Name: "hyprctl", Args: []string{"configerrors"}})
		detail := strings.TrimSpace(string(b))
		ok := err == nil && detail == ""
		if err != nil {
			message := fmt.Sprintf("hyprctl configerrors failed: %v", err)
			if detail != "" {
				message += "; " + detail
			}
			detail = message
		} else if ok {
			detail = "No configuration errors"
		}
		add("Hyprland config", ok, detail, "Inspect hyprctl configerrors, fix the reported configuration, then run: hyprctl reload")
	}
	return checks
}

func FormatChecks(checks []Check) string {
	var b strings.Builder
	for _, c := range checks {
		status := "OK"
		if !c.OK {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "%-7s %-20s %s\n", status, c.Name, c.Detail)
		if !c.OK && c.Fix != "" {
			fmt.Fprintf(&b, "        Fix: %s\n", c.Fix)
		}
	}
	return b.String()
}
