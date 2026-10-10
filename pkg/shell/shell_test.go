package shell

import (
	"strings"
	"testing"
)

func TestCleanShellContent(t *testing.T) {
	sampleProfile := `# User custom configuration
export EDITOR=nano
export FOO="bar"

# nvsh (node-version-shifter)
export NVSH_DIR="/home/user/.nvsh"
export PATH="$NVSH_DIR/shims:$NVSH_DIR/bin:$PATH"

alias ll="ls -la"
`

	cleaned, modified := CleanShellContent(sampleProfile)
	if !modified {
		t.Fatalf("expected modified to be true, got false")
	}

	if strings.Contains(cleaned, "NVSH_DIR") {
		t.Errorf("cleaned content still contains NVSH_DIR: %s", cleaned)
	}

	if strings.Contains(cleaned, "nvsh (node-version-shifter)") {
		t.Errorf("cleaned content still contains nvsh comment: %s", cleaned)
	}

	if !strings.Contains(cleaned, "export EDITOR=nano") {
		t.Errorf("cleaned content accidentally removed unrelated line: %s", cleaned)
	}

	if !strings.Contains(cleaned, "alias ll=\"ls -la\"") {
		t.Errorf("cleaned content accidentally removed alias: %s", cleaned)
	}

	// Test no-op
	noNvsh := "export A=1\nexport B=2\n"
	cleanedNoOp, modifiedNoOp := CleanShellContent(noNvsh)
	if modifiedNoOp {
		t.Errorf("expected modified to be false for content without nvsh")
	}
	if cleanedNoOp != noNvsh {
		t.Errorf("expected content to be unchanged")
	}
}

func TestGetEnvSnippet(t *testing.T) {
	bashSnippet := GetEnvSnippet("bash")
	if !strings.Contains(bashSnippet, "NVSH_DIR") || !strings.Contains(bashSnippet, "PATH") {
		t.Errorf("unexpected bash snippet: %s", bashSnippet)
	}

	fishSnippet := GetEnvSnippet("fish")
	if !strings.Contains(fishSnippet, "fish_add_path") {
		t.Errorf("unexpected fish snippet: %s", fishSnippet)
	}

	psSnippet := GetEnvSnippet("powershell")
	if !strings.Contains(psSnippet, "$env:NVSH_DIR") {
		t.Errorf("unexpected powershell snippet: %s", psSnippet)
	}
}

func TestUpdateShellPathContent(t *testing.T) {
	initialBash := `# Header
export NVSH_DIR="/home/user/.nvsh"
export PATH="$NVSH_DIR/shims:$NVSH_DIR/bin:$PATH"
`
	// 1. Initial insert of version in PATH
	updated := UpdateShellPathContent(initialBash, "/home/user/.nvsh/versions/v20.18.0/bin", false)
	expectedV20 := `export PATH="$NVSH_DIR/shims:$NVSH_DIR/bin:/home/user/.nvsh/versions/v20.18.0/bin:$PATH"`
	if !strings.Contains(updated, expectedV20) {
		t.Errorf("expected updated PATH to contain v20, got: %s", updated)
	}

	// 2. Update to a new version (must replace old version, keeping only 1 version in PATH)
	updatedAgain := UpdateShellPathContent(updated, "/home/user/.nvsh/versions/v26.0.0/bin", false)
	expectedV26 := `export PATH="$NVSH_DIR/shims:$NVSH_DIR/bin:/home/user/.nvsh/versions/v26.0.0/bin:$PATH"`
	if !strings.Contains(updatedAgain, expectedV26) {
		t.Errorf("expected updated PATH to contain v26, got: %s", updatedAgain)
	}
	if strings.Contains(updatedAgain, "v20.18.0") {
		t.Errorf("old version v20.18.0 still present in PATH: %s", updatedAgain)
	}

	// 3. Revert when all versions are removed
	reverted := UpdateShellPathContent(updatedAgain, "", false)
	expectedReverted := `export PATH="$NVSH_DIR/shims:$NVSH_DIR/bin:$PATH"`
	if !strings.Contains(reverted, expectedReverted) {
		t.Errorf("expected PATH reverted to shims and bin only, got: %s", reverted)
	}
	if strings.Contains(reverted, "versions") {
		t.Errorf("versions directory still in reverted PATH: %s", reverted)
	}

	// 4. Fish syntax
	initialFish := `set -gx NVSH_DIR "/home/user/.nvsh"
fish_add_path "$NVSH_DIR/shims" "$NVSH_DIR/bin"
`
	fishUpdated := UpdateShellPathContent(initialFish, "/home/user/.nvsh/versions/v26.0.0/bin", true)
	expectedFish := `fish_add_path "$NVSH_DIR/shims" "$NVSH_DIR/bin" "/home/user/.nvsh/versions/v26.0.0/bin"`
	if !strings.Contains(fishUpdated, expectedFish) {
		t.Errorf("expected fish_add_path with version, got: %s", fishUpdated)
	}

	fishReverted := UpdateShellPathContent(fishUpdated, "", true)
	expectedFishReverted := `fish_add_path "$NVSH_DIR/shims" "$NVSH_DIR/bin"`
	if !strings.Contains(fishReverted, expectedFishReverted) {
		t.Errorf("expected fish_add_path reverted, got: %s", fishReverted)
	}

	// 5. Strips legacy NODE_HOME lines if present
	withNodeHome := `# Header
export NVSH_DIR="/home/user/.nvsh"
export NODE_HOME="/home/user/.nvsh/versions/v20.18.0"
export PATH="$NVSH_DIR/shims:$NVSH_DIR/bin:$PATH"
`
	cleanedNodeHome := UpdateShellPathContent(withNodeHome, "/home/user/.nvsh/versions/v26.0.0/bin", false)
	if strings.Contains(cleanedNodeHome, "NODE_HOME") {
		t.Errorf("UpdateShellPathContent failed to strip NODE_HOME: %s", cleanedNodeHome)
	}
}

func TestCleanShellContentWithVersionsAndNodeHome(t *testing.T) {
	content := `# User profile
export EDITOR=nano
export NVSH_DIR="/home/user/.nvsh"
export NODE_HOME="/home/user/.nvsh/versions/v22.14.0"
export PATH="$NVSH_DIR/shims:$NVSH_DIR/bin:/home/user/.nvsh/versions/v22.14.0/bin:$PATH"
`
	cleaned, mod := CleanShellContent(content)
	if !mod {
		t.Fatalf("expected modified to be true")
	}
	if strings.Contains(cleaned, "NODE_HOME") || strings.Contains(cleaned, "NVSH_DIR") || strings.Contains(cleaned, "shims") {
		t.Errorf("CleanShellContent failed to remove nvsh entries: %s", cleaned)
	}
	if !strings.Contains(cleaned, "export EDITOR=nano") {
		t.Errorf("CleanShellContent accidentally removed EDITOR: %s", cleaned)
	}
}

