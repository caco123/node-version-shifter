package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"20.18.0", "v20.18.0"},
		{"v20.18.0", "v20.18.0"},
		{"22", "v22"},
		{"v22", "v22"},
		{"lts", "lts"},
		{"latest", "latest"},
		{"", ""},
	}

	for _, tc := range tests {
		got := NormalizeVersion(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeVersion(%q) = %q; expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int // -1, 0, 1
	}{
		{"v18.0.0", "v20.0.0", -1},
		{"v20.0.0", "v18.0.0", 1},
		{"v20.18.0", "v20.18.0", 0},
		{"v20.9.0", "v20.18.0", -1},
		{"v22.1.0", "v22.0.1", 1},
	}

	for _, tc := range tests {
		got := CompareVersions(tc.v1, tc.v2)
		if got != tc.expected {
			t.Errorf("CompareVersions(%q, %q) = %d; expected %d", tc.v1, tc.v2, got, tc.expected)
		}
	}
}

func TestDirectoriesAndDefaultVersion(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("NVSH_DIR", tempDir)

	if err := InitDirs(); err != nil {
		t.Fatalf("InitDirs failed: %v", err)
	}

	// Verify folders exist
	for _, p := range []string{BinDir(), ShimsDir(), VersionsDir(), CacheDir()} {
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			t.Errorf("expected directory %s to exist", p)
		}
	}

	// Test default version set and get
	if err := SetDefaultVersion("20.18.0"); err != nil {
		t.Fatalf("SetDefaultVersion failed: %v", err)
	}

	def, err := GetDefaultVersion()
	if err != nil {
		t.Fatalf("GetDefaultVersion failed: %v", err)
	}
	if def != "v20.18.0" {
		t.Errorf("expected v20.18.0, got %s", def)
	}

	// Test installed versions listing
	fakeV20 := filepath.Join(VersionsDir(), "v20.18.0")
	fakeV22 := filepath.Join(VersionsDir(), "v22.1.0")
	_ = os.Mkdir(fakeV20, 0755)
	_ = os.Mkdir(fakeV22, 0755)

	installed, err := GetInstalledVersions()
	if err != nil {
		t.Fatalf("GetInstalledVersions failed: %v", err)
	}
	if len(installed) != 2 || installed[0] != "v20.18.0" || installed[1] != "v22.1.0" {
		t.Errorf("unexpected installed versions: %v", installed)
	}
}
