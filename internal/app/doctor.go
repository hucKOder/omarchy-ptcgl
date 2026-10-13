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
}

func (a *App) Doctor(ctx context.Context) []Check {
	var checks []Check
	for _, name := range []string{"omarchy", "hyprctl", "umu-run", "osslsigncode", "zenity", "xdg-mime"} {
		path, err := a.LookPath(name)
		if err != nil {
			path = "missing"
		}
		checks = append(checks, Check{name, err == nil, path})
	}
	for _, item := range []struct {
		name   string
		ready  bool
		detail string
	}{
		{"Hyprland session", os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "", "XWayland game window"},
		{"Proton", validateProton(a.protonPath()) == nil, a.protonPath()},
		{"prefix", a.prefixReady(), a.Paths.Prefix},
		{"Visual C++ runtime", a.vcReady(), "vcrun2019"},
		{"game", exists(a.gamePath()), a.gamePath()},
		{"app-menu entry", exists(filepath.Join(a.Paths.Applications, desktopID)), desktopID},
	} {
		checks = append(checks, Check{item.name, item.ready, item.detail})
	}
	b, err := a.Runner.Output(ctx, Command{Name: "xdg-mime", Args: []string{"query", "default", callbackMIME}})
	handler := strings.TrimSpace(string(b))
	checks = append(checks, Check{"browser callback", err == nil && handler == callbackID, handler})
	main, _ := os.ReadFile(filepath.Join(a.Paths.Hypr, "hyprland.lua"))
	checks = append(checks, Check{"Hyprland rules", strings.Contains(string(main), managedBlock(filepath.Join(a.Paths.Hypr, Name+".lua"))) && exists(filepath.Join(a.Paths.Hypr, Name+".lua")), "tiled; windowed startup; opaque"})
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" {
		b, err := a.Runner.Output(ctx, Command{Name: "hyprctl", Args: []string{"configerrors"}})
		checks = append(checks, Check{"Hyprland config", err == nil && strings.TrimSpace(string(b)) == "", strings.TrimSpace(string(b))})
	}
	return checks
}

func FormatChecks(checks []Check) string {
	var b strings.Builder
	for _, c := range checks {
		status := "OK"
		if !c.OK {
			status = "MISSING"
		}
		fmt.Fprintf(&b, "%-7s %-20s %s\n", status, c.Name, c.Detail)
	}
	return b.String()
}
