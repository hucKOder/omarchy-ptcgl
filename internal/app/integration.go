package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const desktopID = "ptcgl-omarchy.desktop"
const callbackID = "ptcgl-omarchy-callback.desktop"
const callbackMIME = "x-scheme-handler/tpcitcgapp"
const blockBegin = "-- BEGIN ptcgl-omarchy"
const blockEnd = "-- END ptcgl-omarchy"

type Integration struct {
	PreviousHandler string `json:"previous_handler"`
	HadHandler      bool   `json:"had_handler"`
}

func desktopQuote(s string) string {
	// Desktop Exec fields are not shell commands; literal % needs doubling.
	s = strings.ReplaceAll(s, "%", "%%")
	for _, char := range []string{`\`, `"`, "`", "$"} {
		s = strings.ReplaceAll(s, char, `\`+char)
	}
	// Backslashes are also escaped in the desktop-file string layer.
	s = strings.ReplaceAll(s, `\`, `\\`)
	return `"` + s + `"`
}

func luaQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return `"` + s + `"`
}

func managedBlock(path string) string {
	return blockBegin + "\ndofile(" + luaQuote(path) + ")\n" + blockEnd + "\n"
}

func removeBlock(source string) (string, error) {
	start := strings.Index(source, blockBegin)
	if start < 0 {
		if strings.Contains(source, blockEnd) {
			return "", fmt.Errorf("incomplete ptcgl-omarchy config block")
		}
		return source, nil
	}
	end := strings.Index(source[start:], blockEnd)
	if end < 0 || strings.Contains(source[start+len(blockBegin):], blockBegin) {
		return "", fmt.Errorf("ambiguous ptcgl-omarchy config block")
	}
	end += start + len(blockEnd)
	if end < len(source) && source[end] == '\n' {
		end++
	}
	return source[:start] + source[end:], nil
}

func (a *App) desktopFiles() map[string][]byte {
	exe := desktopQuote(a.Paths.Bin)
	return map[string][]byte{
		desktopID:  []byte("[Desktop Entry]\nType=Application\nName=Pokémon TCG Live (Omarchy)\nComment=Play Pokémon TCG Live in a tiled Hyprland window\nExec=" + exe + " app\nIcon=ptcgl-omarchy\nTerminal=false\nCategories=Game;CardGame;\nStartupNotify=false\nActions=Update;Doctor;\n\n[Desktop Action Update]\nName=Update game\nExec=" + exe + " update --gui\n\n[Desktop Action Doctor]\nName=Diagnostics\nExec=" + exe + " doctor --gui\n"),
		callbackID: []byte("[Desktop Entry]\nType=Application\nName=Pokémon TCG Live authentication\nExec=" + exe + " callback %u\nNoDisplay=true\nTerminal=false\nMimeType=" + callbackMIME + ";\n"),
	}
}

func (a *App) integrationState() (Integration, error) {
	var state Integration
	b, err := os.ReadFile(filepath.Join(a.Paths.State, "integration.json"))
	if err != nil {
		return state, err
	}
	return state, json.Unmarshal(b, &state)
}

// InstallSelf copies the executable atomically before registering desktop files.
func (a *App) InstallSelf(ctx context.Context) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("install as your desktop user, without sudo")
	}
	return a.withLock(func() error {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		b, err := os.ReadFile(exe)
		if err != nil {
			return err
		}
		if err := AtomicWrite(a.Paths.Bin, b, 0755); err != nil {
			return err
		}
		return a.integrate(ctx)
	})
}

func (a *App) Integrate(ctx context.Context) error {
	return a.withLock(func() error { return a.integrate(ctx) })
}

func (a *App) integrate(ctx context.Context) error {
	if err := a.require("omarchy", "xdg-mime"); err != nil {
		return err
	}
	if !exists(a.Paths.Bin) {
		return fmt.Errorf("install the executable first with the installer or make install-launcher")
	}
	main := filepath.Join(a.Paths.Hypr, "hyprland.lua")
	original, err := os.ReadFile(main)
	if err != nil {
		return fmt.Errorf("Omarchy Lua config required at %s", main)
	}
	clean, err := removeBlock(string(original))
	if err != nil {
		return err
	}
	rule := filepath.Join(a.Paths.Hypr, Name+".lua")
	rules, _ := assets.ReadFile("assets/ptcgl-omarchy.lua")
	oldRule, oldRuleErr := os.ReadFile(rule)
	if oldRuleErr == nil && !strings.Contains(string(oldRule), "Managed by ptcgl-omarchy") {
		return fmt.Errorf("%s already exists and is not managed by this tool", rule)
	}
	updated := strings.TrimRight(clean, "\n") + "\n\n" + managedBlock(rule)
	if err := AtomicWrite(rule, rules, 0644); err != nil {
		return err
	}
	if string(original) != updated {
		backup := main + ".bak.ptcgl-omarchy." + time.Now().UTC().Format("20060102T150405.000000000")
		if err := AtomicWrite(backup, original, 0600); err != nil {
			return err
		}
		if err := AtomicWrite(main, []byte(updated), 0644); err != nil {
			return err
		}
	}
	if err := a.validateHypr(ctx); err != nil {
		_ = AtomicWrite(main, original, 0644)
		if oldRuleErr == nil {
			_ = AtomicWrite(rule, oldRule, 0644)
		} else {
			_ = os.Remove(rule)
		}
		_ = a.validateHypr(ctx)
		return fmt.Errorf("Hyprland integration rolled back: %w", err)
	}
	state, err := a.integrationState()
	if os.IsNotExist(err) {
		b, queryErr := a.Runner.Output(ctx, Command{Name: "xdg-mime", Args: []string{"query", "default", callbackMIME}})
		if queryErr != nil {
			return fmt.Errorf("query browser callback handler: %w", queryErr)
		}
		state.PreviousHandler = strings.TrimSpace(string(b))
		state.HadHandler = state.PreviousHandler != ""
		if state.PreviousHandler == callbackID {
			state.PreviousHandler = ""
			state.HadHandler = false
		}
		if err := writeJSON(filepath.Join(a.Paths.State, "integration.json"), state); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	for name, content := range a.desktopFiles() {
		path := filepath.Join(a.Paths.Applications, name)
		if err := AtomicWrite(path, content, 0644); err != nil {
			return err
		}
		if _, err := a.LookPath("desktop-file-validate"); err == nil {
			if err := a.Runner.Run(ctx, Command{Name: "desktop-file-validate", Args: []string{path}}); err != nil {
				return err
			}
		}
	}
	icon, _ := assets.ReadFile("assets/ptcgl-omarchy.svg")
	iconPath := filepath.Join(filepath.Dir(a.Paths.Data), "icons/hicolor/scalable/apps/ptcgl-omarchy.svg")
	if err := AtomicWrite(iconPath, icon, 0644); err != nil {
		return err
	}
	if _, err := a.LookPath("update-desktop-database"); err == nil {
		if err := a.Runner.Run(ctx, Command{Name: "update-desktop-database", Args: []string{a.Paths.Applications}}); err != nil {
			return err
		}
	}
	if err := a.Runner.Run(ctx, Command{Name: "xdg-mime", Args: []string{"default", callbackID, callbackMIME}}); err != nil {
		return err
	}
	a.progress("Installed app-menu entry, browser callback, and tiled Hyprland rules.")
	return nil
}

func (a *App) validateHypr(ctx context.Context) error {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := a.Runner.Run(ctx, Command{Name: "hyprctl", Args: []string{"reload"}, Quiet: true}); err != nil {
		return err
	}
	b, err := a.Runner.Output(ctx, Command{Name: "hyprctl", Args: []string{"configerrors"}})
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(b)) != "" {
		return fmt.Errorf("%s", strings.TrimSpace(string(b)))
	}
	return nil
}

func (a *App) Uninstall(ctx context.Context) error {
	return a.withLock(func() error {
		state, err := a.integrationState()
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		b, queryErr := a.Runner.Output(ctx, Command{Name: "xdg-mime", Args: []string{"query", "default", callbackMIME}})
		if queryErr != nil {
			return queryErr
		}
		if strings.TrimSpace(string(b)) == callbackID && state.HadHandler {
			if err := a.Runner.Run(ctx, Command{Name: "xdg-mime", Args: []string{"default", state.PreviousHandler, callbackMIME}}); err != nil {
				return err
			}
		}
		// xdg-mime has no unset command. Remove only references to our desktop
		// file, retaining every other handler and unrelated MIME preference.
		for _, path := range []string{
			filepath.Join(filepath.Dir(a.Paths.Config), "mimeapps.list"),
			filepath.Join(a.Paths.Applications, "mimeapps.list"),
		} {
			b, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			clean := removeMIMEReference(string(b))
			if clean != string(b) {
				if err := AtomicWrite(path, []byte(clean), 0600); err != nil {
					return err
				}
			}
		}
		main := filepath.Join(a.Paths.Hypr, "hyprland.lua")
		original, err := os.ReadFile(main)
		if err != nil {
			return err
		}
		clean, err := removeBlock(string(original))
		if err != nil {
			return err
		}
		if clean != string(original) {
			backup := main + ".bak.ptcgl-omarchy." + time.Now().UTC().Format("20060102T150405.000000000")
			if err := AtomicWrite(backup, original, 0600); err != nil {
				return err
			}
			if err := AtomicWrite(main, []byte(clean), 0644); err != nil {
				return err
			}
		}
		if err := a.validateHypr(ctx); err != nil {
			_ = AtomicWrite(main, original, 0644)
			_ = a.validateHypr(ctx)
			return err
		}
		for _, path := range []string{
			filepath.Join(a.Paths.Hypr, Name+".lua"), filepath.Join(a.Paths.Applications, desktopID), filepath.Join(a.Paths.Applications, callbackID),
			filepath.Join(filepath.Dir(a.Paths.Data), "icons/hicolor/scalable/apps/ptcgl-omarchy.svg"),
			filepath.Join(a.Paths.State, "integration.json"), a.Paths.Bin,
		} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		if _, err := a.LookPath("update-desktop-database"); err == nil {
			if err := a.Runner.Run(ctx, Command{Name: "update-desktop-database", Args: []string{a.Paths.Applications}}); err != nil {
				return err
			}
		}
		a.progress("Removed launcher integration. Game data is available at " + a.Paths.Data)
		return nil
	})
}

func removeMIMEReference(source string) string {
	var result []string
	for _, line := range strings.Split(source, "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found || strings.TrimSpace(key) != callbackMIME {
			result = append(result, line)
			continue
		}
		var kept []string
		removed := false
		for _, id := range strings.Split(value, ";") {
			if strings.TrimSpace(id) == callbackID {
				removed = true
			} else if id != "" {
				kept = append(kept, id)
			}
		}
		if !removed {
			result = append(result, line)
			continue
		}
		if len(kept) > 0 {
			result = append(result, key+"="+strings.Join(kept, ";")+";")
		}
	}
	return strings.Join(result, "\n")
}
