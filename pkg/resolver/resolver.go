package resolver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/caco123/node-version-shifter/pkg/config"
)

// ResolutionResult contains the resolved version tag and where it originated.
type ResolutionResult struct {
	Version string
	Source  string // e.g. "environment NVSH_VERSION", "/path/to/.nvmrc", "global default"
}

// PackageJSON represents the relevant fields of package.json.
type PackageJSON struct {
	Engines struct {
		Node string `json:"node"`
	} `json:"engines"`
}

// ResolveActiveVersion finds the appropriate installed Node.js version for the current working directory.
func ResolveActiveVersion(workDir string) (*ResolutionResult, error) {
	installed, err := config.GetInstalledVersions()
	if err != nil {
		return nil, fmt.Errorf("failed to list installed versions: %w", err)
	}
	if len(installed) == 0 {
		return nil, fmt.Errorf("no Node.js versions are installed. Run 'nvsh install lts' to install one")
	}

	// 1. Check environment variables
	if envVer := os.Getenv("NVSH_VERSION"); envVer != "" {
		matched, err := MatchInstalledVersion(envVer, installed)
		if err != nil {
			return nil, fmt.Errorf("NVSH_VERSION='%s': %w", envVer, err)
		}
		return &ResolutionResult{
			Version: matched,
			Source:  "environment variable NVSH_VERSION",
		}, nil
	}

	if envVer := os.Getenv("NODE_VERSION"); envVer != "" {
		matched, err := MatchInstalledVersion(envVer, installed)
		if err != nil {
			return nil, fmt.Errorf("NODE_VERSION='%s': %w", envVer, err)
		}
		return &ResolutionResult{
			Version: matched,
			Source:  "environment variable NODE_VERSION",
		}, nil
	}

	// 2. Walk directory tree upwards
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	res, err := findVersionInDirectoryTree(workDir, installed)
	if err == nil && res != nil {
		return res, nil
	}

	// 3. Global default
	defVer, err := config.GetDefaultVersion()
	if err == nil && defVer != "" {
		matched, err := MatchInstalledVersion(defVer, installed)
		if err == nil {
			return &ResolutionResult{
				Version: matched,
				Source:  "global default",
			}, nil
		}
	}

	// 4. Fallback to the latest installed version
	latestInstalled := installed[len(installed)-1]
	return &ResolutionResult{
		Version: latestInstalled,
		Source:  "latest installed version (fallback)",
	}, nil
}

func findVersionInDirectoryTree(startDir string, installed []string) (*ResolutionResult, error) {
	curr := filepath.Clean(startDir)

	for {
		// Priority 1: .nvsh-version
		nvshFile := filepath.Join(curr, ".nvsh-version")
		if content, err := os.ReadFile(nvshFile); err == nil {
			raw := strings.TrimSpace(string(content))
			if raw != "" {
				matched, err := MatchInstalledVersion(raw, installed)
				if err != nil {
					return nil, fmt.Errorf("file %s requested '%s': %w", nvshFile, raw, err)
				}
				return &ResolutionResult{
					Version: matched,
					Source:  nvshFile,
				}, nil
			}
		}

		// Priority 2: .nvmrc
		nvmrcFile := filepath.Join(curr, ".nvmrc")
		if content, err := os.ReadFile(nvmrcFile); err == nil {
			raw := strings.TrimSpace(string(content))
			if raw != "" {
				matched, err := MatchInstalledVersion(raw, installed)
				if err != nil {
					return nil, fmt.Errorf("file %s requested '%s': %w", nvmrcFile, raw, err)
				}
				return &ResolutionResult{
					Version: matched,
					Source:  nvmrcFile,
				}, nil
			}
		}

		// Priority 3: .node-version
		nodeVerFile := filepath.Join(curr, ".node-version")
		if content, err := os.ReadFile(nodeVerFile); err == nil {
			raw := strings.TrimSpace(string(content))
			if raw != "" {
				matched, err := MatchInstalledVersion(raw, installed)
				if err != nil {
					return nil, fmt.Errorf("file %s requested '%s': %w", nodeVerFile, raw, err)
				}
				return &ResolutionResult{
					Version: matched,
					Source:  nodeVerFile,
				}, nil
			}
		}

		// Priority 4: package.json (engines.node)
		pkgFile := filepath.Join(curr, "package.json")
		if content, err := os.ReadFile(pkgFile); err == nil {
			var pkg PackageJSON
			if err := json.Unmarshal(content, &pkg); err == nil && pkg.Engines.Node != "" {
				parsed := ParseEnginesNode(pkg.Engines.Node)
				if parsed != "" {
					matched, err := MatchInstalledVersion(parsed, installed)
					if err == nil {
						return &ResolutionResult{
							Version: matched,
							Source:  fmt.Sprintf("%s (engines.node: '%s')", pkgFile, pkg.Engines.Node),
						}, nil
					}
				}
			}
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			// Reached filesystem root
			break
		}
		curr = parent
	}

	return nil, fmt.Errorf("no version config found in tree")
}

// ParseEnginesNode extracts a version hint from an engines.node semver range like ">=20.0.0", "^20.0.0", "20.x".
func ParseEnginesNode(rangeStr string) string {
	rangeStr = strings.TrimSpace(rangeStr)
	// Match numbers like 20, 20.x, 20.10.0
	re := regexp.MustCompile(`(\d+)(\.\d+)?(\.\d+)?`)
	match := re.FindString(rangeStr)
	return match
}

// MatchInstalledVersion finds the best matching installed version for a given pattern.
// Examples:
// - "20" -> picks highest installed v20.x.y
// - "v20" -> picks highest installed v20.x.y
// - "v20.18.0" -> exact match
// - "lts" or "lts/*" -> highest installed version
// - "latest" or "node" -> highest installed version
func MatchInstalledVersion(pattern string, installed []string) (string, error) {
	if len(installed) == 0 {
		return "", fmt.Errorf("no versions installed")
	}

	clean := strings.TrimSpace(pattern)
	clean = strings.TrimPrefix(clean, "v")

	// Special keywords
	lower := strings.ToLower(clean)
	if lower == "lts" || lower == "lts/*" || lower == "latest" || lower == "node" || lower == "default" {
		return installed[len(installed)-1], nil
	}

	// Exact match check
	exactTag := "v" + clean
	for _, v := range installed {
		if v == exactTag {
			return v, nil
		}
	}

	// Partial match (e.g. "20" or "20.18" or "20.x")
	prefixMatch := "v" + strings.TrimSuffix(clean, ".x")
	if !strings.HasSuffix(prefixMatch, ".") {
		prefixMatch += "."
	}

	var candidates []string
	for _, v := range installed {
		if strings.HasPrefix(v, prefixMatch) {
			candidates = append(candidates, v)
		}
	}

	if len(candidates) > 0 {
		// Pick highest candidate
		return candidates[len(candidates)-1], nil
	}

	return "", fmt.Errorf("requested version '%s' is not installed (installed: %s)", pattern, strings.Join(installed, ", "))
}
