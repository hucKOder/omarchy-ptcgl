package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	commands []Command
	run      func(Command) error
	output   func(Command) ([]byte, error)
}

func (r *fakeRunner) Run(_ context.Context, c Command) error {
	r.commands = append(r.commands, c)
	if r.run != nil {
		return r.run(c)
	}
	return nil
}
func (r *fakeRunner) Output(_ context.Context, c Command) ([]byte, error) {
	r.commands = append(r.commands, c)
	if r.output != nil {
		return r.output(c)
	}
	return nil, nil
}

func (r *fakeRunner) Start(c Command) error { return r.Run(context.Background(), c) }

func testApp(t *testing.T) (*App, *fakeRunner) {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home with spaces and é")
	p := Paths{Home: home, Data: filepath.Join(home, "data", Name), Cache: filepath.Join(home, "cache", Name),
		Config: filepath.Join(home, "config", Name), State: filepath.Join(home, "state", Name),
		Prefix: filepath.Join(home, "data", Name, "prefix"), Bin: filepath.Join(home, "bin", Name),
		Applications: filepath.Join(home, "data", "applications"), Hypr: filepath.Join(home, "config", "hypr")}
	a, err := New(p, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{}
	a.Runner = r
	a.LookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	return a, r
}

func useTestPaths(t *testing.T, p Paths) {
	t.Helper()
	t.Setenv("HOME", p.Home)
	t.Setenv("XDG_DATA_HOME", filepath.Dir(p.Data))
	t.Setenv("XDG_CACHE_HOME", filepath.Dir(p.Cache))
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(p.Config))
	t.Setenv("XDG_STATE_HOME", filepath.Dir(p.State))
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := AtomicWrite(path, content, 0600); err != nil {
		t.Fatal(err)
	}
}

func readyRuntime(t *testing.T, a *App) {
	t.Helper()
	for _, name := range []string{"proton", "toolmanifest.vdf", "compatibilitytool.vdf", "version"} {
		writeTestFile(t, filepath.Join(a.protonPath(), name), []byte("runtime"))
	}
	if err := os.MkdirAll(filepath.Join(a.protonPath(), "files"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"system.reg", "user.reg", "userdef.reg", "drive_c/windows/system32/msiexec.exe"} {
		writeTestFile(t, filepath.Join(a.Paths.Prefix, name), []byte("prefix"))
	}
	for _, name := range []string{"concrt140.dll", "msvcp140.dll", "vcruntime140.dll", "vcruntime140_1.dll"} {
		writeTestFile(t, filepath.Join(a.Paths.Prefix, "drive_c/windows/system32", name), []byte("dll"))
	}
}

func testDownloadClient(server *httptest.Server) Downloader {
	d := NewDownloader()
	d.Client.Transport = server.Client().Transport
	return d
}

func TestDownloadPreservesCacheOnFailures(t *testing.T) {
	for _, mode := range []string{"truncated", "checksum", "oversize", "notfound"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "truncated":
					w.Header().Set("Content-Length", "100")
					_, _ = w.Write([]byte("short"))
				case "oversize":
					_, _ = w.Write(bytes.Repeat([]byte("x"), 33))
				case "notfound":
					w.WriteHeader(404)
				default:
					_, _ = w.Write([]byte("wrong"))
				}
			}))
			defer server.Close()
			dest := filepath.Join(t.TempDir(), "cached.msi")
			writeTestFile(t, dest, []byte("valid old installer"))
			err := testDownloadClient(server).Download(context.Background(), server.URL, dest, 32, func(string) error { return errors.New("invalid checksum") })
			if err == nil {
				t.Fatal("expected download failure")
			}
			b, _ := os.ReadFile(dest)
			if string(b) != "valid old installer" {
				t.Fatal("previous cache was replaced")
			}
			leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(dest), ".download-*"))
			if len(leftovers) != 0 {
				t.Fatal("temporary download was retained")
			}
		})
	}
}

