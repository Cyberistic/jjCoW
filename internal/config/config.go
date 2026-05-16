package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ConfigFileName = ".jjw.yaml"

// IndexConfig contains workspace index configuration.
type IndexConfig struct {
	Max int `yaml:"max"` // Maximum allowed index (0 = no limit)
}

// Config represents the repository-level configuration.
type Config struct {
	Version         int         `yaml:"version"`
	WorkspaceDir    string      `yaml:"workspace_dir"`
	BookmarkPattern string      `yaml:"bookmark_pattern"`
	DefaultBranch   string      `yaml:"default_branch"` // Branch to compare against (e.g. "main")
	RepoDir         string      `yaml:"repo_dir"`       // Subdirectory containing the jj repo (default ".")
	TrackRemote     string      `yaml:"track_remote"`   // Remote to auto-track bookmarks on (e.g. "origin")
	Hooks           HooksConfig `yaml:"hooks"`
	Index           IndexConfig `yaml:"index"`
}

// JJRoot returns the absolute path to the jj repository root.
// This is repoRoot/RepoDir, where repoRoot is the directory containing .jjw.yaml.
func (c *Config) JJRoot(repoRoot string) string {
	return filepath.Join(repoRoot, c.RepoDir)
}

// HooksConfig contains all lifecycle hook configurations.
type HooksConfig struct {
	PreCreate  []HookEntry `yaml:"pre_create"`
	PostCreate []HookEntry `yaml:"post_create"`
	PreDelete  []HookEntry `yaml:"pre_delete"`
	PostDelete []HookEntry `yaml:"post_delete"`
	Info       []HookEntry `yaml:"info"`
}

// HookEntry represents a single hook script configuration.
type HookEntry struct {
	Script string            `yaml:"script"`
	Env    map[string]string `yaml:"env"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Version:         1,
		WorkspaceDir:    "workspaces",
		BookmarkPattern: "{name}",
		DefaultBranch:   "main",
		RepoDir:         ".",
	}
}

// Load reads the configuration from the given repository root.
func Load(repoRoot string) (*Config, error) {
	configPath := filepath.Join(repoRoot, ConfigFileName)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if cfg.WorkspaceDir == "" {
		cfg.WorkspaceDir = "workspaces"
	}
	if cfg.BookmarkPattern == "" {
		cfg.BookmarkPattern = "{name}"
	}
	if cfg.DefaultBranch == "" {
		cfg.DefaultBranch = "main"
	}
	if cfg.RepoDir == "" {
		cfg.RepoDir = "."
	}

	return cfg, nil
}

// Exists checks if a config file exists in the given repository root.
func Exists(repoRoot string) bool {
	_, err := os.Stat(filepath.Join(repoRoot, ConfigFileName))
	return err == nil
}

// GetMainRepoRoot walks up from the current directory looking for .jjw.yaml.
// This works because workspaces are always created as subdirectories of the main repo.
func GetMainRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ConfigFileName)); err == nil {
			resolved, err := filepath.EvalSymlinks(dir)
			if err != nil {
				return dir, nil
			}
			return resolved, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
