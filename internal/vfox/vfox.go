package vfox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MinVersion is the oldest vfox release enver supports.
const MinVersion = "1.0.13"

// Manager is a thin, defensive adapter over the vfox CLI.
type Manager struct {
	Bin     string
	Home    string
	Version string

	writeMu  sync.Mutex
	cfgMu    sync.RWMutex
	sdkRoot  string
	registry string
}

// Info is the serializable state of the local vfox installation.
type Info struct {
	Bin        string `json:"bin"`
	Version    string `json:"version"`
	Home       string `json:"home"`
	SDKRoot    string `json:"sdkRoot"`
	Registry   string `json:"registry"`
	ConfigPath string `json:"configPath"`
	Compatible bool   `json:"compatible"`
	MinVersion string `json:"minVersion"`
}

// SDK is one runtime managed by vfox.
type SDK struct {
	Name     string    `json:"name"`
	Current  string    `json:"current,omitempty"`
	Versions []Version `json:"versions"`
}

// Version is an installed or available runtime version.
type Version struct {
	Version   string `json:"version"`
	Path      string `json:"path,omitempty"`
	Installed bool   `json:"installed"`
	Current   bool   `json:"current"`
	Stable    bool   `json:"stable,omitempty"`
	LTS       bool   `json:"lts,omitempty"`
	Meta      string `json:"meta,omitempty"`
}

