package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/enver-dev/enver/internal/fsutil"
)

// Data is enver's own application configuration. It never duplicates vfox state.
type Data struct {
	VfoxBin    string `json:"vfoxBin,omitempty"`
	Port       int    `json:"port,omitempty"`
	LastPreset string `json:"lastPreset,omitempty"`
	Locale     string `json:"locale,omitempty"`
}

// Store persists enver configuration as JSON.
type Store struct {
	path string
	mu   sync.RWMutex
	data Data
}

// Dir returns enver's application data directory.
func Dir() (string, error) {
	if v := os.Getenv("ENVER_HOME"); v != "" {
		return v, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "enver"), nil
}

// Load reads the configuration file, creating it when missing.
func Load() (*Store, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.json")
	data, err := fsutil.ReadJSON(path, Data{})
	if err != nil {
		return nil, err
	}
	s := &Store{path: path, data: data}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		_ = fsutil.WriteJSON(path, data)
	}
	return s, nil
}

func (s *Store) Path() string { return s.path }

func (s *Store) Data() Data {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// Update mutates the configuration under lock and persists it.
func (s *Store) Update(fn func(*Data)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.data)
	return fsutil.WriteJSON(s.path, s.data)
}

// CacheDir returns the directory used for catalog and metadata caches.
func CacheDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cache"), nil
}
