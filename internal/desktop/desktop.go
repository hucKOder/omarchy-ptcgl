// Package desktop renders desktop entries and managed integration fragments.
// It does not write files or invoke desktop commands.
package desktop

import (
	_ "embed"
	"fmt"
	"strings"
)

const (
	DesktopID    = "ptcgl-omarchy.desktop"
	CallbackID   = "ptcgl-omarchy-callback.desktop"
	CallbackMIME = "x-scheme-handler/tpcitcgapp"
	BlockBegin   = "-- BEGIN ptcgl-omarchy"
	BlockEnd     = "-- END ptcgl-omarchy"
)

//go:embed assets/ptcgl-omarchy.desktop.in
var desktopTemplate string

//go:embed assets/ptcgl-omarchy-callback.desktop.in
var callbackTemplate string

// QuoteExec quotes a literal executable path for a desktop Exec field.
func QuoteExec(s string) string {
	// Desktop Exec fields are not shell commands; literal % needs doubling.
	s = strings.ReplaceAll(s, "%", "%%")
	for _, char := range []string{`\`, `"`, "`", "$"} {
		s = strings.ReplaceAll(s, char, `\`+char)
	}
	// Backslashes are also escaped in the desktop-file string layer.
	s = strings.ReplaceAll(s, `\`, `\\`)
	return `"` + s + `"`
}

// QuoteLua quotes a string literal in a Hyprland Lua configuration.
func QuoteLua(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return `"` + s + `"`
}

// Files renders the desktop entries for the installed launcher path.
func Files(executable string) map[string][]byte {
	exe := QuoteExec(executable)
	return map[string][]byte{
		DesktopID:  []byte(strings.ReplaceAll(desktopTemplate, "@EXEC@", exe)),
		CallbackID: []byte(strings.ReplaceAll(callbackTemplate, "@EXEC@", exe)),
	}
}

// ManagedBlock renders the managed Hyprland include for path.
func ManagedBlock(path string) string {
	return BlockBegin + "\ndofile(" + QuoteLua(path) + ")\n" + BlockEnd + "\n"
}

// RemoveBlock removes a single managed include, rejecting ambiguous blocks.
func RemoveBlock(source string) (string, error) {
	start := strings.Index(source, BlockBegin)
	if start < 0 {
		if strings.Contains(source, BlockEnd) {
			return "", fmt.Errorf("incomplete ptcgl-omarchy config block")
		}
		return source, nil
	}
	end := strings.Index(source[start:], BlockEnd)
	if end < 0 || strings.Contains(source[start+len(BlockBegin):], BlockBegin) {
		return "", fmt.Errorf("ambiguous ptcgl-omarchy config block")
	}
	end += start + len(BlockEnd)
	if end < len(source) && source[end] == '\n' {
		end++
	}
	return source[:start] + source[end:], nil
}

// RemoveMIMEReference removes only this launcher's callback handler.
func RemoveMIMEReference(source string) string {
	var result []string
	for _, line := range strings.Split(source, "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found || strings.TrimSpace(key) != CallbackMIME {
			result = append(result, line)
			continue
		}
		var kept []string
		removed := false
		for _, id := range strings.Split(value, ";") {
			if strings.TrimSpace(id) == CallbackID {
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
