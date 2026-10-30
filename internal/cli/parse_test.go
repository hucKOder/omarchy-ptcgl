package cli

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
		args    []string
		want    Options
		proceed bool
		code    int
		message string
	}{
		{name: "play arguments", command: "play", args: []string{"--gui", "--", "--screen-width", "900"}, want: Options{GUI: true, Args: []string{"--screen-width", "900"}}, proceed: true},
		{name: "update check", command: "update", args: []string{"--check"}, want: Options{Check: true}, proceed: true},
		{name: "status json", command: "status", args: []string{"--json"}, want: Options{JSON: true}, proceed: true},
		{name: "invalid json", command: "setup", args: []string{"--json"}, want: Options{JSON: true}, code: 2, message: "--json is only valid"},
		{name: "invalid check", command: "repair", args: []string{"--check"}, want: Options{Check: true}, code: 2, message: "--check is only valid"},
		{name: "invalid save", command: "doctor", args: []string{"--save", "report", "--gui"}, want: Options{GUI: true, Save: "report"}, code: 2, message: "--save is only valid"},
		{name: "positional argument", command: "setup", args: []string{"unexpected"}, want: Options{Args: []string{"unexpected"}}, code: 2, message: "unexpected positional arguments"},
		{name: "help", command: "setup", args: []string{"--help"}, message: "Usage of setup:"},
		{name: "unknown flag", command: "setup", args: []string{"--unknown"}, code: 2, message: "flag provided but not defined"},
		{name: "callback bypasses flags", command: "callback", args: []string{"--gui=SECRET"}, proceed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			got, proceed, code := Parse(tc.command, tc.args, &output)
			if got.GUI != tc.want.GUI || got.Check != tc.want.Check || got.JSON != tc.want.JSON || got.Save != tc.want.Save || !slices.Equal(got.Args, tc.want.Args) || proceed != tc.proceed || code != tc.code {
				t.Fatalf("got %+v, proceed=%t, code=%d; want %+v, %t, %d", got, proceed, code, tc.want, tc.proceed, tc.code)
			}
			if !strings.Contains(output.String(), tc.message) || strings.Contains(output.String(), "SECRET") {
				t.Fatalf("unexpected output %q", output.String())
			}
		})
	}
}
