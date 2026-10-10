package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/caco123/node-version-shifter/pkg/config"
)

// DetectShell attempts to guess the current user's shell.
func DetectShell() string {
	if runtime.GOOS == "windows" {
		if os.Getenv("PSModulePath") != "" {
			return "powershell"
		}
		return "cmd"
	}

	shellPath := os.Getenv("SHELL")
	if shellPath != "" {
		base := filepath.Base(shellPath)
		return base
	}

	return "bash"
}

// GetEnvSnippet returns the shell-specific snippet to add nvsh shims and bin to PATH (and NODE_HOME if a default version is installed).
func GetEnvSnippet(shellName string) string {
	home, _ := os.UserHomeDir()
	nvshDir := filepath.Join(home, ".nvsh")

	var nodeHomeExport string
	def, err := config.GetDefaultVersion()
	if err == nil && config.VersionInstalled(def) {
		vPath := config.VersionPath(def)
		switch shellName {
		case "zsh", "bash":
			nodeHomeExport = fmt.Sprintf("export NODE_HOME=\"%s\"\n", vPath)
		case "fish":
			nodeHomeExport = fmt.Sprintf("set -gx NODE_HOME \"%s\"\n", vPath)
		case "powershell":
			nodeHomeExport = fmt.Sprintf("$env:NODE_HOME = \"%s\"\n", vPath)
		default:
			nodeHomeExport = fmt.Sprintf("export NODE_HOME=\"%s\"\n", vPath)
		}
	}

	switch shellName {
	case "zsh", "bash":
		return fmt.Sprintf("\n# nvsh (node-version-shifter)\nexport NVSH_DIR=\"%s\"\n%sexport PATH=\"$NVSH_DIR/shims:$NVSH_DIR/bin:$PATH\"\n", nvshDir, nodeHomeExport)
	case "fish":
		return fmt.Sprintf("\n# nvsh (node-version-shifter)\nset -gx NVSH_DIR \"%s\"\n%sfish_add_path \"%s/shims\" \"%s/bin\"\n", nvshDir, nodeHomeExport, nvshDir, nvshDir)
	case "powershell":
		return fmt.Sprintf("\n# nvsh (node-version-shifter)\n$env:NVSH_DIR = \"%s\"\n%s$env:PATH = \"$env:NVSH_DIR\\shims;$env:NVSH_DIR\\bin;$env:PATH\"\n", nvshDir, nodeHomeExport)
	default:
		return fmt.Sprintf("\n# nvsh\n%sexport PATH=\"%s:%s:$PATH\"\n", nodeHomeExport, config.ShimsDir(), config.BinDir())
	}
}

// SetupShellProfile automatically writes the environment snippet to the appropriate user shell config.
func SetupShellProfile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	shellName := DetectShell()

	if runtime.GOOS == "windows" {
		// Update user-level PATH in Windows Registry (no admin required!)
		err := updateWindowsUserPath()
		if err == nil {
			return "Windows User Environment PATH updated successfully", nil
		}
		// If automated registry update fails, fallback to profile info
		return "Please add %USERPROFILE%\\.nvsh\\shims and %USERPROFILE%\\.nvsh\\bin to your User PATH", nil
	}

	var targetFile string
	switch shellName {
	case "zsh":
		targetFile = filepath.Join(home, ".zshrc")
	case "fish":
		targetFile = filepath.Join(home, ".config", "fish", "config.fish")
	case "bash":
		targetFile = filepath.Join(home, ".bashrc")
		if runtime.GOOS == "darwin" {
			// On macOS, bash uses .bash_profile for login shells
			bp := filepath.Join(home, ".bash_profile")
			if _, err := os.Stat(bp); err == nil {
				targetFile = bp
			}
		}
	default:
		targetFile = filepath.Join(home, ".profile")
	}

	snippet := GetEnvSnippet(shellName)

	// Check if already present
	content, _ := os.ReadFile(targetFile)
	if strings.Contains(string(content), "NVSH_DIR") || strings.Contains(string(content), "node-version-shifter") {
		return fmt.Sprintf("nvsh is already configured in %s", targetFile), nil
	}

	// Create directory if missing
	_ = os.MkdirAll(filepath.Dir(targetFile), 0755)

	f, err := os.OpenFile(targetFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("could not write to %s: %w", targetFile, err)
	}
	defer f.Close()

	if _, err := f.WriteString(snippet); err != nil {
		return "", err
	}

	return fmt.Sprintf("Configured %s. Run 'source %s' or restart your terminal to apply.", targetFile, targetFile), nil
}