func TestDownloadPublishesVerifiedBytesAndRejectsHTTP(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("verified")) }))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "installer.msi")
	verified := false
	err := testDownloadClient(server).Download(context.Background(), server.URL, dest, 100, func(path string) error {
		b, _ := os.ReadFile(path)
		verified = string(b) == "verified"
		return nil
	})
	if err != nil || !verified {
		t.Fatalf("download: %v", err)
	}
	b, _ := os.ReadFile(dest)
	if string(b) != "verified" {
		t.Fatal("incorrect published download")
	}
	if err := NewDownloader().Download(context.Background(), "http://example.com/file", dest, 100, func(string) error { return nil }); err == nil {
		t.Fatal("accepted plaintext URL")
	}
}

func tarFixture(t *testing.T, headers []*tar.Header) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "proton.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, h := range headers {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write(bytes.Repeat([]byte("x"), int(h.Size))); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractProtonHandlesLinksWithoutFollowingThem(t *testing.T) {
	for _, item := range []struct {
		name    string
		headers []*tar.Header
		ok      bool
	}{
		{"valid", []*tar.Header{{Name: "GE/files/a", Typeflag: tar.TypeReg, Mode: 0755, Size: 1}, {Name: "GE/files/b", Typeflag: tar.TypeSymlink, Linkname: "a"}}, true},
		{"traversal", []*tar.Header{{Name: "GE/../../escape", Typeflag: tar.TypeReg, Size: 1}}, false},
		{"absolute", []*tar.Header{{Name: "/GE/file", Typeflag: tar.TypeReg, Size: 1}}, false},
		{"escaping link", []*tar.Header{{Name: "GE/files/link", Typeflag: tar.TypeSymlink, Linkname: "../../../escape"}}, false},
		{"symlink write", []*tar.Header{{Name: "GE/files/link", Typeflag: tar.TypeSymlink, Linkname: "target"}, {Name: "GE/files/link/file", Typeflag: tar.TypeReg, Size: 1}}, false},
		{"device", []*tar.Header{{Name: "GE/device", Typeflag: tar.TypeChar}}, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			dest := t.TempDir()
			err := extractProton(context.Background(), tarFixture(t, item.headers), dest, "GE")
			if (err == nil) != item.ok {
				t.Fatalf("extraction result: %v", err)
			}
			if item.ok {
				b, err := os.ReadFile(filepath.Join(dest, "GE/files/b"))
				if err != nil || string(b) != "x" {
					t.Fatal("relative symlink did not work")
				}
			}
		})
	}
}

func TestGameUpdatesPreservePrefixAndAreIdempotent(t *testing.T) {
	a, r := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	registry := filepath.Join(a.Paths.Prefix, "user.reg")
	writeTestFile(t, registry, []byte("saved login and settings"))
	body := append([]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, bytes.Repeat([]byte("m"), 128)...)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	a.Download = testDownloadClient(server)
	a.Profile.InstallerURL = server.URL
	for _, op := range []struct {
		repair, check bool
		expected      int
	}{{false, true, 0}, {false, false, 1}, {false, false, 1}, {true, false, 2}} {
		if err := a.Update(context.Background(), op.repair, op.check); err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, c := range r.commands {
			if c.Name == "umu-run" && len(c.Args) > 1 && c.Args[1] == "/i" {
				count++
				if c.Args[2] != windowsPath(filepath.Join(a.Paths.Cache, "PokemonTCGLiveInstaller.msi")) {
					t.Fatal("MSI path did not retain Unicode or spaces")
				}
			}
		}
		if count != op.expected {
			t.Fatalf("installer runs=%d; expected=%d", count, op.expected)
		}
	}
	b, _ := os.ReadFile(registry)
	if string(b) != "saved login and settings" {
		t.Fatal("update reset game settings")
	}
	if a.installation().Operation != "repair" {
		t.Fatal("repair did not update installation receipt")
	}
	last := r.commands[len(r.commands)-1]
	if !strings.Contains(strings.Join(last.Args, " "), "REINSTALL=ALL") {
		t.Fatal("same-MSI repair did not reinstall files")
	}
}

