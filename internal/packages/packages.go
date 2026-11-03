package packages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/enver-dev/enver/internal/run"
	"github.com/enver-dev/enver/internal/vfox"
)

// ErrUnsupported means package introspection is not implemented for an SDK yet.
var ErrUnsupported = errors.New("package listing is not supported for this runtime yet")

// Package is one globally installed library.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Manager string `json:"manager"`
}

// List returns globally installed packages for an installed runtime.
func List(ctx context.Context, sdk, versionPath string) ([]Package, error) {
	switch sdk {
	case "nodejs", "node":
		return listNode(ctx, versionPath)
	case "python":
		return listPython(ctx, versionPath)
	default:
		return nil, ErrUnsupported
	}
}

func listNode(ctx context.Context, versionPath string) ([]Package, error) {
	bin := vfox.BinDir("nodejs", versionPath)
	env := vfox.MergeEnv(os.Environ(), map[string]string{
		"PATH": bin + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	npm := filepath.Join(bin, "npm")
	if runtime.GOOS == "windows" {
		npm += ".cmd"
		if _, err := os.Stat(npm); err != nil {
			if p, lookErr := exec.LookPath("npm.cmd"); lookErr == nil {
				npm = p
			}
		}
	} else if _, err := os.Stat(npm); err != nil {
		if p, lookErr := exec.LookPath("npm"); lookErr == nil {
			npm = p
		}
	}
	var out string
	var err error
	if runtime.GOOS == "windows" {
		// cmd.exe resolves npm.cmd through PATH, which is already prefixed.
		out, err = run.Output(ctx, "cmd.exe", []string{"/c", "npm.cmd", "ls", "-g", "--json", "--depth=0"}, versionPath, env)
	} else {
		out, err = run.Output(ctx, npm, []string{"ls", "-g", "--json", "--depth=0"}, versionPath, env)
	}
	if err != nil && strings.TrimSpace(out) == "" {
		return nil, fmt.Errorf("npm ls failed: %w", err)
	}
	start := strings.Index(out, "{")
	end := strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("unexpected npm output: %s", truncate(out, 200))
	}
	var payload struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(out[start:end+1]), &payload); err != nil {
		return nil, fmt.Errorf("parse npm output: %w", err)
	}
	result := make([]Package, 0, len(payload.Dependencies))
	for name, meta := range payload.Dependencies {
		result = append(result, Package{Name: name, Version: meta.Version, Manager: "npm"})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func listPython(ctx context.Context, versionPath string) ([]Package, error) {
	exe := pythonExe(versionPath)
	if exe == "" {
		return nil, errors.New("python interpreter not found in this version")
	}
	out, err := run.Output(ctx, exe, []string{"-m", "pip", "list", "--format=json", "--disable-pip-version-check"}, versionPath, nil)
	if err != nil && strings.TrimSpace(out) == "" {
		return nil, fmt.Errorf("pip list failed: %w", err)
	}
	start := strings.Index(out, "[")
	end := strings.LastIndex(out, "]")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("unexpected pip output: %s", truncate(out, 200))
	}
	var payload []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(out[start:end+1]), &payload); err != nil {
		return nil, fmt.Errorf("parse pip output: %w", err)
	}
	result := make([]Package, 0, len(payload))
	for _, p := range payload {
		result = append(result, Package{Name: p.Name, Version: p.Version, Manager: "pip"})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func pythonExe(versionPath string) string {
	candidates := []string{
		filepath.Join(versionPath, "python.exe"),
		filepath.Join(versionPath, "bin", "python3"),
		filepath.Join(versionPath, "bin", "python"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// Export describes a portable package manifest.
type Export struct {
	Filename string
	Content  string
}

// BuildExport renders a package list as a manifest for the runtime.
func BuildExport(sdk string, pkgs []Package) Export {
	sorted := append([]Package(nil), pkgs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	switch sdk {
	case "nodejs", "node":
		deps := map[string]string{}
		for _, p := range sorted {
			deps[p.Name] = p.Version
		}
		data, _ := json.MarshalIndent(map[string]any{"dependencies": deps}, "", "  ")
		return Export{Filename: "global-packages.json", Content: string(data) + "\n"}
	default:
		var b strings.Builder
		for _, p := range sorted {
			b.WriteString(p.Name)
			b.WriteString("==")
			b.WriteString(p.Version)
			b.WriteString("\n")
		}
		return Export{Filename: "requirements.txt", Content: b.String()}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
