package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLICommandContracts(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		code int
		out  string
		err  string
	}{
		{name: "help", args: []string{"--help"}, out: "Usage: ptcgl-omarchy"},
		{name: "version", args: []string{"--version"}, out: Name + " " + Version},
		{name: "invalid flag", args: []string{"setup", "--json"}, code: 2, err: "--json is only valid"},
		{name: "unexpected argument", args: []string{"setup", "unexpected"}, code: 2, err: "unexpected positional arguments"},
		{name: "verification worker arguments", args: []string{"__verify-installer"}, code: 2, err: "requires one MSI path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := RunCLI(context.Background(), tc.args, &out, &errOut); code != tc.code {
				t.Fatalf("exit code %d, want %d; stderr: %s", code, tc.code, &errOut)
			}
			if !strings.Contains(out.String(), tc.out) || !strings.Contains(errOut.String(), tc.err) {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", &out, &errOut)
			}
		})
	}
}

func TestGraphicalInitializationErrors(t *testing.T) {
	for _, state := range []string{"config.json", "runtime.json"} {
		for _, args := range [][]string{nil, {"doctor", "--gui"}, {"update", "--check", "--gui"}, {"repair", "--gui"}} {
			t.Run(state+"/"+strings.Join(args, " "), func(t *testing.T) {
				a, _ := testApp(t)
				useTestPaths(t, a.Paths)
				log := stubZenity(t)
				dir := a.Paths.Config
				if state == "runtime.json" {
					dir = a.Paths.State
				}
				writeTestFile(t, filepath.Join(dir, state), []byte("{broken JSON"))
				var out, errOut bytes.Buffer
				if code := RunCLI(context.Background(), args, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), state) {
					t.Fatalf("initialization failure not reported: code=%d, stderr=%s", code, &errOut)
				}
				dialogs := readDialogLog(t, log)
				if strings.Count(dialogs, "BEGIN --error") != 1 || !strings.Contains(dialogs, state) || strings.Contains(dialogs, "BEGIN --progress") {
					t.Fatalf("initialization failure did not show a single error dialog:\n%s", dialogs)
				}
			})
		}
	}
}

func TestTerminalInitializationErrorDoesNotOpenDialog(t *testing.T) {
	a, _ := testApp(t)
	useTestPaths(t, a.Paths)
	log := stubZenity(t)
	writeTestFile(t, filepath.Join(a.Paths.Config, "config.json"), []byte("{broken JSON"))
	var out, errOut bytes.Buffer
	if code := RunCLI(context.Background(), []string{"doctor"}, &out, &errOut); code != 1 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("terminal diagnostics opened a dialog")
	}
}

func TestCLICallbackArgumentsStayOutOfDiagnostics(t *testing.T) {
	for _, args := range [][]string{
		{"callback", "--gui=SECRET"},
		{"callback", "--gui", "tpcitcgapp://callback?code=SECRET"},
		{"callback", "https://callback?code=SECRET"},
	} {
		t.Run(strings.Join(args[:2], " "), func(t *testing.T) {
			a, _ := testApp(t)
			useTestPaths(t, a.Paths)
			log := stubZenity(t)
			var out, errOut bytes.Buffer
			if code := RunCLI(context.Background(), args, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), ErrCallback.Error()) {
				t.Fatalf("unexpected callback failure: code=%d, stderr=%s", code, &errOut)
			}
			if strings.Contains(out.String()+errOut.String(), "SECRET") {
				t.Fatal("callback arguments appeared in CLI output")
			}
			if _, err := os.Stat(log); !os.IsNotExist(err) {
				t.Fatal("callback arguments enabled a graphical dialog")
			}
		})
	}
}
