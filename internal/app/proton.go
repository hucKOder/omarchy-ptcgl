package app

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//go:embed assets/*
var assets embed.FS

type Profile struct {
	UMUMinimum      string `json:"umu_minimum"`
	ProtonRelease   string `json:"proton_release"`
	ProtonDirectory string `json:"proton_directory"`
	ProtonURL       string `json:"proton_url"`
	ProtonSHA256    string `json:"proton_sha256"`
	InstallerURL    string `json:"installer_url"`
}

func DefaultProfile() Profile {
	b, _ := assets.ReadFile("assets/compatibility.json")
	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		panic(err)
	}
	return p
}

func (a *App) protonPath() string {
	if a.Config.ProtonPath != "" {
		return a.Config.ProtonPath
	}
	return filepath.Join(a.Paths.Data, "runtime", a.Profile.ProtonDirectory)
}

func validateProton(root string) error {
	for _, name := range []string{"proton", "toolmanifest.vdf", "compatibilitytool.vdf", "version"} {
		if !exists(filepath.Join(root, name)) {
			return fmt.Errorf("Proton is incomplete: missing %s", name)
		}
	}
	s, err := os.Stat(filepath.Join(root, "files"))
	if err != nil || !s.IsDir() {
		return fmt.Errorf("Proton files directory is missing")
	}
	return nil
}

func (a *App) ensureProton(ctx context.Context) error {
	root := a.protonPath()
	if a.Config.ProtonPath != "" {
		return validateProton(root)
	}
	if _, err := os.Stat(root); err == nil {
		if validateProton(root) == nil {
			return writeJSON(filepath.Join(a.Paths.State, "runtime.json"), a.Profile)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	archive := filepath.Join(a.Paths.Cache, a.Profile.ProtonDirectory+".tar.gz")
	verify := func(path string) error {
		hash, err := fileSHA256(path)
		if err != nil {
			return err
		}
		if hash != a.Profile.ProtonSHA256 {
			return fmt.Errorf("GE-Proton SHA-256 mismatch")
		}
		return nil
	}
	if verify(archive) != nil {
		a.phase("downloading", "Downloading "+safeLabel(a.Profile.ProtonRelease)+"…")
		if err := a.download(ctx, a.Profile.ProtonURL, archive, 2<<30, verify); err != nil {
			return err
		}
	}
	parent := filepath.Dir(root)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(parent, ".proton-*")
	if err != nil {
		return err
	}
	keepStaging := false
	defer func() {
		if !keepStaging {
			_ = os.RemoveAll(staging)
		}
	}()
	a.phase("extracting", "Extracting GE-Proton…")
	if err := extractProton(ctx, archive, staging, a.Profile.ProtonDirectory); err != nil {
		return err
	}
	staged := filepath.Join(staging, a.Profile.ProtonDirectory)
	if err := validateProton(staged); err != nil {
		return err
	}
	// Only replace the selected, invalid managed directory after a complete
	// replacement is ready. Preserve the original on publication failure.
	backup := filepath.Join(staging, "incomplete-runtime")
	replaced := false
	if _, err := os.Lstat(root); err == nil {
		if err := os.Rename(root, backup); err != nil {
			return err
		}
		replaced = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(staged, root); err != nil {
		if replaced {
			if restoreErr := os.Rename(backup, root); restoreErr != nil {
				// Do not remove the only remaining copy if restoration fails.
				keepStaging = true
				return fmt.Errorf("publish runtime: %w; previous incomplete runtime is at %s", err, backup)
			}
		}
		return err
	}
	return writeJSON(filepath.Join(a.Paths.State, "runtime.json"), a.Profile)
}

// A launcher upgrade does not silently replace the selected compatibility stack.
// New releases retain the saved profile until this command is explicitly used.
func (a *App) UpdateRuntime(ctx context.Context) error {
	return a.withOperation("runtime-update", func() error {
		if a.Config.ProtonPath != "" {
			return fmt.Errorf("a custom proton_path is configured; manage that runtime directly")
		}
		previous := a.Profile
		a.Profile = DefaultProfile()
		if err := a.ensureProton(ctx); err != nil {
			a.Profile = previous
			return err
		}
		a.progress("Selected " + a.Profile.ProtonRelease + ". Existing game data is retained; the previous runtime remains available for rollback.")
		return nil
	})
}

// Extract into a fresh, root-confined tree. Defer links until regular files and
// all link parents exist, and validate the full link graph without cleaning away
// ".." before expanding symlinks. Devices and hardlinks are rejected.
func extractProton(ctx context.Context, archive, dest, expectedRoot string) error {
	if expectedRoot == "" || expectedRoot == "." || expectedRoot == ".." || filepath.Base(expectedRoot) != expectedRoot {
		return fmt.Errorf("invalid Proton directory")
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	destination, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer destination.Close()
	if err := destination.Mkdir(expectedRoot, 0755); err != nil {
		return err
	}
	tree, err := destination.OpenRoot(expectedRoot)
	if err != nil {
		return err
	}
	defer tree.Close()
	tr := tar.NewReader(gz)
	type link struct{ name, target string }
	var links []link
	targets := make(map[string]string)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(h.Name)
		if filepath.IsAbs(h.Name) || (name != expectedRoot && !strings.HasPrefix(name, expectedRoot+"/")) {
			return fmt.Errorf("invalid archive path")
		}
		path := strings.TrimPrefix(name, expectedRoot+"/")
		if name == expectedRoot {
			path = "."
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := tree.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if h.Size < 0 || h.Size > 8<<30-total {
				return fmt.Errorf("Proton archive exceeds extraction limit")
			}
			total += h.Size
			if err := tree.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			out, err := tree.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(h.Mode)&0777)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if path == "." || h.Linkname == "" || filepath.IsAbs(h.Linkname) {
				return fmt.Errorf("invalid archive symlink")
			}
			if _, duplicate := targets[path]; duplicate {
				return fmt.Errorf("duplicate archive symlink")
			}
			targets[path] = h.Linkname
			links = append(links, link{path, h.Linkname})
		default:
			return fmt.Errorf("unsupported Proton archive entry type %d", h.Typeflag)
		}
	}
	for _, l := range links {
		if err := validateArchiveLink(ctx, l.name, targets); err != nil {
			return err
		}
		// No symlinks exist yet. An archive link used as another entry's parent
		// becomes a directory here and is rejected at link creation, not followed.
		if err := tree.MkdirAll(filepath.Dir(l.name), 0755); err != nil {
			return err
		}
	}
	for _, l := range links {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := tree.Symlink(l.target, l.name); err != nil {
			return err
		}
	}
	return nil
}

// Resolve components in filesystem order, including links whose targets do not
// exist yet. EvalSymlinks cannot validate dangling links, and filepath.Clean
// incorrectly treats "link/.." as lexical rather than following link first.
func validateArchiveLink(ctx context.Context, name string, links map[string]string) error {
	pending := strings.Split(filepath.Dir(name)+"/"+links[name], "/")
	var resolved []string
	expansions := 0
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(resolved) == 0 {
				return fmt.Errorf("archive symlink escapes Proton directory")
			}
			resolved = resolved[:len(resolved)-1]
		default:
			resolved = append(resolved, part)
			if target, ok := links[strings.Join(resolved, "/")]; ok {
				expansions++
				if expansions > 40 {
					return fmt.Errorf("archive symlink cycle or excessive link depth")
				}
				resolved = resolved[:len(resolved)-1]
				pending = append(strings.Split(target, "/"), pending...)
			}
		}
	}
	return nil
}