func updateWindowsUserPath() error {
	shims := config.ShimsDir()
	bin := config.BinDir()

	// PowerShell script to append to User PATH without admin rights
	psCmd := fmt.Sprintf(`
$oldPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($oldPath -notlike "*%s*") {
    $newPath = "%s;%s;" + $oldPath
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
}
`, shims, shims, bin)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	return cmd.Run()
}

// CleanShellContent removes lines associated with nvsh from shell config content.
func CleanShellContent(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	var result []string
	modified := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(line, "nvsh (node-version-shifter)") ||
			strings.Contains(line, "NVSH_DIR") ||
			strings.Contains(line, "NODE_HOME") ||
			(strings.Contains(line, ".nvsh/shims") && strings.Contains(line, ".nvsh/bin")) ||
			(strings.Contains(line, "fish_add_path") && strings.Contains(line, "shims") && strings.Contains(line, "bin")) ||
			trimmed == "# nvsh" {
			modified = true
			continue
		}
		result = append(result, line)
	}

	if !modified {
		return content, false
	}

	cleaned := strings.Join(result, "\n")
	return cleaned, true
}

// RemoveShellProfile cleans up nvsh configurations from shell rc files and environment.
func RemoveShellProfile() ([]string, error) {
	_ = RemoveNodeHome()

	if runtime.GOOS == "windows" {
		if err := removeWindowsUserPath(); err != nil {
			return nil, err
		}
		return []string{"Windows User Environment PATH and variables"}, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	candidateFiles := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".profile"),
		filepath.Join(home, ".config", "fish", "config.fish"),
	}

	var cleaned []string
	for _, f := range candidateFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		newContent, mod := CleanShellContent(string(data))
		if mod {
			if err := os.WriteFile(f, []byte(newContent), 0644); err == nil {
				cleaned = append(cleaned, f)
			}
		}
	}

	return cleaned, nil
}

func removeWindowsUserPath() error {
	psCmd := `
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath) {
    $parts = $userPath -split ';' | Where-Object {
        $trimmed = $_.Trim()
        $trimmed -ne "" -and ($trimmed -notlike "*\.nvsh*") -and ($trimmed -notlike "*node-version-shifter*")
    }
    $newPath = $parts -join ';'
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
}
if ([Environment]::GetEnvironmentVariable("NVSH_DIR", "User")) {
    [Environment]::SetEnvironmentVariable("NVSH_DIR", $null, "User")
}
if ([Environment]::GetEnvironmentVariable("NODE_HOME", "User")) {
    [Environment]::SetEnvironmentVariable("NODE_HOME", $null, "User")
}
`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	return cmd.Run()
}

// SetDefaultNodeHome sets the NODE_HOME environment variable to the specified path
// in the Windows user registry or Unix shell configuration profiles.
// Only one NODE_HOME variable exists and points to the global default version.
func SetDefaultNodeHome(nodePath string) error {
	_ = os.Setenv("NODE_HOME", nodePath)

	if runtime.GOOS == "windows" {
		return setWindowsUserNodeHome(nodePath)
	}

	return setUnixShellNodeHome(nodePath)
}

// RemoveNodeHome removes the NODE_HOME environment variable from the Windows
// user registry or Unix shell configuration profiles.
func RemoveNodeHome() error {
	_ = os.Unsetenv("NODE_HOME")

	if runtime.GOOS == "windows" {
		return removeWindowsUserNodeHome()
	}

	return removeUnixShellNodeHome()
}

func setWindowsUserNodeHome(nodePath string) error {
	cleanPath := strings.ReplaceAll(nodePath, "'", "''")
	psCmd := fmt.Sprintf("[Environment]::SetEnvironmentVariable('NODE_HOME', '%s', 'User')", cleanPath)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	return cmd.Run()
}

func removeWindowsUserNodeHome() error {
	psCmd := `[Environment]::SetEnvironmentVariable("NODE_HOME", $null, "User")`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	return cmd.Run()
}

