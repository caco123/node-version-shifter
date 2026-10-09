package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fongo/node-version-shifter/pkg/config"
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
