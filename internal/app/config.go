package app

import (
	"encoding/json"
	"os"

	"ptcgl-omarchy/internal/config"
)

const Name = config.Name

type (
	Paths  = config.Paths
	Config = config.Config
)

func AtomicWrite(path string, data []byte, mode os.FileMode) error {
	return config.AtomicWrite(path, data, mode)
}

func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(path, append(b, '\n'), 0o600)
}

func exists(path string) bool { s, err := os.Stat(path); return err == nil && s.Mode().IsRegular() }
