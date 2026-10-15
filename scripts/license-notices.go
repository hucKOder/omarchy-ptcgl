//go:build ignore

// Generate the license notices shipped inside the Linux launcher.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type module struct {
	Path, Version, Dir string
	Main               bool
}

func generate() ([]byte, error) {
	cmd := exec.Command("go", "list", "-deps", "-json", "./cmd/ptcgl-omarchy")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	modules := make(map[string]module)
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var pkg struct{ Module *module }
		if err := decoder.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if pkg.Module != nil && !pkg.Module.Main {
			modules[pkg.Module.Path] = *pkg.Module
		}
	}
	paths := make([]string, 0, len(modules))
	for path := range modules {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var notices bytes.Buffer
	notices.WriteString("Omarchy PTCGL — bundled software licenses\n\n")
	license, err := os.ReadFile("LICENSE")
	if err != nil {
		return nil, err
	}
	notices.Write(license)
	goLicense, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "LICENSE"))
	if err != nil {
		return nil, err
	}
	notices.WriteString("\n\n============================================================\nGo standard library and runtime\n============================================================\n\n")
	notices.Write(goLicense)
	for _, path := range paths {
		mod := modules[path]
		entries, err := os.ReadDir(mod.Dir)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&notices, "\n\n============================================================\n%s %s\n============================================================\n", path, mod.Version)
		found := false
		for _, entry := range entries {
			name := strings.ToUpper(entry.Name())
			if entry.IsDir() || !(strings.HasPrefix(name, "LICENSE") || name == "COPYING" || name == "NOTICE" || name == "COPYRIGHT") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(mod.Dir, entry.Name()))
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(&notices, "\n--- %s ---\n\n", entry.Name())
			notices.Write(data)
			found = found || strings.HasPrefix(name, "LICENSE") || name == "COPYING"
		}
		if !found {
			return nil, fmt.Errorf("no license found for %s", path)
		}
	}
	return notices.Bytes(), nil
}

func main() {
	check := flag.Bool("check", false, "check the committed notices without changing them")
	flag.Parse()
	data, err := generate()
	const destination = "internal/app/assets/third-party-notices.txt"
	if err == nil && *check {
		var existing []byte
		existing, err = os.ReadFile(destination)
		if err == nil && !bytes.Equal(existing, data) {
			err = fmt.Errorf("bundled license notices are outdated; run make licenses")
		}
	} else if err == nil {
		err = os.WriteFile(destination, data, 0644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