// EnvInfo mirrors `vfox env --json`.
type EnvInfo struct {
	IsHookEnv bool           `json:"is_hook_env"`
	Paths     []string       `json:"paths"`
	SDKs      map[string]any `json:"sdks"`
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07]*\x07`)

// StripANSI removes terminal escape sequences from vfox output.
func StripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

// HomePath resolves the vfox home directory.
func HomePath() string {
	if v := os.Getenv("VFOX_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".vfox"
	}
	current := filepath.Join(home, ".vfox")
	if _, err := os.Stat(current); err == nil {
		return current
	}
	legacy := filepath.Join(home, ".version-fox")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return current
}

// Detect locates vfox and reads its current configuration.
func Detect(override string) (*Manager, error) {
	bin, err := findBinary(override)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return nil, fmt.Errorf("run %s --version: %w", bin, err)
	}
	version := ""
	if m := regexp.MustCompile(`(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)`).FindStringSubmatch(string(out)); len(m) > 1 {
		version = m[1]
	}
	if version == "" {
		return nil, fmt.Errorf("cannot parse vfox version from %q", strings.TrimSpace(string(out)))
	}
	m := &Manager{Bin: bin, Home: HomePath(), Version: version}
	if err := m.Refresh(ctx); err != nil {
		// A missing/broken config is not fatal; defaults are usable.
		m.setLayout(filepath.Join(m.Home, "cache"), "")
	}
	return m, nil
}

func findBinary(override string) (string, error) {
	candidates := []string{}
	if override != "" {
		candidates = append(candidates, override)
	}
	if v := os.Getenv("ENVER_VFOX_BIN"); v != "" {
		candidates = append(candidates, v)
	}
	if p, err := exec.LookPath("vfox"); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	candidates = append(candidates,
		filepath.Join(home, ".vfox", "bin", "vfox.exe"),
		filepath.Join(home, ".vfox", "bin", "vfox"),
		`C:\Program Files\vfox\vfox.exe`,
		"/usr/local/bin/vfox",
		"/opt/homebrew/bin/vfox",
	)
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("vfox not found; install it from https://vfox.dev or set ENVER_VFOX_BIN")
}

// Refresh re-reads vfox configuration.
func (m *Manager) Refresh(ctx context.Context) error {
	cfg, err := m.Config(ctx)
	if err != nil {
		return err
	}
	root := strings.TrimSpace(cfg["storage.sdkPath"])
	if root == "" {
		root = filepath.Join(m.Home, "cache")
	} else if !filepath.IsAbs(root) {
		root = filepath.Join(m.Home, root)
	}
	m.setLayout(filepath.Clean(root), strings.TrimSpace(cfg["registry.address"]))
	return nil
}

func (m *Manager) setLayout(root, registry string) {
	m.cfgMu.Lock()
	defer m.cfgMu.Unlock()
	m.sdkRoot = root
	m.registry = registry
}

// SDKRoot is the directory that holds installed SDK versions.
func (m *Manager) SDKRoot() string {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return m.sdkRoot
}

// RegistryURL returns the configured plugin registry, if any.
func (m *Manager) RegistryURL() string {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return m.registry
}

// Info returns the current adapter state.
func (m *Manager) Info() Info {
	return Info{
		Bin:        m.Bin,
		Version:    m.Version,
		Home:       m.Home,
		SDKRoot:    m.SDKRoot(),
		Registry:   m.RegistryURL(),
		ConfigPath: filepath.Join(m.Home, "config.yaml"),
		Compatible: CompareVersions(m.Version, MinVersion) >= 0,
		MinVersion: MinVersion,
	}
}

// Config lists vfox configuration as key/value pairs.
func (m *Manager) Config(ctx context.Context) (map[string]string, error) {
	out, err := m.runOutput(ctx, "config", "-l")
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, " = ")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+3:])
		if key != "" {
			result[key] = val
		}
	}
	return result, nil
}

// ConfigSet writes a vfox configuration value.
func (m *Manager) ConfigSet(ctx context.Context, key, value string) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	out, err := m.runOutput(ctx, "config", key, value)
	if err != nil {
		return wrapCmdErr(out, err)
	}
	return m.Refresh(ctx)
}

// ConfigUnset removes a vfox configuration value.
func (m *Manager) ConfigUnset(ctx context.Context, key string) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	out, err := m.runOutput(ctx, "config", "--unset", key)
	if err != nil {
		return wrapCmdErr(out, err)
	}
	return m.Refresh(ctx)
}

// Installed scans installed SDK versions from disk.
func (m *Manager) Installed(ctx context.Context) ([]SDK, error) {
	root := m.SDKRoot()
	if root == "" {
		return nil, errors.New("vfox SDK root is not configured")
	}
	roots := []string{root}
	legacy := filepath.Join(root, "cache")
	if info, err := os.Stat(legacy); err == nil && info.IsDir() {
		roots = append(roots, legacy)
	}
	found := map[string]map[string]string{}
	for _, base := range roots {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, sdkEntry := range entries {
			if !sdkEntry.IsDir() || strings.HasPrefix(sdkEntry.Name(), ".") {
				continue
			}
			sdkName := sdkEntry.Name()
			versions, err := os.ReadDir(filepath.Join(base, sdkName))
			if err != nil {
				continue
			}
			for _, verEntry := range versions {
				if !verEntry.IsDir() || !strings.HasPrefix(verEntry.Name(), "v-") {
					continue
				}
				version := strings.TrimPrefix(verEntry.Name(), "v-")
				if version == "" {
					continue
				}
				// A sibling .installing marker means vfox never finished this
				// version. Treating it as installed would offer broken runtimes.
				marker := filepath.Join(base, sdkName, ".v-"+version+".installing")
				if _, markerErr := os.Stat(marker); markerErr == nil {
					continue
				}
				versionDir := filepath.Join(base, sdkName, verEntry.Name())
				resolved := resolveVersionDir(versionDir)
				if isEmptyDir(resolved) {
					continue
				}
				if found[sdkName] == nil {
					found[sdkName] = map[string]string{}
				}
				if _, exists := found[sdkName][version]; !exists {
					found[sdkName][version] = resolved
				}
			}
		}
	}
	// Plugins that are installed but have no versions should still be visible.
	plugins := m.PluginNames()
	for _, name := range plugins {
		if found[name] == nil {
			found[name] = map[string]string{}
		}
	}
	current := m.currentVersions()
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]SDK, 0, len(names))
	for _, name := range names {
		versions := make([]Version, 0, len(found[name]))
		for version, path := range found[name] {
			versions = append(versions, Version{
				Version:   version,
				Path:      path,
				Installed: true,
				Current:   current[name] == version,
			})
		}
		sort.Slice(versions, func(i, j int) bool {
			return CompareVersions(versions[i].Version, versions[j].Version) > 0
		})
		result = append(result, SDK{Name: name, Current: current[name], Versions: versions})
	}
	return result, nil
}

func resolveVersionDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return dir
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 1 {
		return filepath.Join(dir, dirs[0])
	}
	return dir
}

func isEmptyDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err != nil || len(entries) == 0
}

// PluginNames lists installed vfox plugins.
func (m *Manager) PluginNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, dir := range []string{"plugin", "plugins"} {
		entries, err := os.ReadDir(filepath.Join(m.Home, dir))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && !seen[e.Name()] {
				seen[e.Name()] = true
				out = append(out, e.Name())
			}
		}
	}
	sort.Strings(out)
	return out
}

func (m *Manager) currentVersions() map[string]string {
	out := map[string]string{}
	entries, err := os.ReadDir(filepath.Join(m.Home, "sdks"))
	if err == nil {
		for _, e := range entries {
			target, linkErr := os.Readlink(filepath.Join(m.Home, "sdks", e.Name()))
			if linkErr == nil {
				if v := versionFromPath(target); v != "" {
					out[e.Name()] = v
				}
			}
		}
	}
	// The global selection file is a more reliable fallback on Windows.
	for name, version := range ParseToolsToml(filepath.Join(m.Home, ".vfox.toml")) {
		if _, ok := out[name]; !ok {
			out[name] = version
		}
	}
	return out
}

func versionFromPath(p string) string {
	parts := strings.Split(filepath.ToSlash(p), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.HasPrefix(parts[i], "v-") {
			return strings.TrimPrefix(parts[i], "v-")
		}
	}
	return ""
}

// Search lists versions available from an SDK plugin.
func (m *Manager) Search(ctx context.Context, sdk string) ([]Version, error) {
	out, err := m.runOutput(ctx, "search", sdk)
	if err != nil {
		return nil, err
	}
	installed := map[string]string{}
	if sdks, scanErr := m.Installed(ctx); scanErr == nil {
		for _, s := range sdks {
			if s.Name != sdk {
				continue
			}
			for _, v := range s.Versions {
				installed[v.Version] = v.Path
			}
		}
	}
	current := m.currentVersions()[sdk]
	var result []Version
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(StripANSI(raw))
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		rest := strings.TrimSpace(line[2:])
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		version := fields[0]
		meta := strings.TrimSpace(strings.TrimPrefix(rest, version))
		path := installed[version]
		result = append(result, Version{
			Version:   version,
			Path:      path,
			Installed: path != "",
			Current:   current == version,
			Stable:    strings.Contains(meta, "stable"),
			LTS:       strings.Contains(meta, "LTS"),
			Meta:      meta,
		})
	}
	return result, nil
}

// Install installs a runtime version, streaming normalized log lines.
func (m *Manager) Install(ctx context.Context, sdk, version string, onLine func(string)) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	cmd := m.command(ctx, "install", sdk+"@"+version)
	lw := newLineWriter(onLine)
	cmd.Stdout = lw
	cmd.Stderr = lw
	err := cmd.Run()
	lw.Flush()
	return err
}

// Uninstall removes a runtime version.
func (m *Manager) Uninstall(ctx context.Context, sdk, version string, onLine func(string)) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	cmd := m.command(ctx, "uninstall", sdk+"@"+version)
	lw := newLineWriter(onLine)
	cmd.Stdout = lw
	cmd.Stderr = lw
	err := cmd.Run()
	lw.Flush()
	return err
}

// Use switches the active version for a scope.
func (m *Manager) Use(ctx context.Context, sdk, version, scope, dir string, onLine func(string)) error {
	flag := "-g"
	switch scope {
	case "", "global":
		flag = "-g"
	case "project":
		if dir == "" {
			return errors.New("project directory is required")
		}
		flag = "-p"
	case "session":
		flag = "-s"
	default:
		return fmt.Errorf("invalid scope %q", scope)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	cmd := m.command(ctx, "use", flag, sdk+"@"+version)
	// Tell vfox we are a hook environment. Without this, `vfox use` tries to
	// spawn the parent process as a new shell, which is wrong (and recursive)
	// when called from a long-lived GUI/API server.
	cmd.Env = append(cmd.Env, "__VFOX_SHELL="+hookShellName())
	if flag == "-p" {
		cmd.Dir = dir
	}
	lw := newLineWriter(onLine)
	cmd.Stdout = lw
	cmd.Stderr = lw
	err := cmd.Run()
	lw.Flush()
	return err
}

// AddPlugin installs an SDK plugin from the registry.
func (m *Manager) AddPlugin(ctx context.Context, sdk string, onLine func(string)) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	cmd := m.command(ctx, "add", sdk)
	lw := newLineWriter(onLine)
	cmd.Stdout = lw
	cmd.Stderr = lw
	err := cmd.Run()
	lw.Flush()
	return err
}

// RemovePlugin removes an SDK plugin.
func (m *Manager) RemovePlugin(ctx context.Context, sdk string, onLine func(string)) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	cmd := m.command(ctx, "remove", sdk)
	lw := newLineWriter(onLine)
	cmd.Stdout = lw
	cmd.Stderr = lw
	err := cmd.Run()
	lw.Flush()
	return err
}

// EnvJSON reads the vfox environment snapshot.
func (m *Manager) EnvJSON(ctx context.Context) (*EnvInfo, error) {
	out, err := m.runOutput(ctx, "env", "--json")
	if err != nil {
		return nil, err
	}
	start := strings.Index(out, "{")
	end := strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("unexpected vfox env output: %s", truncate(out, 200))
	}
	var info EnvInfo
	if err := json.Unmarshal([]byte(out[start:end+1]), &info); err != nil {
		return nil, fmt.Errorf("parse vfox env: %w", err)
	}
	return &info, nil
}

// Current returns the currently selected version for an SDK.
func (m *Manager) Current(ctx context.Context, sdk string) string {
	if v := m.currentVersions()[sdk]; v != "" {
		return v
	}
	out, err := m.runOutput(ctx, "current", sdk)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(StripANSI(out))
	line = strings.TrimPrefix(line, "->")
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "v")
	return line
}

// ProjectVersions reads a project's .vfox.toml.
func ProjectVersions(dir string) map[string]string {
	return ParseToolsToml(filepath.Join(dir, ".vfox.toml"))
}

// ParseToolsToml extracts the [tools] table from a vfox TOML file.
func ParseToolsToml(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := map[string]string{}
	section := ""
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		if section != "tools" {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if key == "" {
			continue
		}
		val = strings.Trim(val, `"'`)
		out[key] = val
	}
	return out
}

