package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	AppName    = "nvsh"
	AppVersion = "1.0.0"
)

// Config represents persistent application settings.
type Config struct {
	DefaultVersion string `json:"default_version,omitempty"`
	NodeDistURL    string `json:"node_dist_url,omitempty"`
}

// GetRootDir returns the base directory for nvsh (~/.nvsh by default, or $NVSH_DIR).
func GetRootDir() string {
	if custom := os.Getenv("NVSH_DIR"); custom != "" {
		return custom
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback if home cannot be determined
		home = os.TempDir()
	}
	return filepath.Join(home, ".nvsh")
}

// BinDir returns the directory where the nvsh executable is installed.
func BinDir() string {
	return filepath.Join(GetRootDir(), "bin")
}

// ShimsDir returns the directory containing binary shims (node, npm, npx, corepack).
func ShimsDir() string {
	return filepath.Join(GetRootDir(), "shims")
}

// VersionsDir returns the directory containing installed Node.js versions.
func VersionsDir() string {
	return filepath.Join(GetRootDir(), "versions")
}

// CacheDir returns the directory for temporary downloads and metadata cache.
func CacheDir() string {
	return filepath.Join(GetRootDir(), "cache")
}

// DefaultFile returns the path to the file storing the global default version.
func DefaultFile() string {
	return filepath.Join(GetRootDir(), "default")
}

// ConfigFile returns the path to config.json.
func ConfigFile() string {
	return filepath.Join(GetRootDir(), "config.json")
}

// InitDirs creates the required directory tree under ~/.nvsh.
func InitDirs() error {
	dirs := []string{
		GetRootDir(),
		BinDir(),
		ShimsDir(),
		VersionsDir(),
		CacheDir(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}
	return nil
}

// GetDefaultVersion reads the global default version from default file or config.json.
func GetDefaultVersion() (string, error) {
	// First check the default plain-text file
	data, err := os.ReadFile(DefaultFile())
	if err == nil {
		v := strings.TrimSpace(string(data))
		if v != "" {
			return NormalizeVersion(v), nil
		}
	}

	// Fallback to config.json
	cfg, err := LoadConfig()
	if err == nil && cfg.DefaultVersion != "" {
		return NormalizeVersion(cfg.DefaultVersion), nil
	}

	return "", fmt.Errorf("no default version configured")
}

// SetDefaultVersion persists the global default version.
func SetDefaultVersion(ver string) error {
	if err := InitDirs(); err != nil {
		return err
	}
	normalized := NormalizeVersion(ver)
	if err := os.WriteFile(DefaultFile(), []byte(normalized+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write default version: %w", err)
	}

	cfg, _ := LoadConfig()
	cfg.DefaultVersion = normalized
	return SaveConfig(cfg)
}

// LoadConfig reads config.json or returns default settings.
func LoadConfig() (Config, error) {
	var cfg Config
	cfg.NodeDistURL = "https://nodejs.org/dist"

	data, err := os.ReadFile(ConfigFile())
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.NodeDistURL == "" {
		cfg.NodeDistURL = "https://nodejs.org/dist"
	}
	return cfg, nil
}

// SaveConfig persists config to disk.
func SaveConfig(cfg Config) error {
	if err := InitDirs(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFile(), data, 0644)
}

// NormalizeVersion ensures a version string is formatted consistently (e.g. "20.18.0" -> "v20.18.0").
func NormalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") && isNumericOrSemver(v) {
		return "v" + v
	}
	return v
}

func isNumericOrSemver(s string) bool {
	if len(s) == 0 {
		return false
	}
	return s[0] >= '0' && s[0] <= '9'
}

// GetInstalledVersions returns a sorted list of installed version tags (e.g. ["v18.20.0", "v20.18.0"]).
func GetInstalledVersions() ([]string, error) {
	vDir := VersionsDir()
	entries, err := os.ReadDir(vDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, "v") {
				versions = append(versions, name)
			}
		}
	}

	// Sort versions in ascending semver order
	sort.Slice(versions, func(i, j int) bool {
		return CompareVersions(versions[i], versions[j]) < 0
	})

	return versions, nil
}

// CompareVersions compares two version strings (e.g. "v20.18.0" vs "v22.1.0").
func CompareVersions(v1, v2 string) int {
	clean1 := strings.TrimPrefix(v1, "v")
	clean2 := strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(clean1, ".")
	parts2 := strings.Split(clean2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}
		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}
	return 0
}

// VersionPath returns the directory path for a specific version.
func VersionPath(ver string) string {
	return filepath.Join(VersionsDir(), NormalizeVersion(ver))
}

// VersionInstalled checks whether a version is installed.
func VersionInstalled(ver string) bool {
	info, err := os.Stat(VersionPath(ver))
	return err == nil && info.IsDir()
}

// BinaryPathForVersion returns the target executable path for a command inside an installed version.
// On Windows: <verDir>/<cmd>.exe, <cmd>.cmd, or <verDir>/node.exe for npm-cli.js.
// On Unix: <verDir>/bin/<cmd>.
func BinaryPathForVersion(ver, cmdName string) (string, error) {
	vPath := VersionPath(ver)
	if !VersionInstalled(ver) {
		return "", fmt.Errorf("version %s is not installed", ver)
	}

	if runtime.GOOS == "windows" {
		// 1. Direct .exe
		p := filepath.Join(vPath, cmdName+".exe")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		// 2. Direct name without extension
		p = filepath.Join(vPath, cmdName)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		// 3. .cmd script
		p = filepath.Join(vPath, cmdName+".cmd")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		// 4. .bat script
		p = filepath.Join(vPath, cmdName+".bat")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("command '%s' not found in %s", cmdName, vPath)
	}

	// Unix (Linux, macOS)
	p := filepath.Join(vPath, "bin", cmdName)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("command '%s' not found in %s", cmdName, filepath.Join(vPath, "bin"))
}
