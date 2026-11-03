package venv

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coyunai/enver/internal/fsutil"
)

// Record describes a Python virtual environment managed by enver.
type Record struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Path          string    `json:"path"`
	PythonPath    string    `json:"pythonPath"`
	PythonVersion string    `json:"pythonVersion"`
	Tool          string    `json:"tool"`
	ProjectDir    string    `json:"projectDir,omitempty"`
	Preset        string    `json:"preset,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Store persists virtual environment metadata.
type Store struct {
	path  string
	mu    sync.RWMutex
	items []Record
}

func Load(path string) (*Store, error) {
	items, err := fsutil.ReadJSON(path, []Record{})
	if err != nil {
		return nil, err
	}
	return &Store{path: path, items: items}, nil
}

func (s *Store) List() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]Record(nil), s.items...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (s *Store) Get(id string) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.items {
		if r.ID == id {
			return r, true
		}
	}
	return Record{}, false
}

func (s *Store) Add(r *Record) error {
	if r.ID == "" {
		r.ID = newID()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, *r)
	return fsutil.WriteJSON(s.path, s.items)
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.items {
		if r.ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return fsutil.WriteJSON(s.path, s.items)
		}
	}
	return fmt.Errorf("virtual environment %q not found", id)
}

// PythonExe returns the interpreter inside the environment.
func PythonExe(path string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(path, "Scripts", "python.exe")
	}
	return filepath.Join(path, "bin", "python")
}

// FindPython locates the base interpreter inside an installed SDK version.
func FindPython(versionPath string) string {
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

// BinDir returns the directory to prepend to PATH when activating.
func BinDir(path string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(path, "Scripts")
	}
	return filepath.Join(path, "bin")
}

// ReadConfig parses pyvenv.cfg.
func ReadConfig(path string) map[string]string {
	data, err := os.ReadFile(filepath.Join(path, "pyvenv.cfg"))
	if err != nil {
		return nil
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		out[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
	}
	return out
}

// Health reports whether an environment still matches its base interpreter.
func Health(r Record) (bool, string) {
	if _, err := os.Stat(r.Path); err != nil {
		return false, "environment directory is missing"
	}
	cfg := ReadConfig(r.Path)
	if cfg == nil {
		return false, "pyvenv.cfg is missing"
	}
	if _, err := os.Stat(r.PythonPath); err != nil {
		return false, "base interpreter is missing"
	}
	if v, ok := cfg["version"]; ok && r.PythonVersion != "" && !strings.HasPrefix(v, r.PythonVersion) && !strings.HasPrefix(r.PythonVersion, v) {
		return false, fmt.Sprintf("base version changed (%s -> %s)", r.PythonVersion, v)
	}
	if _, err := os.Stat(PythonExe(r.Path)); err != nil {
		return false, "environment interpreter is missing"
	}
	return true, ""
}

// ValidName ensures a friendly display name.
func ValidName(name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && !strings.ContainsAny(name, `/\:*?"<>|`)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("virtual environment not found")