// Environment builds a process environment from vfox state plus overrides.
func (m *Manager) Environment(ctx context.Context, prepend []string, extra map[string]string) ([]string, error) {
	info, err := m.EnvJSON(ctx)
	if err != nil {
		return nil, err
	}
	paths := append([]string{}, prepend...)
	paths = append(paths, info.Paths...)
	updates := map[string]string{}
	for k, v := range extra {
		updates[k] = v
	}
	base := os.Environ()
	base = PrependPaths(base, paths)
	return MergeEnv(base, updates), nil
}

// PrependPaths adds directories to PATH in the given order.
func PrependPaths(env []string, dirs []string) []string {
	clean := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d != "" {
			clean = append(clean, d)
		}
	}
	if len(clean) == 0 {
		return env
	}
	prefix := strings.Join(clean, string(os.PathListSeparator))
	key := "PATH"
	value := ""
	for i, kv := range env {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			continue
		}
		if strings.EqualFold(kv[:idx], "PATH") {
			key = kv[:idx]
			value = kv[idx+1:]
			env = append(env[:i], env[i+1:]...)
			break
		}
	}
	if value != "" {
		prefix += string(os.PathListSeparator) + value
	}
	return append(env, key+"="+prefix)
}

// MergeEnv applies updates to base, matching keys case-insensitively.
func MergeEnv(base []string, updates map[string]string) []string {
	out := append([]string(nil), base...)
	index := map[string]int{}
	for i, kv := range out {
		if idx := strings.IndexByte(kv, '='); idx > 0 {
			index[strings.ToLower(kv[:idx])] = i
		}
	}
	for k, v := range updates {
		lk := strings.ToLower(k)
		if i, ok := index[lk]; ok {
			out[i] = k + "=" + v
		} else {
			index[lk] = len(out)
			out = append(out, k+"="+v)
		}
	}
	return out
}

