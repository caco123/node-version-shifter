package resolver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchInstalledVersion(t *testing.T) {
	installed := []string{"v18.20.5", "v20.17.0", "v20.18.0", "v22.14.0"}

	tests := []struct {
		pattern  string
		expected string
		wantErr  bool
	}{
		{"20", "v20.18.0", false},
		{"v20", "v20.18.0", false},
		{"20.17", "v20.17.0", false},
		{"20.18.0", "v20.18.0", false},
		{"v20.18.0", "v20.18.0", false},
		{"18", "v18.20.5", false},
		{"lts", "v22.14.0", false},
		{"latest", "v22.14.0", false},
		{"node", "v22.14.0", false},
		{"21", "", true}, // not installed
	}

	for _, tc := range tests {
		got, err := MatchInstalledVersion(tc.pattern, installed)
		if tc.wantErr {
			if err == nil {
				t.Errorf("MatchInstalledVersion(%q) expected error, got %q", tc.pattern, got)
			}
		} else {
			if err != nil {
				t.Errorf("MatchInstalledVersion(%q) unexpected error: %v", tc.pattern, err)
			}
			if got != tc.expected {
				t.Errorf("MatchInstalledVersion(%q) = %q; expected %q", tc.pattern, got, tc.expected)
			}
		}
	}
}

func TestParseEnginesNode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{">=20.0.0", "20.0.0"},
		{"^20.10.0", "20.10.0"},
		{">= 18.0.0 < 21.0.0", "18.0.0"},
		{"22.x", "22"},
		{"v20", "20"},
	}

	for _, tc := range tests {
		got := ParseEnginesNode(tc.input)
		if got != tc.expected {
			t.Errorf("ParseEnginesNode(%q) = %q; expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestDirectoryTreeWalking(t *testing.T) {
	tempRoot := t.TempDir()
	subDir := filepath.Join(tempRoot, "sub", "project")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Place .nvmrc in tempRoot
	nvmrcPath := filepath.Join(tempRoot, ".nvmrc")
	if err := os.WriteFile(nvmrcPath, []byte("v20\n"), 0644); err != nil {
		t.Fatal(err)
	}

	installed := []string{"v18.0.0", "v20.18.0", "v22.0.0"}
	res, err := findVersionInDirectoryTree(subDir, installed)
	if err != nil {
		t.Fatalf("findVersionInDirectoryTree failed: %v", err)
	}

	if res.Version != "v20.18.0" {
		t.Errorf("expected v20.18.0, got %s", res.Version)
	}
	if res.Source != nvmrcPath {
		t.Errorf("expected source %s, got %s", nvmrcPath, res.Source)
	}

	// Override with .node-version in subDir
	nodeVerFile := filepath.Join(subDir, ".node-version")
	_ = os.WriteFile(nodeVerFile, []byte("18\n"), 0644)

	res2, err := findVersionInDirectoryTree(subDir, installed)
	if err != nil {
		t.Fatalf("findVersionInDirectoryTree failed: %v", err)
	}
	if res2.Version != "v18.0.0" {
		t.Errorf("expected v18.0.0, got %s", res2.Version)
	}
}