// UpdateShellNodeHomeContent updates or inserts the NODE_HOME export in shell configuration content.
// It ensures that only ONE NODE_HOME variable definition exists in the file.
func UpdateShellNodeHomeContent(content, nodePath string, isFish bool) string {
	lines := strings.Split(content, "\n")
	var result []string
	found := false

	var targetPrefix string
	var newLine string
	if isFish {
		targetPrefix = "set -gx NODE_HOME"
		newLine = fmt.Sprintf("set -gx NODE_HOME \"%s\"", nodePath)
	} else {
		targetPrefix = "export NODE_HOME="
		newLine = fmt.Sprintf("export NODE_HOME=\"%s\"", nodePath)
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, targetPrefix) || trimmed == "export NODE_HOME" {
			if !found {
				result = append(result, newLine)
				found = true
			}
			// If duplicate NODE_HOME definitions exist, remove them so only one exists
			continue
		}
		result = append(result, line)
	}

	if found {
		return strings.Join(result, "\n")
	}

	// If not found, try to insert right after NVSH_DIR definition
	var finalResult []string
	inserted := false
	nvshPrefix := "export NVSH_DIR="
	if isFish {
		nvshPrefix = "set -gx NVSH_DIR"
	}

	for _, line := range result {
		finalResult = append(finalResult, line)
		trimmed := strings.TrimSpace(line)
		if !inserted && strings.HasPrefix(trimmed, nvshPrefix) {
			finalResult = append(finalResult, newLine)
			inserted = true
		}
	}

	if inserted {
		return strings.Join(finalResult, "\n")
	}

	// Otherwise, append to end
	if len(result) > 0 && result[len(result)-1] != "" {
		result = append(result, "")
	}
	result = append(result, newLine)
	return strings.Join(result, "\n")
}

// RemoveShellNodeHomeContent removes any NODE_HOME lines from shell configuration content.
func RemoveShellNodeHomeContent(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	var result []string
	modified := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "export NODE_HOME=") ||
			strings.HasPrefix(trimmed, "set -gx NODE_HOME") ||
			trimmed == "export NODE_HOME" {
			modified = true
			continue
		}
		result = append(result, line)
	}

	if !modified {
		return content, false
	}
	return strings.Join(result, "\n"), true
}

func setUnixShellNodeHome(nodePath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	candidateFiles := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".profile"),
		filepath.Join(home, ".config", "fish", "config.fish"),
	}

	updatedCount := 0
	for _, f := range candidateFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "NVSH_DIR") || strings.Contains(content, "node-version-shifter") || strings.Contains(content, "NODE_HOME") {
			isFish := strings.HasSuffix(f, "config.fish")
			newContent := UpdateShellNodeHomeContent(content, nodePath, isFish)
			if newContent != content {
				_ = os.WriteFile(f, []byte(newContent), 0644)
				updatedCount++
			}
		}
	}

	if updatedCount == 0 {
		shellName := DetectShell()
		var primaryFile string
		switch shellName {
		case "zsh":
			primaryFile = filepath.Join(home, ".zshrc")
		case "fish":
			primaryFile = filepath.Join(home, ".config", "fish", "config.fish")
		case "bash":
			primaryFile = filepath.Join(home, ".bashrc")
			if runtime.GOOS == "darwin" {
				bp := filepath.Join(home, ".bash_profile")
				if _, err := os.Stat(bp); err == nil {
					primaryFile = bp
				}
			}
		default:
			primaryFile = filepath.Join(home, ".profile")
		}

		data, _ := os.ReadFile(primaryFile)
		isFish := shellName == "fish"
		newContent := UpdateShellNodeHomeContent(string(data), nodePath, isFish)
		_ = os.MkdirAll(filepath.Dir(primaryFile), 0755)
		_ = os.WriteFile(primaryFile, []byte(newContent), 0644)
	}

	return nil
}

func removeUnixShellNodeHome() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	candidateFiles := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".profile"),
		filepath.Join(home, ".config", "fish", "config.fish"),
	}

	for _, f := range candidateFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		newContent, mod := RemoveShellNodeHomeContent(string(data))
		if mod {
			_ = os.WriteFile(f, []byte(newContent), 0644)
		}
	}
	return nil
}

