package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cyberistic/jjCoW/internal/config"
	"github.com/Cyberistic/jjCoW/internal/hooks"
	"github.com/Cyberistic/jjCoW/internal/jj"

	"github.com/spf13/cobra"
)

var (
	deleteForce        bool
	deleteKeepBookmark bool
)

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Delete without warning about uncommitted or unmerged work")
	deleteCmd.Flags().BoolVarP(&deleteKeepBookmark, "keep-bookmark", "k", false, "Keep the associated bookmark (default: delete it)")
	rootCmd.AddCommand(deleteCmd)
}

var deleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete a workspace",
	Long: `Delete a jj workspace, its files, and the associated bookmark.

If no name is provided and you're currently inside a workspace,
that workspace will be deleted.

Without --force, jjw warns and asks for confirmation if the workspace
has a non-empty working copy or its work does not appear to be merged
into the configured default branch. With --force, these safety warnings
are skipped.

By default, the associated bookmark is also deleted.
Use --keep-bookmark to preserve it.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDelete,
}

func runDelete(cmd *cobra.Command, args []string) error {
	repoRoot, err := config.GetMainRepoRoot()
	if err != nil {
		return fmt.Errorf("not in a jjw-enabled repository: %w", err)
	}

	cfg, err := config.Load(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	jjRoot := cfg.JJRoot(repoRoot)

	// Determine which workspace to delete
	var name string
	var workspacePath string

	if len(args) > 0 {
		name = args[0]
		workspacePath = filepath.Join(repoRoot, cfg.WorkspaceDir, name)
	} else {
		// Auto-detect from current directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		workspacesDir := filepath.Join(repoRoot, cfg.WorkspaceDir)
		if !isPathWithin(cwd, workspacesDir) {
			return fmt.Errorf("not in a workspace (specify name or cd into a workspace)")
		}

		rel, err := filepath.Rel(workspacesDir, cwd)
		if err != nil {
			return fmt.Errorf("failed to determine workspace: %w", err)
		}
		parts := strings.Split(rel, string(filepath.Separator))
		name = parts[0]
		workspacePath = filepath.Join(workspacesDir, name)
	}

	// Check if workspace directory exists
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return fmt.Errorf("workspace %q does not exist", name)
	}

	// Derive the bookmark name
	bookmarkName := strings.ReplaceAll(cfg.BookmarkPattern, "{name}", name)

	// Create hook environment
	env := &hooks.Env{
		Name:         name,
		Path:         workspacePath,
		Bookmark:     bookmarkName,
		RepoRoot:     repoRoot,
		JJRoot:       jjRoot,
		WorkspaceDir: cfg.WorkspaceDir,
	}

	if idx, err := jj.GetIndex(repoRoot, name); err == nil {
		env.Index = idx
	}

	// Check if user is in the workspace being deleted
	cwd, _ := os.Getwd()
	inDeletedWorkspace := isPathWithin(cwd, workspacePath)

	if !deleteForce {
		warnings := deletionWarnings(jjRoot, repoRoot, workspacePath, name, bookmarkName, cfg.DefaultBranch)
		if len(warnings) > 0 {
			cmd.Println("Warning: this workspace may contain work that has not been merged:")
			for _, warning := range warnings {
				cmd.Printf("  - %s\n", warning)
			}
			if !confirmAction("Delete anyway?") {
				return fmt.Errorf("aborted")
			}
		}
	}

	// Run pre-delete hooks
	if err := hooks.RunPreDelete(cfg, env); err != nil {
		if !deleteForce {
			return fmt.Errorf("pre-delete hook failed: %w", err)
		}
		cmd.Printf("Warning: pre-delete hook failed: %v\n", err)
	}

	// Forget the workspace in jj
	cmd.Printf("Forgetting workspace %q...\n", name)
	if err := jj.WorkspaceForget(jjRoot, name); err != nil {
		// Workspace might already be forgotten, continue with cleanup
		cmd.Printf("Warning: jj workspace forget: %v\n", err)
	}

	// Remove the workspace directory
	cmd.Printf("Removing workspace files...\n")
	if err := os.RemoveAll(workspacePath); err != nil {
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	// Delete the bookmark unless --keep-bookmark is specified
	if !deleteKeepBookmark && jj.BookmarkExists(jjRoot, bookmarkName) {
		cmd.Printf("Deleting bookmark %q...\n", bookmarkName)
		if err := jj.BookmarkDelete(jjRoot, bookmarkName); err != nil {
			cmd.Printf("Warning: failed to delete bookmark: %v\n", err)
		}
	}

	// Clean up metadata
	_ = jj.CleanupMetadata(repoRoot, name)

	// Run post-delete hooks
	if err := hooks.RunPostDelete(cfg, env); err != nil {
		cmd.Printf("Warning: post-delete hook failed: %v\n", err)
	}

	cmd.Printf("Workspace %q deleted successfully\n", name)

	// If user was in the deleted workspace, navigate back
	if inDeletedWorkspace {
		if cdFile := os.Getenv("JJW_CD_FILE"); cdFile != "" {
			_ = os.WriteFile(cdFile, []byte(repoRoot+"\n"), 0600)
		} else {
			cmd.Printf("\nRun `cd %s` to return to the repository root\n", repoRoot)
		}
	}

	return nil
}

func confirmAction(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	_ = os.Stdout.Sync()
	fmt.Printf("%s [y/N] ", prompt)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
