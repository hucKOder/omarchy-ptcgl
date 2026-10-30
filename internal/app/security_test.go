package app

import (
	"archive/tar"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveLinkGraphCannotEscapeOrCycle(t *testing.T) {
	for _, mode := range []string{"transitive escape", "dangling escape", "cycle", "valid chain", "valid dangling"} {
		t.Run(mode, func(t *testing.T) {
			base := t.TempDir()
			staging, outside := filepath.Join(base, "staging"), filepath.Join(base, "escape")
			for _, path := range []string{staging, outside} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			headers := []*tar.Header{
				{Name: "GE/files/deep/root", Typeflag: tar.TypeSymlink, Linkname: "../.."},
				{Name: "GE/files/escape", Typeflag: tar.TypeSymlink, Linkname: "deep/root/../../escape"},
			}
			switch mode {
			case "transitive escape":
				headers = append(headers, &tar.Header{Name: "GE/files/escape/created", Typeflag: tar.TypeSymlink, Linkname: "../target"})
			case "cycle":
				headers = []*tar.Header{{Name: "GE/a", Typeflag: tar.TypeSymlink, Linkname: "b"}, {Name: "GE/b", Typeflag: tar.TypeSymlink, Linkname: "a"}}
			case "valid chain", "valid dangling":
				headers = []*tar.Header{{Name: "GE/files/a", Typeflag: tar.TypeSymlink, Linkname: "b"}, {Name: "GE/files/b", Typeflag: tar.TypeSymlink, Linkname: "target"}}
				if mode == "valid chain" {
					headers = append(headers, &tar.Header{Name: "GE/files/target", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1})
				}
			}
			err := extractProton(context.Background(), tarFixture(t, headers), staging, "GE")
			if (err == nil) != strings.HasPrefix(mode, "valid ") {
				t.Fatalf("extraction error = %v", err)
			}
			entries, readErr := os.ReadDir(outside)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("extraction wrote outside staging: %v", readErr)
			}
			if mode == "valid chain" {
				data, err := os.ReadFile(filepath.Join(staging, "GE/files/a"))
				if err != nil || string(data) != "x" {
					t.Fatalf("valid relative link chain broken: %v", err)
				}
			}
		})
	}
}

func TestRuntimeCommandsExcludeInheritedCredentials(t *testing.T) {
	secrets := []string{"GH_TOKEN", "GITHUB_TOKEN", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "AZURE_CLIENT_SECRET", "CUSTOM_SERVICE_CREDENTIAL", "LC_API_KEY", "PROTON_API_KEY", "VK_API_KEY", "LD_PRELOAD", "LD_LIBRARY_PATH"}
	for _, name := range secrets {
		t.Setenv(name, "security-test-placeholder")
	}
	settings := map[string]string{
		"DISPLAY": ":1", "WAYLAND_DISPLAY": "wayland-1", "XDG_RUNTIME_DIR": "/run/user/1000",
		"XDG_DATA_HOME": "/custom/data", "XDG_CONFIG_HOME": "/custom/config", "XDG_CACHE_HOME": "/custom/cache",
		"XDG_STATE_HOME": "/custom/state", "DBUS_SESSION_BUS_ADDRESS": "unix:path=/run/user/1000/bus",
		"LC_CTYPE": "en_US.UTF-8", "VK_DRIVER_FILES": "/graphics/driver.json", "PULSE_SERVER": "unix:/audio/socket",
	}
	for name, value := range settings {
		t.Setenv(name, value)
	}
	a, runner := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	runner.run = func(c Command) error {
		if c.Name == "umu-run" && len(c.Args) > 1 && c.Args[1] == "-screen-fullscreen" {
			return a.Callback(context.Background(), "tpcitcgapp://callback?code=test&state=test")
		}
		return nil
	}
	if err := a.Play(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	// Include setup/maintenance command construction as well as actual Play and
	// Callback routing through the fake runner. No real UMU/game process is run.
	commands := append(runner.commands, a.umu(""), a.umu("winetricks", "-q", "vcrun2019"), a.umu("msiexec.exe", "/i", "test.msi"))
	for _, command := range commands {
		if command.Name != "umu-run" {
			continue
		}
		for _, secret := range secrets {
			for _, entry := range command.Env {
				if strings.HasPrefix(entry, secret+"=") {
					t.Errorf("runtime inherited %s", secret)
				}
			}
		}
		for name, value := range settings {
			if !contains(command.Env, name+"="+value) {
				t.Errorf("required setting %s was lost", name)
			}
		}
		if !contains(command.Env, "WINEPREFIX="+a.Paths.Prefix) || !contains(command.Env, "PROTONPATH="+a.protonPath()) {
			t.Error("managed prefix/runtime override was lost")
		}
	}
}
