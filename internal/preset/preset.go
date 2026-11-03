package preset

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coyunai/enver/internal/fsutil"
)

// PythonOptions configures the Python isolation attached to a preset.
type PythonOptions struct {
	Venv     string `json:"venv,omitempty"`
	Tool     string `json:"tool,omitempty"`
	AutoSync bool   `json:"autoSync,omitempty"`
}

// Preset is a named, switchable environment composition.
type Preset struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Runtimes    map[string]string   `json:"runtimes"`
	Env         map[string]string   `json:"env,omitempty"`
	Python      *PythonOptions      `json:"python,omitempty"`
	Packages    map[string][]string `json:"packages,omitempty"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Validate normalizes and checks a preset.
func Validate(p *Preset) error {
	p.Name = strings.TrimSpace(p.Name)
	if !namePattern.MatchString(p.Name) {
		return errors.New("preset name must start with a letter or digit and contain only letters, digits, dot, dash or underscore")
	}
	if len(p.Runtimes) == 0 {
		return errors.New("a preset needs at least one runtime")
	}
	normalized := make(map[string]string, len(p.Runtimes))
	for sdk, version := range p.Runtimes {
		sdk = strings.TrimSpace(sdk)
		version = strings.TrimSpace(version)
		if sdk == "" || version == "" {
			return errors.New("runtime entries need both SDK and version")
		}
		if strings.ContainsAny(sdk+version, " \t\n") {
			return fmt.Errorf("invalid runtime entry %q@%q", sdk, version)
		}
		normalized[sdk] = version
	}
	p.Runtimes = normalized
	return nil
}

// Store persists presets as JSON.
type Store struct {
	path  string
	mu    sync.RWMutex
	items map[string]Preset
}

func Load(path string) (*Store, error) {
	items, err := fsutil.ReadJSON(path, map[string]Preset{})
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = map[string]Preset{}
	}
	return &Store{path: path, items: items}, nil
}

func (s *Store) List() []Preset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Preset, 0, len(s.items))
	for _, p := range s.items {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) Get(name string) (Preset, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.items[name]
	return p, ok
}

func (s *Store) Save(p Preset) error {
	if err := Validate(&p); err != nil {
		return err
	}
	p.UpdatedAt = time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[p.Name] = p
	return fsutil.WriteJSON(s.path, s.items)
}

func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[name]; !ok {
		return fmt.Errorf("preset %q not found", name)
	}
	delete(s.items, name)
	return fsutil.WriteJSON(s.path, s.items)
}
