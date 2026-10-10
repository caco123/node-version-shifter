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

// GetEnvSnippet returns the shell-specific snippet to add nvsh shims and bin to PATH.
func GetEnvSnippet(shellName string) string {
	home, _ := os.UserHomeDir()

	switch shellName {
	case "zsh", "bash":
		return fmt.Sprintf("\n# nvsh (node-version-shifter)\nexport NVSH_DIR=\"%s\"\nexport PATH=\"$NVSH_DIR/shims:$NVSH_DIR/bin:$PATH\"\n", filepath.Join(home, ".nvsh"))
	case "fish":
		nvshDir := filepath.Join(home, ".nvsh")
		return fmt.Sprintf("\n# nvsh (node-version-shifter)\nset -gx NVSH_DIR \"%s\"\nfish_add_path \"%s/shims\" \"%s/bin\"\n", nvshDir, nvshDir, nvshDir)
	case "powershell":
		nvshDir := filepath.Join(home, ".nvsh")
		return fmt.Sprintf("\n# nvsh (node-version-shifter)\n$env:NVSH_DIR = \"%s\"\n$env:PATH = \"$env:NVSH_DIR\\shims;$env:NVSH_DIR\\bin;$env:PATH\"\n", nvshDir)
	default:
		return fmt.Sprintf("\n# nvsh\nexport PATH=\"%s:%s:$PATH\"\n", config.ShimsDir(), config.BinDir())
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
	if runtime.GOOS == "windows" {
		if err := removeWindowsUserPath(); err != nil {
			return nil, err
		}
		return []string{"Windows User Environment PATH"}, nil
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
`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	return cmd.Run()
}

