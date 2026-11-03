package catalog

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/coyunai/enver/internal/fsutil"
)

const DefaultURL = "https://version-fox.github.io/vfox-plugins/index.json"

//go:embed fallback.json
var fallbackData []byte

// Plugin is one entry in the vfox plugin registry.
type Plugin struct {
	Name      string `json:"name"`
	Desc      string `json:"desc"`
	Homepage  string `json:"homepage"`
	Installed bool   `json:"installed"`
}

type cacheFile struct {
	FetchedAt time.Time `json:"fetchedAt"`
	Items     []Plugin  `json:"items"`
}

// Load returns the plugin catalog. It prefers a fresh cache, then the network,
// then a stale cache, and finally the embedded snapshot.
func Load(ctx context.Context, cachePath string, maxAge time.Duration) (items []Plugin, source string) {
	if cached, err := fsutil.ReadJSON(cachePath, cacheFile{}); err == nil && len(cached.Items) > 0 {
		if time.Since(cached.FetchedAt) < maxAge {
			return cached.Items, "cache"
		}
	}
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DefaultURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "enver")
		if resp, doErr := client.Do(req); doErr == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var fetched []Plugin
				if json.NewDecoder(resp.Body).Decode(&fetched) == nil && len(fetched) > 0 {
					_ = fsutil.WriteJSON(cachePath, cacheFile{FetchedAt: time.Now(), Items: fetched})
					return fetched, "network"
				}
			}
		}
	}
	if cached, err := fsutil.ReadJSON(cachePath, cacheFile{}); err == nil && len(cached.Items) > 0 {
		return cached.Items, "cache-stale"
	}
	var embedded []Plugin
	if err := json.Unmarshal(fallbackData, &embedded); err == nil && len(embedded) > 0 {
		return embedded, "embedded"
	}
	return nil, "empty"
}

// Search filters by name, description or homepage.
func Search(items []Plugin, query string) []Plugin {
	if query == "" {
		return items
	}
	out := make([]Plugin, 0, len(items))
	q := strings.ToLower(query)
	for _, p := range items {
		if strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Desc), q) ||
			strings.Contains(strings.ToLower(p.Homepage), q) {
			out = append(out, p)
		}
	}
	return out
}

// Validate checks that the embedded fallback is well formed.
func Validate() error {
	var items []Plugin
	if err := json.Unmarshal(fallbackData, &items); err != nil {
		return fmt.Errorf("catalog fallback: %w", err)
	}
	if len(items) == 0 {
		return errors.New("catalog fallback is empty")
	}
	return nil
}

// CachePath is a convenience helper for callers.
func CachePath(dir string) string {
	return filepath.Join(dir, "catalog.json")
}
