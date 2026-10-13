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
		if err := validateProton(root); err != nil {
			return err
		}
		return writeJSON(filepath.Join(a.Paths.State, "runtime.json"), a.Profile)
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
		a.progress("Downloading " + a.Profile.ProtonRelease + "…")
		if err := a.Download.Download(ctx, a.Profile.ProtonURL, archive, 2<<30, verify); err != nil {
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
	defer os.RemoveAll(staging)
	a.progress("Extracting GE-Proton…")
	if err := extractProton(ctx, archive, staging, a.Profile.ProtonDirectory); err != nil {
		return err
	}
	staged := filepath.Join(staging, a.Profile.ProtonDirectory)
	if err := validateProton(staged); err != nil {
		return err
	}
	if err := os.Rename(staged, root); err != nil {
		return err
	}
	return writeJSON(filepath.Join(a.Paths.State, "runtime.json"), a.Profile)
}

// A launcher upgrade does not silently replace the selected compatibility stack.
// New releases retain the saved profile until this command is explicitly used.
func (a *App) UpdateRuntime(ctx context.Context) error {
	return a.withLock(func() error {
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

// Proton contains relative symlinks. Extract regular files first, then links;
// this prevents later archive entries from writing through a symlink. Links
// must remain inside the expected Proton tree. Devices and hardlinks are rejected.
func extractProton(ctx context.Context, archive, dest, expectedRoot string) error {
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
	tr := tar.NewReader(gz)
	type link struct{ name, target string }
	var links []link
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
		path := filepath.Join(dest, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			total += h.Size
			if h.Size < 0 || total > 8<<30 {
				return fmt.Errorf("Proton archive exceeds extraction limit")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(h.Mode)&0777)
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
			target := filepath.Clean(filepath.Join(filepath.Dir(name), h.Linkname))
			if filepath.IsAbs(h.Linkname) || (target != expectedRoot && !strings.HasPrefix(target, expectedRoot+"/")) {
				return fmt.Errorf("archive symlink escapes Proton directory")
			}
			links = append(links, link{path, h.Linkname})
		default:
			return fmt.Errorf("unsupported Proton archive entry type %d", h.Typeflag)
		}
	}
	for _, l := range links {
		if err := os.MkdirAll(filepath.Dir(l.name), 0755); err != nil {
			return err
		}
		if err := os.Symlink(l.target, l.name); err != nil {
			return err
		}
	}
	return nil
}
