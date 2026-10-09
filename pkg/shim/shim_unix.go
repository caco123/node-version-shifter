//go:build !windows

package shim

import (
	"fmt"
	"os"
	"syscall"
)

// execTarget replaces current process with target binary via syscall.Exec.
func execTarget(targetBin, cmdName string, args []string, env []string, versionPath string) error {
	argv := append([]string{targetBin}, args...)
	err := syscall.Exec(targetBin, argv, env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nvsh: failed to exec %s: %v\n", targetBin, err)
		os.Exit(1)
	}
	return nil
}
