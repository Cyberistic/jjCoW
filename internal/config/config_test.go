package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Version != 1 || cfg.WorkspaceDir != "workspaces" || cfg.BookmarkPattern != "{name}" || cfg.DefaultBranch != "main" || cfg.RepoDir != "." {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkspaceDir != "workspaces" || cfg.BookmarkPattern != "{name}" || cfg.DefaultBranch != "main" || cfg.RepoDir != "." {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
}

func TestLoadExplicitValues(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`version: 1
workspace_dir: ws
bookmark_pattern: "user/{name}"
default_branch: trunk
repo_dir: src
track_remote: origin
index:
  max: 4
`)
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkspaceDir != "ws" || cfg.BookmarkPattern != "user/{name}" || cfg.DefaultBranch != "trunk" || cfg.RepoDir != "src" || cfg.TrackRemote != "origin" || cfg.Index.Max != 4 {
		t.Fatalf("explicit values not loaded: %+v", cfg)
	}
	if got := cfg.JJRoot(dir); got != filepath.Join(dir, "src") {
		t.Fatalf("JJRoot = %q", got)
	}
}

func TestGetMainRepoRoot(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	got, err := GetMainRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(dir)
	if got != want {
		t.Fatalf("GetMainRepoRoot = %q, want %q", got, want)
	}
}
