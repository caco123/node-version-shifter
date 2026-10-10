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