func TestFailedInstallerDoesNotAdvanceReceipt(t *testing.T) {
	a, r := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	old := Installation{InstallerSHA256: "previous"}
	if err := writeJSON(filepath.Join(a.Paths.State, "installation.json"), old); err != nil {
		t.Fatal(err)
	}
	body := []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	a.Download = testDownloadClient(server)
	a.Profile.InstallerURL = server.URL
	r.run = func(Command) error { return errors.New("installer crashed") }
	if a.Update(context.Background(), false, false) == nil {
		t.Fatal("installer failure was ignored")
	}
	if a.installation().InstallerSHA256 != "previous" {
		t.Fatal("failed installer advanced the receipt")
	}
}

func TestRunningGameBlocksMaintenanceButAllowsCallback(t *testing.T) {
	a, r := testApp(t)
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	r.run = func(c Command) error {
		if len(c.Args) > 1 && c.Args[1] == "-screen-fullscreen" {
			if c.Dir != filepath.Dir(a.gamePath()) {
				t.Fatal("updater has wrong working directory")
			}
			if err := a.Update(context.Background(), false, false); !errors.Is(err, ErrBusy) {
				t.Fatalf("update during game: %v", err)
			}
			if err := a.Callback(context.Background(), "tpcitcgapp://callback?code=SECRET&state=SECRET"); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	}
	if err := a.Play(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	var callback Command
	for _, c := range r.commands {
		if len(c.Args) > 1 && strings.HasPrefix(c.Args[1], "tpcitcgapp:") {
			callback = c
		}
	}
	if !callback.Quiet || !contains(callback.Env, "PROTON_VERB=runinprefix") || !contains(callback.Env, "PROTON_LOG=0") {
		t.Fatal("callback was not isolated from logs or running prefix")
	}
	if exists(filepath.Join(a.Paths.State, "play-session.json")) {
		t.Fatal("session marker retained after game exits")
	}
	if err := a.Callback(context.Background(), "tpcitcgapp://callback?code=SECRET"); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatal("inactive callback exposed secret or launched game")
	}
}

func contains(items []string, item string) bool {
	for _, v := range items {
		if v == item {
			return true
		}
	}
	return false
}

func TestCallbacksRejectBadTargetsWithoutEchoingInput(t *testing.T) {
	for _, raw := range []string{"", "https://callback?code=SECRET", "tpcitcgapp://evil?code=SECRET", "tpcitcgapp://callback:443?code=SECRET", "tpcitcgapp://user@callback?code=SECRET", "tpcitcgapp://callback?code=SECRET\n", "tpcitcgapp://callback?code=%bad%"} {
		if err := ValidateCallback(raw); err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Fatalf("bad callback: %v", err)
		}
	}
	if ValidateCallback("tpcitcgapp://callback?code=SECRET&state=SECRET") != nil {
		t.Fatal("valid callback rejected")
	}
}

