package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func folderApp(t *testing.T) (*App, *fakeRunner) {
	t.Helper()
	a, r := testApp(t)
	for _, path := range []string{filepath.Dir(a.gamePath()), a.Paths.Config, a.Paths.Cache} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return a, r
}

func TestOpenFoldersDuringActiveSession(t *testing.T) {
	a, r := folderApp(t)
	lock, err := acquireLock(a.Paths)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseLock(lock)
	marker := filepath.Join(a.Paths.State, "play-session.json")
	writeTestFile(t, marker, []byte("active session"))
	for target, path := range map[string]string{"game": filepath.Dir(a.gamePath()), "config": a.Paths.Config, "cache": a.Paths.Cache} {
		if err := a.OpenFolder(context.Background(), target); err != nil {
			t.Fatal(err)
		}
		c := r.commands[len(r.commands)-1]
		if c.Name != "xdg-open" || len(c.Args) != 1 || c.Args[0] != path {
			t.Fatalf("folder path was not passed intact to the file manager: %+v", c)
		}
	}
	if len(r.commands) != 3 {
		t.Fatalf("folder access ran extra commands: %+v", r.commands)
	}
	b, _ := os.ReadFile(marker)
	if string(b) != "active session" {
		t.Fatal("folder access changed the active game session")
	}
}

func TestMissingFoldersAreExplainedWithoutCreatingThem(t *testing.T) {
	for _, tc := range []struct{ target, hint string }{
		{"game", "ptcgl-omarchy setup"},
		{"config", "config.json"},
		{"cache", "downloads files"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			a, r := testApp(t)
			err := a.OpenFolder(context.Background(), tc.target)
			if err == nil || !strings.Contains(err.Error(), "does not exist") || !strings.Contains(err.Error(), tc.hint) {
				t.Fatalf("missing folder was not explained: %v", err)
			}
			if len(r.commands) != 0 {
				t.Fatal("missing folder opened a file manager")
			}
			if _, err := os.Stat(a.Paths.Home); !os.IsNotExist(err) {
				t.Fatal("missing folder access created application files")
			}
		})
	}
}

func TestOpenFolderRejectsInvalidTargetsAndReportsErrors(t *testing.T) {
	for _, mode := range []string{"unknown", "not directory", "missing opener", "failed opener"} {
		t.Run(mode, func(t *testing.T) {
			a, r := folderApp(t)
			target, want := "config", ""
			switch mode {
			case "unknown":
				target, want = "../other", "choose game, config, or cache"
			case "not directory":
				if err := os.Remove(a.Paths.Config); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, a.Paths.Config, []byte("not a directory"))
				want = "not a directory"
			case "missing opener":
				a.LookPath = func(string) (string, error) { return "", errors.New("not found") }
				want = "omarchy pkg add xdg-utils"
			case "failed opener":
				r.run = func(Command) error { return errors.New("file manager failed") }
				want = "file manager failed"
			}
			err := a.OpenFolder(context.Background(), target)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != "failed opener" && len(r.commands) != 0 {
				t.Fatal("invalid folder request invoked an opener")
			}
		})
	}
}

func TestFolderChooserSelectionAndCancellation(t *testing.T) {
	for _, selection := range []string{"game", "config", "cache", "", "cancel", "error", "invalid"} {
		t.Run("selection="+selection, func(t *testing.T) {
			a, r := folderApp(t)
			log := stubZenity(t)
			t.Setenv("TEST_FOLDER_SELECTION", selection)
			// A successful chooser may emit GTK warnings on stderr.
			t.Setenv("TEST_FOLDER_STDERR", "GTK diagnostic\n")
			switch selection {
			case "cancel":
				t.Setenv("TEST_FOLDER_EXIT", "1")
			case "error":
				t.Setenv("TEST_FOLDER_EXIT", "2")
			}
			err := a.ChooseFolder(context.Background())
			if (err != nil) != (selection == "error" || selection == "invalid") {
				t.Fatalf("unexpected chooser result: %v", err)
			}
			path := map[string]string{"game": filepath.Dir(a.gamePath()), "config": a.Paths.Config, "cache": a.Paths.Cache}[selection]
			if path == "" {
				if len(r.commands) != 0 {
					t.Fatal("cancelled, empty, or invalid selection opened a folder")
				}
			} else if len(r.commands) != 1 || r.commands[0].Name != "xdg-open" || len(r.commands[0].Args) != 1 || r.commands[0].Args[0] != path {
				t.Fatalf("wrong folder opened: %+v", r.commands)
			}
			if dialogs := readDialogLog(t, log); !strings.Contains(dialogs, a.Paths.Config) || !strings.Contains(dialogs, "--print-column=1") {
				t.Fatalf("chooser lost folder locations or selection IDs:\n%s", dialogs)
			}
		})
	}
}

func TestFolderCLIUsesXDGPathsWithBrokenConfiguration(t *testing.T) {
	a, _ := folderApp(t)
	dialogLog := stubZenity(t)
	useTestPaths(t, a.Paths)
	writeTestFile(t, filepath.Join(a.Paths.Config, "config.json"), []byte("{broken config"))
	writeTestFile(t, filepath.Join(a.Paths.State, "runtime.json"), []byte("{broken runtime"))
	openerLog := filepath.Join(filepath.Dir(dialogLog), "opened.log")
	t.Setenv("TEST_OPEN_LOG", openerLog)
	opener := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$TEST_OPEN_LOG\"\n"
	if err := os.WriteFile(filepath.Join(filepath.Dir(dialogLog), "xdg-open"), []byte(opener), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		path string
	}{
		{[]string{"open", "game"}, filepath.Dir(a.gamePath())},
		{[]string{"open", "config"}, a.Paths.Config},
		{[]string{"open", "cache"}, a.Paths.Cache},
		{[]string{"open", "--gui"}, a.Paths.Config},
	} {
		t.Setenv("TEST_FOLDER_SELECTION", "config")
		var out, errOut bytes.Buffer
		if code := RunCLI(context.Background(), tc.args, &out, &errOut); code != 0 {
			t.Fatalf("folder command failed: code=%d, stderr=%s", code, &errOut)
		}
		b, err := os.ReadFile(openerLog)
		if err != nil || string(b) != tc.path+"\n" {
			t.Fatalf("wrong XDG folder opened: %q, err=%v", b, err)
		}
		if err := os.Remove(openerLog); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"open"}, {"open", "game", "config"}, {"open", "--check", "cache"}} {
		var out, errOut bytes.Buffer
		if code := RunCLI(context.Background(), args, &out, &errOut); code != 2 {
			t.Fatalf("invalid folder command accepted: %v, code=%d, stderr=%s", args, code, &errOut)
		}
		if _, err := os.Stat(openerLog); !os.IsNotExist(err) {
			t.Fatal("invalid CLI arguments opened a folder")
		}
	}
}
