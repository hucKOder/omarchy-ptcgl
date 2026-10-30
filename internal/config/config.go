// Package config resolves launcher paths and reads user configuration.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const Name = "ptcgl-omarchy"

type Paths struct {
	Home, Data, Cache, Config, State, Prefix, Bin, Applications, Hypr string
}

func UserPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	xdg := func(key, fallback string) (string, error) {
		value := os.Getenv(key)
		if value == "" {
			return filepath.Join(home, fallback), nil
		}
		if !filepath.IsAbs(value) {
			return "", fmt.Errorf("%s must be an absolute path", key)
		}
		return filepath.Clean(value), nil
	}
	data, err := xdg("XDG_DATA_HOME", ".local/share")
	if err != nil {
		return Paths{}, err
	}
	cache, err := xdg("XDG_CACHE_HOME", ".cache")
	if err != nil {
		return Paths{}, err
	}
	config, err := xdg("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return Paths{}, err
	}
	state, err := xdg("XDG_STATE_HOME", ".local/state")
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		Home: home, Data: filepath.Join(data, Name), Cache: filepath.Join(cache, Name),
		Config: filepath.Join(config, Name), State: filepath.Join(state, Name),
		Prefix: filepath.Join(data, Name, "prefix"), Bin: filepath.Join(home, ".local/bin", Name),
		Applications: filepath.Join(data, "applications"), Hypr: filepath.Join(config, "hypr"),
	}, nil
}

type Config struct {
	// Empty selects the pinned compatibility profile shipped with the launcher.
	ProtonPath string   `json:"proton_path,omitempty"`
	ExtraArgs  []string `json:"extra_args,omitempty"`
}

func LoadConfig(p Paths) (Config, error) {
	var c Config
	b, err := os.ReadFile(filepath.Join(p.Config, "config.json"))
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("config.json: %w", err)
	}
	if c.ProtonPath != "" && !filepath.IsAbs(c.ProtonPath) {
		return c, fmt.Errorf("proton_path must be absolute")
	}
	return c, nil
}

func AtomicWrite(path string, data []byte, mode os.FileMode) error {
	// Preserve dotfile-manager symlinks by replacing the resolved target.
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		path = resolved
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ptcgl-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
