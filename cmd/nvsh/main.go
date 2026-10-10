package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/caco123/node-version-shifter/pkg/config"
	"github.com/caco123/node-version-shifter/pkg/downloader"
	"github.com/caco123/node-version-shifter/pkg/remote"
	"github.com/caco123/node-version-shifter/pkg/resolver"
	"github.com/caco123/node-version-shifter/pkg/shell"
	"github.com/caco123/node-version-shifter/pkg/shim"
)

func main() {
	// 1. Detect if running as a Shim (e.g. node, npm, npx, corepack)
	rawName := filepath.Base(os.Args[0])
	cmdName := strings.TrimSuffix(strings.ToLower(rawName), ".exe")

	if cmdName != "nvsh" && cmdName != "node-version-shifter" {
		// Executing as a shim!
		if err := shim.Execute(cmdName, os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "nvsh shim error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 2. Running as CLI management tool
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	subcommand := strings.ToLower(os.Args[1])

	switch subcommand {
	case "install", "i":
		cmdInstall(os.Args[2:])
	case "uninstall", "rm":
		cmdUninstall(os.Args[2:])
	case "use":
		cmdUse(os.Args[2:])
	case "default":
		cmdDefault(os.Args[2:])
	case "ls", "list":
		cmdList()
	case "ls-remote", "list-remote":
		cmdListRemote(os.Args[2:])
	case "current":
		cmdCurrent()
	case "which":
		cmdWhich(os.Args[2:])
	case "exec":
		cmdExec(os.Args[2:])
	case "rehash", "shims":
		cmdRehash()
	case "verify":
		cmdVerify(os.Args[2:])
	case "setup", "init":
		cmdSetup()
	case "self-uninstall":
		cmdSelfUninstall(os.Args[2:])
	case "env":
		cmdEnv()
	case "version", "-v", "--version":
		fmt.Printf("nvsh version %s (%s/%s)\n", config.AppVersion, runtime.GOOS, runtime.GOARCH)
	case "help", "-h", "--help":
		printHelp()
	case "shim": // debug hook to manually test shim execution
		if len(os.Args) < 3 {
			fmt.Println("Usage: nvsh shim <binary-name> [args...]")
			os.Exit(1)
		}
		if err := shim.Execute(os.Args[2], os.Args[3:]); err != nil {
			fmt.Fprintf(os.Stderr, "nvsh shim error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command '%s'. Run 'nvsh help' for usage.\n", subcommand)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Printf(`nvsh - Node.js Version Shifter (v%s)
Fast, cross-platform Node.js version manager.
No administrator privileges needed. Shim mode (no symlinks).

Usage:
  nvsh <command> [arguments]

Commands:
  install <version>     Download and install a Node.js version (e.g. lts, 22, v20.18.0)
  uninstall <version>   Remove an installed Node.js version
  self-uninstall        Completely remove nvsh, all shims, and shell configuration
  use <version>         Set Node.js version for current directory (.node-version)
  default [version]     Get or set the global default version
  current               Display active Node.js version and where it was resolved from
  ls, list              List all locally installed Node.js versions
  ls-remote [filter]    List available remote versions from nodejs.org
  which <command>       Display full path to executable that shim would run
  exec <ver> <cmd...>   Run a command with a specific Node.js version
  verify <version>      Verify SHA-256 checksum and GPG signature of downloaded Node.js
  rehash, shims         Recreate/update all shims in ~/.nvsh/shims (no symlinks)
  setup                 Configure environment and shell profile without admin rights
  env                   Print PATH environment variables for eval "$(nvsh env)"
  version               Display nvsh version

Aliases & Version Formats:
  lts                   Latest Active Long Term Support release
  latest                Latest cutting-edge release
  22, 20                Latest release of major version 22 or 20
  v20.18.0              Exact release

Resolution Hierarchy:
  1. NVSH_VERSION or NODE_VERSION environment variable
  2. .nvsh-version in current or parent directory
  3. .nvmrc in current or parent directory
  4. .node-version in current or parent directory
  5. package.json engines.node in current or parent directory
  6. ~/.nvsh/default global configuration
`, config.AppVersion)
}

func cmdInstall(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: nvsh install <version|lts|latest>")
		os.Exit(1)
	}
	target := args[0]

	// Resolve version from remote
	fmt.Printf("Resolving '%s' from nodejs.org...\n", target)
	release, err := remote.ResolveRemoteVersion(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ltsInfo := ""
	if release.IsLTS() {
		ltsInfo = fmt.Sprintf(" (LTS: %s)", release.LTSName())
	}
	fmt.Printf("Found %s%s\n", release.Version, ltsInfo)

	// Download and extract
	if err := downloader.InstallVersion(release.Version); err != nil {
		fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
		os.Exit(1)
	}

	// Rehash shims
	if err := shim.Rehash(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: shim rehash failed: %v\n", err)
	}

	// If no default version is set yet or current default is not installed, make this version the default
	currDef, defErr := config.GetDefaultVersion()
	if defErr != nil || currDef == "" || !config.VersionInstalled(currDef) {
		if err := config.SetDefaultVersion(release.Version); err == nil {
			fmt.Printf("Set %s as default version.\n", release.Version)
			vPath := config.VersionPath(release.Version)
			if err := shell.SetDefaultVersionInPath(vPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not add version to PATH: %v\n", err)
			} else {
				fmt.Printf("Configured default Node.js in PATH -> %s\n", vPath)
			}
		}
	}
}

func cmdUninstall(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: nvsh uninstall <version>")
		fmt.Println("To completely uninstall nvsh, run: nvsh self-uninstall")
		os.Exit(1)
	}
	if args[0] == "--self" || args[0] == "-s" {
		cmdSelfUninstall(args[1:])
		return
	}
	ver := config.NormalizeVersion(args[0])
	vPath := config.VersionPath(ver)

	if !config.VersionInstalled(ver) {
		fmt.Printf("Version %s is not installed.\n", ver)
		os.Exit(1)
	}

	fmt.Printf("Removing %s...\n", vPath)
	if err := os.RemoveAll(vPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to uninstall: %v\n", err)
		os.Exit(1)
	}

	_ = shim.Rehash()
	fmt.Printf("Uninstalled Node.js %s successfully.\n", ver)

	// Check remaining installed versions
	remaining, _ := config.GetInstalledVersions()
	if len(remaining) == 0 {
		// All Node versions were uninstalled! Remove default and remove version from PATH
		_ = os.Remove(config.DefaultFile())
		cfg, err := config.LoadConfig()
		if err == nil {
			cfg.DefaultVersion = ""
			_ = config.SaveConfig(cfg)
		}
		if err := shell.RemoveVersionFromPath(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not remove version from PATH: %v\n", err)
		} else {
			fmt.Println("No Node.js versions remain. Removed Node.js directory from PATH.")
		}
	} else {
		// Check if the uninstalled version was the default version
		defVer, err := config.GetDefaultVersion()
		if err == nil && defVer == ver {
			// Switch default to the latest remaining version
			newDef := remaining[len(remaining)-1]
			_ = config.SetDefaultVersion(newDef)
			newPath := config.VersionPath(newDef)
			if err := shell.SetDefaultVersionInPath(newPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not update PATH: %v\n", err)
			} else {
				fmt.Printf("Default version updated to %s (PATH updated -> %s)\n", newDef, newPath)
			}
		}
	}
}

func cmdSelfUninstall(args []string) {
	force := false
	keepVersions := false

	for _, arg := range args {
		switch arg {
		case "-y", "--yes", "-f", "--force":
			force = true
		case "--keep-versions":
			keepVersions = true
		}
	}

	rootDir := config.GetRootDir()

	if !force {
		fmt.Printf("This will completely remove nvsh, its shims and configuration from:\n  %s\n", rootDir)
		if keepVersions {
			fmt.Printf("  (Downloaded Node.js versions will be kept in %s)\n", config.VersionsDir())
		} else {
			fmt.Println("  (All installed Node.js versions will also be deleted)")
		}
		fmt.Println()
		fmt.Print("Are you sure you want to continue? [y/N]: ")
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" && input != "s" && input != "si" && input != "sí" {
			fmt.Println("Uninstallation cancelled.")
			return
		}
	}

	fmt.Println("\n==> Cleaning shell environment and PATH...")
	cleanedFiles, err := shell.RemoveShellProfile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not fully clean shell profiles: %v\n", err)
	} else if len(cleanedFiles) > 0 {
		for _, f := range cleanedFiles {
			fmt.Printf("    ✓ Cleaned %s\n", f)
		}
	} else {
		fmt.Println("    ✓ Shell configuration was already clean.")
	}

	if _, err := os.Stat(rootDir); err == nil {
		if keepVersions {
			fmt.Println("==> Removing nvsh binaries, shims, cache and configuration (keeping versions)...")
			_ = os.RemoveAll(config.BinDir())
			_ = os.RemoveAll(config.ShimsDir())
			_ = os.RemoveAll(config.CacheDir())
			_ = os.Remove(config.DefaultFile())
			_ = os.Remove(config.ConfigFile())
			fmt.Printf("    ✓ Retained %s\n", config.VersionsDir())
		} else {
			fmt.Printf("==> Removing %s...\n", rootDir)
			if err := os.RemoveAll(rootDir); err != nil {
				if runtime.GOOS == "windows" {
					fmt.Printf("Notice: Scheduling background cleanup (%v)...\n", err)
					delCmd := fmt.Sprintf("ping 127.0.0.1 -n 2 > nul & rmdir /s /q \"%s\"", rootDir)
					_ = exec.Command("cmd.exe", "/C", delCmd).Start()
				} else {
					fmt.Fprintf(os.Stderr, "Warning: could not remove %s: %v\n", rootDir, err)
				}
			} else {
				fmt.Printf("    ✓ Removed %s\n", rootDir)
			}
		}
	} else {
		fmt.Printf("==> Directory %s not found. Skipping folder removal.\n", rootDir)
	}

	fmt.Println("\n======================================================================")
	fmt.Println("  nvsh has been successfully uninstalled!")
	fmt.Println("======================================================================")
	fmt.Println("\nTo reflect changes, please restart your terminal or shell session.")
}

func cmdUse(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: nvsh use <version>")
		os.Exit(1)
	}

	installed, _ := config.GetInstalledVersions()
	matched, err := resolver.MatchInstalledVersion(args[0], installed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Println("You can install it with: nvsh install " + args[0])
		os.Exit(1)
	}

	// Write to .node-version in current directory
	targetFile := ".node-version"
	if err := os.WriteFile(targetFile, []byte(matched+"\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write %s: %v\n", targetFile, err)
		os.Exit(1)
	}

	fmt.Printf("Using Node.js %s in current directory (written to %s)\n", matched, targetFile)
}