func hookShellName() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "bash"
	}
	return filepath.Base(shell)
}

// BinDir returns the directory that should be prepended to PATH for a version.
func BinDir(sdk, versionPath string) string {
	if runtime.GOOS == "windows" {
		return versionPath
	}
	candidate := filepath.Join(versionPath, "bin")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	return versionPath
}

// CompareVersions compares dotted versions, treating prereleases as lower.
func CompareVersions(a, b string) int {
	an, ap := splitVersion(a)
	bn, bp := splitVersion(b)
	for i := 0; i < len(an) || i < len(bn); i++ {
		var av, bv int
		if i < len(an) {
			av = an[i]
		}
		if i < len(bn) {
			bv = bn[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	if ap == bp {
		return 0
	}
	if ap == "" {
		return 1
	}
	if bp == "" {
		return -1
	}
	if ap < bp {
		return -1
	}
	return 1
}

func splitVersion(v string) ([]int, string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	pre := ""
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		pre = v[idx+1:]
		v = v[:idx]
	}
	var nums []int
	for _, part := range strings.Split(v, ".") {
		n := 0
		digits := ""
		for _, r := range part {
			if r < '0' || r > '9' {
				break
			}
			digits += string(r)
		}
		if digits != "" {
			n, _ = strconv.Atoi(digits)
		}
		nums = append(nums, n)
	}
	return nums, pre
}

// VersionAtLeast reports whether current >= minimum.
func VersionAtLeast(current, minimum string) bool {
	return CompareVersions(current, minimum) >= 0
}

func (m *Manager) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, m.Bin, args...)
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "CLICOLOR=0", "TERM=dumb")
	// Never let a spawned helper inherit the server's stdin: on Windows vfox may
	// start a short PowerShell helper and wait for input if stdin stays open.
	cmd.Stdin = strings.NewReader("")
	return cmd
}

