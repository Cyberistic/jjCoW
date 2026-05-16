package shell

import (
	"strings"
	"testing"
)

func TestGenerateSupportedShells(t *testing.T) {
	for _, sh := range []string{"zsh", "bash", "fish"} {
		t.Run(sh, func(t *testing.T) {
			script, err := Generate(sh)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(script, "config") {
				t.Fatalf("generated %s script does not include config command", sh)
			}
			if !strings.Contains(script, "config get workspace_dir") {
				t.Fatalf("generated %s script does not use config get workspace_dir", sh)
			}
			if strings.Contains(script, "grep \"^workspace_dir:\"") {
				t.Fatalf("generated %s script still parses workspace_dir with grep", sh)
			}
		})
	}
}

func TestGenerateUnsupportedShell(t *testing.T) {
	if _, err := Generate("powershell"); err == nil {
		t.Fatal("expected unsupported shell error")
	}
}
