package hooks

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/Cyberistic/jjCoW/internal/config"
)

// Env contains environment variables passed to hooks.
type Env struct {
	Name         string
	Path         string
	Bookmark     string
	RepoRoot     string
	JJRoot       string
	WorkspaceDir string
	Index        int
}

// ToEnvVars converts the Env struct to environment variable format.
func (e *Env) ToEnvVars() []string {
	vars := []string{
		"JJW_NAME=" + e.Name,
		"JJW_PATH=" + e.Path,
		"JJW_BOOKMARK=" + e.Bookmark,
		"JJW_REPO_ROOT=" + e.RepoRoot,
		"JJW_JJ_ROOT=" + e.JJRoot,
		"JJW_WORKSPACE_DIR=" + e.WorkspaceDir,
	}
	if e.Index > 0 {
		vars = append(vars, "JJW_INDEX="+strconv.Itoa(e.Index))
	}
	return vars
}

// Run executes a list of hook entries.
func Run(entries []config.HookEntry, env *Env, workDir string) error {
	for _, entry := range entries {
		if err := runHook(entry, env, workDir); err != nil {
			return err
		}
	}
	return nil
}

func runHook(entry config.HookEntry, env *Env, workDir string) error {
	scriptPath := entry.Script
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Join(env.RepoRoot, scriptPath)
	}

	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("hook script not found: %s", scriptPath)
	}

	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env.ToEnvVars()...)

	for k, v := range entry.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	return cmd.Run()
}

// RunPreCreate runs pre-create hooks.
func RunPreCreate(cfg *config.Config, env *Env) error {
	if len(cfg.Hooks.PreCreate) == 0 {
		return nil
	}
	fmt.Println("Running pre-create hooks...")
	_ = os.Stdout.Sync()
	return Run(cfg.Hooks.PreCreate, env, env.RepoRoot)
}

// RunPostCreate runs post-create hooks.
func RunPostCreate(cfg *config.Config, env *Env) error {
	if len(cfg.Hooks.PostCreate) == 0 {
		return nil
	}
	fmt.Println("Running post-create hooks...")
	_ = os.Stdout.Sync()
	return Run(cfg.Hooks.PostCreate, env, env.Path)
}

// RunPreDelete runs pre-delete hooks.
func RunPreDelete(cfg *config.Config, env *Env) error {
	if len(cfg.Hooks.PreDelete) == 0 {
		return nil
	}
	fmt.Println("Running pre-delete hooks...")
	_ = os.Stdout.Sync()
	return Run(cfg.Hooks.PreDelete, env, env.Path)
}

// RunPostDelete runs post-delete hooks.
func RunPostDelete(cfg *config.Config, env *Env) error {
	if len(cfg.Hooks.PostDelete) == 0 {
		return nil
	}
	fmt.Println("Running post-delete hooks...")
	_ = os.Stdout.Sync()
	return Run(cfg.Hooks.PostDelete, env, env.RepoRoot)
}

// RunInfo runs info hooks and returns captured stdout.
func RunInfo(cfg *config.Config, env *Env) (string, error) {
	if len(cfg.Hooks.Info) == 0 {
		return "", nil
	}
	return runAndCapture(cfg.Hooks.Info, env, env.Path)
}

func runAndCapture(entries []config.HookEntry, env *Env, workDir string) (string, error) {
	var output bytes.Buffer
	for _, entry := range entries {
		out, err := runHookCapture(entry, env, workDir)
		if err != nil {
			return "", err
		}
		output.WriteString(out)
	}
	return output.String(), nil
}

func runHookCapture(entry config.HookEntry, env *Env, workDir string) (string, error) {
	scriptPath := entry.Script
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Join(env.RepoRoot, scriptPath)
	}

	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("hook script not found: %s", scriptPath)
	}

	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Dir = workDir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env.ToEnvVars()...)

	for k, v := range entry.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
