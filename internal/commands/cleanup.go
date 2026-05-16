package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aranw/jjw/internal/config"
	"github.com/aranw/jjw/internal/hooks"
	"github.com/aranw/jjw/internal/jj"

	"github.com/spf13/cobra"
)

var (
	cleanupDryRun       bool
	cleanupForce        bool
	cleanupKeepBookmark bool
)

func init() {
	cleanupCmd.Flags().BoolVarP(&cleanupDryRun, "dry-run", "n", false, "Show what would be deleted without deleting")
	cleanupCmd.Flags().BoolVarP(&cleanupForce, "force", "f", false, "Skip confirmation prompts and pre-delete hook failures")
	cleanupCmd.Flags().BoolVarP(&cleanupKeepBookmark, "keep-bookmark", "k", false, "Keep the associated bookmarks (default: delete them)")
	rootCmd.AddCommand(cleanupCmd)
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up merged workspaces",
	Long: `Find and remove clean workspaces whose bookmarks have been merged.

This command identifies workspaces eligible for cleanup:
- Bookmarks that have been merged into the default branch
- Workspaces whose bookmark is gone but whose working copy parent is merged

Cleanup only deletes workspaces with an empty working copy. If an otherwise
eligible workspace has uncommitted changes, cleanup cancels instead of
deleting it.

By default, both the workspace and its associated bookmark are deleted.

Use --dry-run to see what would be deleted without actually deleting.
Use --force to skip confirmation prompts and pre-delete hook failures.
Use --keep-bookmark to preserve the associated bookmarks.`,
	RunE: runCleanup,
}

type cleanupCandidate struct {
	name     string
	path     string
	bookmark string
	status   *jj.WorkspaceStatus
}

