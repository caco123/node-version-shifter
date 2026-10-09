package remote

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fongo/node-version-shifter/pkg/config"
)

// NodeRelease models an item from https://nodejs.org/dist/index.json.
type NodeRelease struct {
	Version string   `json:"version"` // e.g. "v22.14.0"
	Date    string   `json:"date"`
	Files   []string `json:"files"`
	LTS     any      `json:"lts"` // false (bool) or "Jod" / "Iron" (string)
}

// LTSName returns the LTS codename if this release is an LTS version, or empty string.
func (r *NodeRelease) LTSName() string {
	switch v := r.LTS.(type) {
	case string:
		return v
	case bool:
		if v {
			return "LTS"
		}
	}
	return ""
}

// IsLTS returns true if this release has an LTS designation.
func (r *NodeRelease) IsLTS() bool {
	return r.LTSName() != ""
}

const (
	indexCacheFile = "index.json"
	cacheTTL       = 1 * time.Hour
)

// GetReleases retrieves the Node.js releases from cache or by fetching nodejs.org.
func GetReleases(forceRefresh bool) ([]NodeRelease, error) {
	cachePath := filepath.Join(config.CacheDir(), indexCacheFile)

	if !forceRefresh {
		if info, err := os.Stat(cachePath); err == nil {
			if time.Since(info.ModTime()) < cacheTTL {
				data, err := os.ReadFile(cachePath)
				if err == nil {
					var releases []NodeRelease
					if err := json.Unmarshal(data, &releases); err == nil && len(releases) > 0 {
						return releases, nil
					}
				}
			}
		}
	}

	// Fetch remote
	cfg, _ := config.LoadConfig()
	url := fmt.Sprintf("%s/index.json", strings.TrimRight(cfg.NodeDistURL, "/"))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		// Fallback to cache even if expired
		if data, readErr := os.ReadFile(cachePath); readErr == nil {
			var releases []NodeRelease
			if json.Unmarshal(data, &releases) == nil && len(releases) > 0 {
				return releases, nil
			}
		}
		return nil, fmt.Errorf("failed to fetch releases from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error fetching %s: %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var releases []NodeRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("failed to parse index.json: %w", err)
	}

	// Save to cache
	_ = config.InitDirs()
	_ = os.WriteFile(cachePath, body, 0644)

	return releases, nil
}

// ResolveRemoteVersion finds the concrete version tag (e.g. "v22.14.0") from a user query string.
// Supported queries:
// - "lts" or "lts/*" -> latest active LTS
// - "lts/iron" or "iron" -> latest LTS with that codename
// - "latest" or "current" or "node" -> newest release
// - "22" or "v22" -> latest 22.x.x
// - "22.14" -> latest 22.14.x
// - "22.14.0" or "v22.14.0" -> exact version
func ResolveRemoteVersion(query string) (*NodeRelease, error) {
	releases, err := GetReleases(false)
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	clean := strings.TrimSpace(strings.ToLower(query))
	cleanNoV := strings.TrimPrefix(clean, "v")

	// 1. "latest", "current", "node"
	if clean == "latest" || clean == "current" || clean == "node" {
		return &releases[0], nil
	}

	// 2. "lts" or "lts/*"
	if clean == "lts" || clean == "lts/*" {
		for _, r := range releases {
			if r.IsLTS() {
				return &r, nil
			}
		}
		return nil, fmt.Errorf("no LTS release found")
	}

	// 3. "lts/<codename>" or "<codename>"
	if strings.HasPrefix(clean, "lts/") {
		codename := strings.TrimPrefix(clean, "lts/")
		for _, r := range releases {
			if strings.EqualFold(r.LTSName(), codename) {
				return &r, nil
			}
		}
		return nil, fmt.Errorf("no LTS release found with codename '%s'", codename)
	}

	// 4. Exact match (v20.18.0 or 20.18.0)
	exactTag := "v" + cleanNoV
	for _, r := range releases {
		if strings.EqualFold(r.Version, exactTag) {
			return &r, nil
		}
	}

	// 5. Major or minor prefix match (e.g. "20" -> "v20.")
	prefix := "v" + strings.TrimSuffix(cleanNoV, ".x")
	if !strings.HasSuffix(prefix, ".") {
		prefix += "."
	}

	for _, r := range releases {
		if strings.HasPrefix(strings.ToLower(r.Version), prefix) {
			return &r, nil
		}
	}

	// 6. Check if query matches codename directly (e.g. "iron")
	for _, r := range releases {
		if strings.EqualFold(r.LTSName(), clean) {
			return &r, nil
		}
	}

	return nil, fmt.Errorf("no remote Node.js version matched '%s'", query)
}