func cmdDefault(args []string) {
	if len(args) == 0 {
		def, err := config.GetDefaultVersion()
		if err != nil {
			fmt.Println("No global default version set.")
		} else {
			fmt.Printf("Global default version: %s\n", def)
			if config.VersionInstalled(def) {
				fmt.Printf("Default path in PATH: %s\n", config.VersionPath(def))
			}
		}
		return
	}

	installed, _ := config.GetInstalledVersions()
	matched, err := resolver.MatchInstalledVersion(args[0], installed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Println("You can install it with: nvsh install " + args[0])
		os.Exit(1)
	}

	if err := config.SetDefaultVersion(matched); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set default version: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Set default Node.js version to %s\n", matched)
	vPath := config.VersionPath(matched)
	if err := shell.SetDefaultVersionInPath(vPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not update PATH: %v\n", err)
	} else {
		fmt.Printf("Updated default Node.js in PATH -> %s\n", vPath)
	}
}

func cmdList() {
	versions, err := config.GetInstalledVersions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(versions) == 0 {
		fmt.Println("No Node.js versions installed.")
		fmt.Println("Install one using: nvsh install lts")
		return
	}

	defVersion, _ := config.GetDefaultVersion()
	res, _ := resolver.ResolveActiveVersion("")

	fmt.Println("Installed Node.js versions:")
	for _, v := range versions {
		prefix := "  "
		suffixes := []string{}

		if res != nil && res.Version == v {
			prefix = "* "
			suffixes = append(suffixes, "active")
		}
		if v == defVersion {
			suffixes = append(suffixes, "default")
		}

		suffixStr := ""
		if len(suffixes) > 0 {
			suffixStr = " (" + strings.Join(suffixes, ", ") + ")"
		}

		fmt.Printf("%s%s%s\n", prefix, v, suffixStr)
	}
}