func runCleanup(cmd *cobra.Command, args []string) error {
	repoRoot, err := config.GetMainRepoRoot()
	if err != nil {
		return fmt.Errorf("not in a jjw-enabled repository: %w", err)
	}

	cfg, err := config.Load(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	jjRoot := cfg.JJRoot(repoRoot)
	workspacesDir := filepath.Join(repoRoot, cfg.WorkspaceDir)

	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		if os.IsNotExist(err) {
			cmd.Println("No workspaces eligible for cleanup")
			return nil
		}
		return fmt.Errorf("failed to read workspace directory: %w", err)
	}

	var candidates []cleanupCandidate

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		wsPath := filepath.Join(workspacesDir, name)
		bookmarkName := strings.ReplaceAll(cfg.BookmarkPattern, "{name}", name)

		// Skip if not a jj workspace
		if _, err := os.Stat(filepath.Join(wsPath, ".jj")); os.IsNotExist(err) {
			continue
		}

		// Never clean up the workspace whose bookmark is the default branch
		if bookmarkName == cfg.DefaultBranch {
			continue
		}

		bookmarkExists := jj.BookmarkExists(jjRoot, bookmarkName)

		if bookmarkExists {
			status := jj.GetWorkspaceStatus(jjRoot, repoRoot, wsPath, name, bookmarkName, cfg.DefaultBranch)

			// Cleanup if merged and no commits ahead. Never clean up a non-empty
			// working copy; it may contain local work that has not been preserved.
			if status.IsMerged && status.CommitsAhead == 0 {
				if !status.IsEmpty {
					return fmt.Errorf("workspace %q is eligible for cleanup but has uncommitted changes; canceling", name)
				}
				candidates = append(candidates, cleanupCandidate{
					name:     name,
					path:     wsPath,
					bookmark: bookmarkName,
					status:   status,
				})
			}
		} else {
			// Bookmark is gone — likely merged and deleted on the remote.
			// Check if the workspace working copy is an ancestor of the default branch.
			if jj.IsWorkingCopyMerged(wsPath, cfg.DefaultBranch) {
				if !jj.IsWorkingCopyEmpty(wsPath) {
					return fmt.Errorf("workspace %q is eligible for cleanup but has uncommitted changes; canceling", name)
				}
				candidates = append(candidates, cleanupCandidate{
					name:     name,
					path:     wsPath,
					bookmark: bookmarkName,
					status:   &jj.WorkspaceStatus{IsMerged: true, IsEmpty: true},
				})
			}
		}
	}

	if len(candidates) == 0 {
		cmd.Println("No workspaces eligible for cleanup")
		return nil
	}

	// Display candidates
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Workspaces eligible for cleanup:")
	fmt.Fprintln(out)

	nameWidth := len("NAME")
	bookmarkWidth := len("BOOKMARK")
	for _, c := range candidates {
		if len(c.name) > nameWidth {
			nameWidth = len(c.name)
		}
		if len(c.bookmark) > bookmarkWidth {
			bookmarkWidth = len(c.bookmark)
		}
	}

	fmt.Fprintf(out, "  %-*s  %-*s  %s\n", nameWidth, "NAME", bookmarkWidth, "BOOKMARK", "STATUS")
	for _, c := range candidates {
		statusStr := formatStatus(c.status)
		fmt.Fprintf(out, "  %-*s  %-*s  %s\n", nameWidth, c.name, bookmarkWidth, c.bookmark, statusStr)
	}
	fmt.Fprintln(out)

	if cleanupDryRun {
		cmd.Printf("Would delete %d workspace(s)", len(candidates))
		if !cleanupKeepBookmark {
			cmd.Print(" and their bookmarks")
		}
		cmd.Println()
		return nil
	}

	if !cleanupForce {
		cmd.Printf("Delete %d workspace(s)", len(candidates))
		if !cleanupKeepBookmark {
			cmd.Print(" and their bookmarks")
		}
		cmd.Println("?")
		if !confirmAction("Proceed?") {
			return fmt.Errorf("aborted")
		}
	}

	// Build set of workspaces known to jj
	knownWorkspaces := make(map[string]bool)
	if wsList, err := jj.WorkspaceList(jjRoot); err == nil {
		for _, ws := range wsList {
			knownWorkspaces[ws.Name] = true
		}
	}

	// Check if user is in any candidate workspace
	cwd, _ := os.Getwd()
	inDeletedWorkspace := false
	for _, c := range candidates {
		if isPathWithin(cwd, c.path) {
			inDeletedWorkspace = true
			break
		}
	}

	var deleted int
	for _, c := range candidates {
		env := &hooks.Env{
			Name:         c.name,
			Path:         c.path,
			Bookmark:     c.bookmark,
			RepoRoot:     repoRoot,
			JJRoot:       jjRoot,
			WorkspaceDir: cfg.WorkspaceDir,
		}

		if idx, err := jj.GetIndex(repoRoot, c.name); err == nil {
			env.Index = idx
		}

		// Run pre-delete hooks
		if err := hooks.RunPreDelete(cfg, env); err != nil {
			if !cleanupForce {
				cmd.Printf("Skipping %s: pre-delete hook failed: %v\n", c.name, err)
				continue
			}
			cmd.Printf("Warning: pre-delete hook failed for %s: %v\n", c.name, err)
		}

		// Forget workspace (only if jj knows about it)
		if knownWorkspaces[c.name] {
			cmd.Printf("Forgetting workspace %q...\n", c.name)
			if err := jj.WorkspaceForget(jjRoot, c.name); err != nil {
				cmd.Printf("Warning: jj workspace forget %s: %v\n", c.name, err)
			}
		}

		// Remove files
		if err := os.RemoveAll(c.path); err != nil {
			cmd.Printf("Error: failed to remove %s: %v\n", c.name, err)
			continue
		}

		// Delete bookmark
		if !cleanupKeepBookmark && jj.BookmarkExists(jjRoot, c.bookmark) {
			cmd.Printf("Deleting bookmark %q...\n", c.bookmark)
			if err := jj.BookmarkDelete(jjRoot, c.bookmark); err != nil {
				cmd.Printf("Warning: failed to delete bookmark %s: %v\n", c.bookmark, err)
			}
		}

		// Clean up metadata
		_ = jj.CleanupMetadata(repoRoot, c.name)

		// Run post-delete hooks
		if err := hooks.RunPostDelete(cfg, env); err != nil {
			cmd.Printf("Warning: post-delete hook failed for %s: %v\n", c.name, err)
		}

		deleted++
	}

	cmd.Printf("Cleaned up %d workspace(s)\n", deleted)

	if inDeletedWorkspace && deleted > 0 {
		if cdFile := os.Getenv("JJW_CD_FILE"); cdFile != "" {
			_ = os.WriteFile(cdFile, []byte(repoRoot+"\n"), 0600)
		} else {
			cmd.Printf("\nRun `cd %s` to return to the repository root\n", repoRoot)
		}
	}

	return nil
}
