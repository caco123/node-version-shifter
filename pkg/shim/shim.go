package shim

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/caco123/node-version-shifter/pkg/config"
	"github.com/caco123/node-version-shifter/pkg/resolver"
)

// DefaultShims are the base binaries guaranteed to be shimmed.
var DefaultShims = []string{"node", "npm", "npx", "corepack"}

// Rehash creates or updates all binary shims in ~/.nvsh/shims without any symlinks.
func Rehash() error {
	if err := config.InitDirs(); err != nil {
		return err
	}

	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to determine nvsh executable path: %w", err)
	}

	// Follow symlinks if the binary itself was run from a symlink to get the real source binary
	if realPath, err := filepath.EvalSymlinks(selfPath); err == nil {
		selfPath = realPath
	}

	// Collect all binary names across standard defaults and installed versions
	allBinaries := make(map[string]bool)
	for _, name := range DefaultShims {
		allBinaries[name] = true
	}

	// Scan installed versions for additional global binaries (e.g. yarn, pnpm, typescript, nodemon)
	versions, _ := config.GetInstalledVersions()
	for _, v := range versions {
		vPath := config.VersionPath(v)
		scanBinariesInVersion(vPath, allBinaries)
	}

	shimsDir := config.ShimsDir()

	for binName := range allBinaries {
		shimName := binName
		if runtime.GOOS == "windows" {
			shimName = binName + ".exe"
		}

		targetShimPath := filepath.Join(shimsDir, shimName)

		// Create shim using hardlink or binary copy (NO symlinks!)
		if err := installShimBinary(selfPath, targetShimPath); err != nil {
			return fmt.Errorf("failed to create shim for %s: %w", binName, err)
		}
	}

	return nil
}

// scanBinariesInVersion looks for executables inside an installed version directory.
func scanBinariesInVersion(vPath string, dest map[string]bool) {
	if runtime.GOOS == "windows" {
		entries, err := os.ReadDir(vPath)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".exe") {
				base := strings.TrimSuffix(lower, ".exe")
				dest[base] = true
			} else if strings.HasSuffix(lower, ".cmd") {
				base := strings.TrimSuffix(lower, ".cmd")
				dest[base] = true
			}
		}
		return
	}

	// Unix: <vPath>/bin/
	binDir := filepath.Join(vPath, "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		// Check if executable
		if info.Mode()&0111 != 0 {
			dest[e.Name()] = true
		}
	}
}

// installShimBinary creates targetShimPath from srcPath using hardlink or file copy (strictly NO symlinks).
func installShimBinary(srcPath, targetShimPath string) error {
	// If the file already exists, check if it's identical
	if srcInfo, err := os.Stat(srcPath); err == nil {
		if dstInfo, err := os.Stat(targetShimPath); err == nil {
			if os.SameFile(srcInfo, dstInfo) {
				return nil // Already the exact hardlinked file
			}
		}
	}

	// Remove old file if it exists
	_ = os.Remove(targetShimPath)

	// Attempt 1: Hardlink (0 disk space, instant, no symlinks, works without admin)
	linkErr := os.Link(srcPath, targetShimPath)
	if linkErr == nil {
		return nil
	}

	// Attempt 2: Direct file copy (failsafe if cross-filesystem or restricted)
	return copyBinary(srcPath, targetShimPath)
}

func copyBinary(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, 0755)
}

// Execute handles shim invocation: resolves the active version and delegates to OS-specific execution.
func Execute(cmdName string, args []string) error {
	// Resolve active version
	res, err := resolver.ResolveActiveVersion("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "nvsh: %v\n", err)
		os.Exit(1)
	}

	targetBin, err := config.BinaryPathForVersion(res.Version, cmdName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nvsh: %v\n", err)
		os.Exit(1)
	}

	// Prepare updated PATH so child processes run binaries from the active version
	vPath := config.VersionPath(res.Version)
	var versionBinDir string
	if runtime.GOOS == "windows" {
		versionBinDir = vPath
	} else {
		versionBinDir = filepath.Join(vPath, "bin")
	}

	currentPath := os.Getenv("PATH")
	var pathSep string
	if runtime.GOOS == "windows" {
		pathSep = ";"
	} else {
		pathSep = ":"
	}
	newPath := versionBinDir + pathSep + currentPath

	// Build environment
	env := os.Environ()
	updatedEnv := make([]string, 0, len(env)+2)
	for _, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			updatedEnv = append(updatedEnv, "PATH="+newPath)
		} else {
			updatedEnv = append(updatedEnv, e)
		}
	}
	updatedEnv = append(updatedEnv, "NVSH_ACTIVE_VERSION="+res.Version)

	return execTarget(targetBin, cmdName, args, updatedEnv, vPath)
}
