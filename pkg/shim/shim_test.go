package shim

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallShimBinaryNoSymlinks(t *testing.T) {
	tempDir := t.TempDir()
	srcBinary := filepath.Join(tempDir, "mock_nvsh")
	targetShim := filepath.Join(tempDir, "node")

	// Create a mock executable source file
	if err := os.WriteFile(srcBinary, []byte("#!/bin/sh\necho mock\n"), 0755); err != nil {
		t.Fatalf("failed to create mock source: %v", err)
	}

	// Install shim
	if err := installShimBinary(srcBinary, targetShim); err != nil {
		t.Fatalf("installShimBinary failed: %v", err)
	}

	// Verify target exists
	info, err := os.Lstat(targetShim)
	if err != nil {
		t.Fatalf("failed to stat target shim: %v", err)
	}

	// CRITICAL TEST: MUST NOT BE A SYMLINK
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("SHIM MUST NOT BE A SYMLINK! Mode was: %v", info.Mode())
	}

	// Verify permissions
	if info.Mode().Perm()&0111 == 0 {
		t.Errorf("Shim must be executable! Mode was: %v", info.Mode())
	}

	// Verify content matches
	content, err := os.ReadFile(targetShim)
	if err != nil {
		t.Fatalf("failed to read shim: %v", err)
	}
	if string(content) != "#!/bin/sh\necho mock\n" {
		t.Errorf("Shim content mismatch: %s", string(content))
	}
}