func TestRuntimeChoiceSurvivesLauncherUpgrade(t *testing.T) {
	a, _ := testApp(t)
	old := a.Profile
	old.ProtonRelease = "GE-ProtonOld"
	old.ProtonDirectory = "GE-ProtonOld"
	old.ProtonSHA256 = strings.Repeat("a", 64)
	if err := writeJSON(filepath.Join(a.Paths.State, "runtime.json"), old); err != nil {
		t.Fatal(err)
	}
	reloaded, err := New(a.Paths, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Profile.ProtonRelease != "GE-ProtonOld" {
		t.Fatal("launcher silently selected new runtime")
	}
}

func TestIntegrationIsRepeatableAndRestoresPreviousHandler(t *testing.T) {
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test-session")
	main := filepath.Join(a.Paths.Hypr, "hyprland.lua")
	writeTestFile(t, main, []byte("-- user configuration\nrequire(\"hypr.monitors\")\n"))
	writeTestFile(t, a.Paths.Bin, []byte("binary"))
	handler := "previous.desktop"
	r.output = func(c Command) ([]byte, error) {
		if c.Name == "xdg-mime" {
			return []byte(handler), nil
		}
		return nil, nil
	}
	r.run = func(c Command) error {
		if c.Name == "xdg-mime" && c.Args[0] == "default" {
			handler = c.Args[1]
		}
		return nil
	}
	for i := 0; i < 2; i++ {
		if err := a.Integrate(context.Background()); err != nil {
			t.Fatal(err)
		}
		desktopPath := filepath.Join(a.Paths.Applications, desktopID)
		entry, err := os.ReadFile(desktopPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{
			"Actions=CheckUpdate;Update;Repair;Folders;Doctor;",
			"[Desktop Action CheckUpdate]\nName=Check installer updates\nExec=" + desktopQuote(a.Paths.Bin) + " update --check --gui",
			"[Desktop Action Repair]\nName=Repair game\nExec=" + desktopQuote(a.Paths.Bin) + " repair --gui",
			"[Desktop Action Folders]\nName=Open folders\nExec=" + desktopQuote(a.Paths.Bin) + " open --gui",
		} {
			if strings.Count(string(entry), action) != 1 {
				t.Fatalf("missing or duplicated desktop action %q", action)
			}
		}
		if validator, err := exec.LookPath("desktop-file-validate"); err == nil {
			if output, err := exec.Command(validator, desktopPath).CombinedOutput(); err != nil {
				t.Fatalf("invalid generated desktop entry: %v\n%s", err, output)
			}
		}
	}
	b, _ := os.ReadFile(main)
	if strings.Count(string(b), blockBegin) != 1 {
		t.Fatal("integration duplicated managed block")
	}
	state, err := a.integrationState()
	if err != nil || state.PreviousHandler != "previous.desktop" {
		t.Fatal("repeat integration lost previous handler")
	}
	if err := a.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(main)
	if !strings.Contains(string(b), "require(\"hypr.monitors\")") || strings.Contains(string(b), blockBegin) {
		t.Fatal("uninstall changed unrelated config")
	}
	if handler != "previous.desktop" {
		t.Fatal("uninstall did not restore previous callback handler")
	}
}

func TestInvalidHyprlandRulesRollback(t *testing.T) {
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test-session")
	main := filepath.Join(a.Paths.Hypr, "hyprland.lua")
	original := []byte("-- original config\n")
	writeTestFile(t, main, original)
	writeTestFile(t, a.Paths.Bin, []byte("binary"))
	r.output = func(c Command) ([]byte, error) {
		if c.Name == "hyprctl" && c.Args[0] == "configerrors" {
			return []byte("invalid rule"), nil
		}
		return nil, nil
	}
	if err := a.Integrate(context.Background()); err == nil {
		t.Fatal("invalid config was accepted")
	}
	b, _ := os.ReadFile(main)
	if !bytes.Equal(b, original) {
		t.Fatal("original config was not restored")
	}
	if exists(filepath.Join(a.Paths.Hypr, Name+".lua")) {
		t.Fatal("invalid rule file retained")
	}
}

func TestRuntimeDownloadRejectsWrongHashBeforeExtracting(t *testing.T) {
	a, _ := testApp(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("bad archive")) }))
	defer server.Close()
	a.Profile.ProtonURL = server.URL
	a.Download = testDownloadClient(server)
	if a.ensureProton(context.Background()) == nil {
		t.Fatal("invalid Proton checksum accepted")
	}
	if _, err := os.Stat(a.protonPath()); !os.IsNotExist(err) {
		t.Fatal("invalid runtime was published")
	}
}