func cmdListRemote(args []string) {
	filter := ""
	if len(args) > 0 {
		filter = strings.ToLower(args[0])
	}

	fmt.Println("Fetching remote releases...")
	releases, err := remote.GetReleases(false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nAvailable Node.js versions:")
	count := 0
	for _, r := range releases {
		if filter != "" && !strings.Contains(strings.ToLower(r.Version), filter) && !strings.Contains(strings.ToLower(r.LTSName()), filter) {
			continue
		}

		ltsTag := ""
		if r.IsLTS() {
			ltsTag = fmt.Sprintf(" (LTS: %s)", r.LTSName())
		}
		fmt.Printf("  %-12s %-12s%s\n", r.Version, r.Date, ltsTag)

		count++
		if count >= 30 && filter == "" {
			fmt.Printf("\n... showing latest 30 versions. Use 'nvsh ls-remote <filter>' to search.\n")
			break
		}
	}
}

func cmdCurrent() {
	res, err := resolver.ResolveActiveVersion("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s (resolved from %s)\n", res.Version, res.Source)
}

func cmdWhich(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: nvsh which <command>")
		os.Exit(1)
	}
	cmdName := args[0]

	res, err := resolver.ResolveActiveVersion("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	binPath, err := config.BinaryPathForVersion(res.Version, cmdName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Println(binPath)
}

func cmdExec(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: nvsh exec <version> <command> [args...]")
		os.Exit(1)
	}
	verInput := args[0]
	cmdName := args[1]
	cmdArgs := args[2:]

	installed, _ := config.GetInstalledVersions()
	matched, err := resolver.MatchInstalledVersion(verInput, installed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	binPath, err := config.BinaryPathForVersion(matched, cmdName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	vPath := config.VersionPath(matched)
	var binDir string
	if runtime.GOOS == "windows" {
		binDir = vPath
	} else {
		binDir = filepath.Join(vPath, "bin")
	}

	pathSep := ":"
	if runtime.GOOS == "windows" {
		pathSep = ";"
	}
	newPath := binDir + pathSep + os.Getenv("PATH")

	cmd := exec.Command(binPath, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			env[i] = "PATH=" + newPath
		}
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

func cmdVerify(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: nvsh verify <version>")
		os.Exit(1)
	}

	target := args[0]
	release, err := remote.ResolveRemoteVersion(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving version: %v\n", err)
		os.Exit(1)
	}

	if err := downloader.VerifyVersion(release.Version); err != nil {
		fmt.Fprintf(os.Stderr, "Verification failed: %v\n", err)
		os.Exit(1)
	}
}

func cmdRehash() {
	fmt.Printf("Rehashing shims in %s (no symlinks)...\n", config.ShimsDir())
	if err := shim.Rehash(); err != nil {
		fmt.Fprintf(os.Stderr, "Rehash error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Shims updated successfully.")
}

func cmdSetup() {
	fmt.Println("Setting up nvsh (user-space, zero administrator privileges)...")

	if err := config.InitDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "Directory initialization error: %v\n", err)
		os.Exit(1)
	}

	// Copy nvsh binary to ~/.nvsh/bin/nvsh
	selfPath, err := os.Executable()
	if err == nil {
		if realPath, err := filepath.EvalSymlinks(selfPath); err == nil {
			selfPath = realPath
		}
		binName := "nvsh"
		if runtime.GOOS == "windows" {
			binName = "nvsh.exe"
		}
		dest := filepath.Join(config.BinDir(), binName)
		_ = copyBinary(selfPath, dest)
	}

	// Create shims
	if err := shim.Rehash(); err != nil {
		fmt.Fprintf(os.Stderr, "Shim setup error: %v\n", err)
	}

	// Update shell profile
	msg, err := shell.SetupShellProfile()
	if err != nil {
		fmt.Printf("Could not auto-configure shell: %v\n", err)
	} else {
		fmt.Println(msg)
	}

	fmt.Println("\nSetup complete! Ensure the following are in your PATH:")
	fmt.Printf("  %s\n", config.ShimsDir())
	fmt.Printf("  %s\n", config.BinDir())
}

func cmdEnv() {
	sh := shell.DetectShell()
	fmt.Print(shell.GetEnvSnippet(sh))
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

	_, err = io.Copy(out, in)
	return err
}
