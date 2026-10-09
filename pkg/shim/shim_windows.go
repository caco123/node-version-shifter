//go:build windows

package shim

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// execTarget executes target binary on Windows forwarding stdio and exit code.
func execTarget(targetBin, cmdName string, args []string, env []string, versionPath string) error {
	var cmd *exec.Cmd

	lowerTarget := strings.ToLower(targetBin)
	nodeExe := filepath.Join(versionPath, "node.exe")

	// Optimized direct execution for npm, npx, corepack via node.exe
	if cmdName == "npm" {
		npmCli := filepath.Join(versionPath, "node_modules", "npm", "bin", "npm-cli.js")
		if fileExists(npmCli) && fileExists(nodeExe) {
			cmd = exec.Command(nodeExe, append([]string{npmCli}, args...)...)
		}
	} else if cmdName == "npx" {
		npxCli := filepath.Join(versionPath, "node_modules", "npm", "bin", "npx-cli.js")
		if fileExists(npxCli) && fileExists(nodeExe) {
			cmd = exec.Command(nodeExe, append([]string{npxCli}, args...)...)
		}
	} else if cmdName == "corepack" {
		corepackCli := filepath.Join(versionPath, "node_modules", "corepack", "dist", "corepack.js")
		if fileExists(corepackCli) && fileExists(nodeExe) {
			cmd = exec.Command(nodeExe, append([]string{corepackCli}, args...)...)
		}
	}

	if cmd == nil {
		if strings.HasSuffix(lowerTarget, ".cmd") || strings.HasSuffix(lowerTarget, ".bat") {
			cmdArgs := append([]string{"/c", targetBin}, args...)
			cmd = exec.Command("cmd.exe", cmdArgs...)
		} else {
			cmd = exec.Command(targetBin, args...)
		}
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "nvsh: failed to run %s: %v\n", targetBin, err)
		os.Exit(1)
	}

	os.Exit(0)
	return nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