func TestEnvironmentHasSingleOverride(t *testing.T) {
	t.Setenv("PROTON_VERB", "waitforexitandrun")
	env := environment(map[string]string{"PROTON_VERB": "runinprefix"})
	count := 0
	for _, v := range env {
		if strings.HasPrefix(v, "PROTON_VERB=") {
			count++
			if v != "PROTON_VERB=runinprefix" {
				t.Fatal("inherited setting won")
			}
		}
	}
	if count != 1 {
		t.Fatal("duplicate environment override")
	}
}

func TestPrefixMatchingHandlesProtonAlias(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(root, filepath.Join(root, "pfx")); err != nil {
		t.Fatal(err)
	}
	if !samePrefix(filepath.Join(root, "pfx")+"/", root) {
		t.Fatal("Proton prefix alias did not match")
	}
	if samePrefix(t.TempDir(), root) {
		t.Fatal("matched another installation")
	}
}

func TestCLIHelpDoesNotNeedRuntime(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := RunCLI(context.Background(), []string{"help"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "runtime-update") {
		t.Fatal("help failed")
	}
}

func TestRepeatLaunchUsesLuaFocusDispatcher(t *testing.T) {
	a, r := testApp(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test-session")
	// Keep an unexpected launch entirely local rather than provisioning a real
	// runtime over the network when process metadata is not available yet.
	readyRuntime(t, a)
	writeTestFile(t, a.gamePath(), []byte("game"))
	if err := os.MkdirAll(a.Paths.Prefix, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a.Paths.Prefix, filepath.Join(a.Paths.Prefix, "pfx")); err != nil {
		t.Fatal(err)
	}
	process := exec.Command("sleep", "30")
	process.Env = environment(map[string]string{"WINEPREFIX": filepath.Join(a.Paths.Prefix, "pfx") + "/"})
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = process.Process.Kill(); _ = process.Wait() }()
	window := Window{Address: "0x123", Title: "Pokemon TCG Live", PID: process.Process.Pid, XWayland: true}
	// /proc can briefly expose pre-exec metadata even after Start returns.
	// Wait for this fixture's environment instead of racing its exec boundary.
	deadline := time.Now().Add(2 * time.Second)
	for !a.ownWindow(window) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !a.ownWindow(window) {
		env, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", process.Process.Pid))
		t.Fatalf("process fixture metadata is unavailable: %v (environment bytes: %d)", err, len(env))
	}
	r.output = func(Command) ([]byte, error) {
		return []byte(fmt.Sprintf(`[{"address":"0x123","title":"Pokemon TCG Live","pid":%d,"xwayland":true}]`, process.Process.Pid)), nil
	}
	if err := a.Play(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	last := r.commands[len(r.commands)-1]
	if last.Name != "hyprctl" || !strings.Contains(strings.Join(last.Args, " "), "hl.dsp.focus") {
		t.Fatal("repeat launch did not use current Lua dispatcher")
	}
	for _, c := range r.commands {
		if c.Name == "umu-run" {
			t.Fatal("repeat launch started a second game")
		}
	}
}

func TestMIMERemovalPreservesOtherApplications(t *testing.T) {
	source := "[Default Applications]\ntext/html=browser.desktop\n" + callbackMIME + "=" + callbackID + ";previous.desktop;\n[Added Associations]\n" + callbackMIME + "=" + callbackID + ";\n"
	clean := removeMIMEReference(source)
	if strings.Contains(clean, callbackID) || !strings.Contains(clean, "text/html=browser.desktop") || !strings.Contains(clean, callbackMIME+"=previous.desktop;") {
		t.Fatal("MIME removal changed unrelated handlers")
	}
}

func TestAtomicWritePreservesConfigSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "dotfiles", "hyprland.lua")
	link := filepath.Join(root, "hyprland.lua")
	writeTestFile(t, target, []byte("original"))
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(link, []byte("updated"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("dotfile symlink was replaced")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "updated" {
		t.Fatal("symlink target was not updated")
	}
}