func (m *Manager) runOutput(ctx context.Context, args ...string) (string, error) {
	var buf bytes.Buffer
	cmd := m.command(ctx, args...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := StripANSI(buf.String())
	if err != nil {
		return out, wrapCmdErr(out, err)
	}
	return out, nil
}

func wrapCmdErr(output string, err error) error {
	msg := strings.TrimSpace(output)
	if len(msg) > 2000 {
		msg = msg[len(msg)-2000:]
	}
	if msg == "" {
		return err
	}
	return fmt.Errorf("%s", msg)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

type lineWriter struct {
	mu   sync.Mutex
	buf  []byte
	emit func(string)
}

func newLineWriter(emit func(string)) *lineWriter {
	if emit == nil {
		emit = func(string) {}
	}
	return &lineWriter{emit: emit}
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexAny(w.buf, "\r\n")
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx])
		w.buf = w.buf[idx+1:]
		w.deliver(line)
	}
	if len(w.buf) > 32*1024 {
		w.deliver(string(w.buf))
		w.buf = w.buf[:0]
	}
	return len(p), nil
}

func (w *lineWriter) deliver(line string) {
	line = strings.TrimSpace(StripANSI(line))
	if line != "" {
		w.emit(line)
	}
}

// Flush emits any partial trailing line.
func (w *lineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		w.deliver(string(w.buf))
		w.buf = w.buf[:0]
	}
}

var _ io.Writer = (*lineWriter)(nil)
