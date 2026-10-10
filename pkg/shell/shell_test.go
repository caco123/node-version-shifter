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

func TestUpdateShellNodeHomeContent(t *testing.T) {
	initialBash := `# Header
export NVSH_DIR="/home/user/.nvsh"
export PATH="$NVSH_DIR/shims:$NVSH_DIR/bin:$PATH"
`
	// 1. Initial insert after NVSH_DIR
	updated := UpdateShellNodeHomeContent(initialBash, "/home/user/.nvsh/versions/v20.18.0", false)
	if !strings.Contains(updated, "export NODE_HOME=\"/home/user/.nvsh/versions/v20.18.0\"") {
		t.Errorf("expected NODE_HOME to be inserted, got: %s", updated)
	}

	// 2. Update to a new version (must replace old version, keeping only 1 definition)
	updatedAgain := UpdateShellNodeHomeContent(updated, "/home/user/.nvsh/versions/v22.14.0", false)
	if !strings.Contains(updatedAgain, "export NODE_HOME=\"/home/user/.nvsh/versions/v22.14.0\"") {
		t.Errorf("expected updated NODE_HOME, got: %s", updatedAgain)
	}
	if strings.Contains(updatedAgain, "v20.18.0") {
		t.Errorf("old version v20.18.0 still present in config: %s", updatedAgain)
	}
	if strings.Count(updatedAgain, "NODE_HOME") != 1 {
		t.Errorf("expected exactly 1 NODE_HOME definition, got count %d: %s", strings.Count(updatedAgain, "NODE_HOME"), updatedAgain)
	}

	// 3. Fish syntax
	initialFish := `set -gx NVSH_DIR "/home/user/.nvsh"
fish_add_path "$NVSH_DIR/shims" "$NVSH_DIR/bin"
`
	fishUpdated := UpdateShellNodeHomeContent(initialFish, "/home/user/.nvsh/versions/v22.14.0", true)
	if !strings.Contains(fishUpdated, "set -gx NODE_HOME \"/home/user/.nvsh/versions/v22.14.0\"") {
		t.Errorf("expected fish set -gx NODE_HOME, got: %s", fishUpdated)
	}
	if strings.Count(fishUpdated, "NODE_HOME") != 1 {
		t.Errorf("expected exactly 1 NODE_HOME in fish config, got: %s", fishUpdated)
	}
}

func TestRemoveShellNodeHomeContent(t *testing.T) {
	content := `# User profile
export NVSH_DIR="/home/user/.nvsh"
export NODE_HOME="/home/user/.nvsh/versions/v22.14.0"
export PATH="$NVSH_DIR/shims:$NVSH_DIR/bin:$PATH"
`
	cleaned, mod := RemoveShellNodeHomeContent(content)
	if !mod {
		t.Fatalf("expected modified to be true")
	}
	if strings.Contains(cleaned, "NODE_HOME") {
		t.Errorf("cleaned content still contains NODE_HOME: %s", cleaned)
	}
	if !strings.Contains(cleaned, "NVSH_DIR") {
		t.Errorf("NVSH_DIR should be retained: %s", cleaned)
	}
}

func TestCleanShellContentWithNodeHome(t *testing.T) {
	content := `# User profile
export EDITOR=nano
export NVSH_DIR="/home/user/.nvsh"
export NODE_HOME="/home/user/.nvsh/versions/v22.14.0"
export PATH="$NVSH_DIR/shims:$NVSH_DIR/bin:$PATH"
`
	cleaned, mod := CleanShellContent(content)
	if !mod {
		t.Fatalf("expected modified to be true")
	}
	if strings.Contains(cleaned, "NODE_HOME") || strings.Contains(cleaned, "NVSH_DIR") {
		t.Errorf("CleanShellContent failed to remove NODE_HOME or NVSH_DIR: %s", cleaned)
	}
	if !strings.Contains(cleaned, "export EDITOR=nano") {
		t.Errorf("CleanShellContent accidentally removed EDITOR: %s", cleaned)
	}
}

