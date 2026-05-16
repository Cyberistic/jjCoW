package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aranw/jjw/internal/config"
	"github.com/aranw/jjw/internal/hooks"
	"github.com/aranw/jjw/internal/jj"

	"github.com/spf13/cobra"
)

var (
	createBookmark string
	createRevision string
)

func init() {
	createCmd.Flags().StringVarP(&createBookmark, "bookmark", "b", "", "Use existing bookmark instead of creating a new one")
	createCmd.Flags().StringVarP(&createRevision, "revision", "r", "", "Base revision for the new workspace (default: default_branch from config)")
	rootCmd.AddCommand(createCmd)
}

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new workspace",
	Long: `Create a new jj workspace with the specified name.

By default, a new bookmark with the same name will be created pointing
at the workspace's working copy commit.

Use --bookmark to associate an existing bookmark instead of creating one.
Use --revision to specify which revision to branch from (defaults to
the default_branch in .jjw.yaml, typically main).

The workspace will be created in the directory specified by workspace_dir
in your .jjw.yaml configuration (default: workspaces/).

After creation, any post_create hooks defined in .jjw.yaml will be executed.`,
	Args: cobra.ExactArgs(1),
	RunE: runCreate,
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	repoRoot, err := config.GetMainRepoRoot()
	if err != nil {
		return fmt.Errorf("not in a jjw-enabled repository: %w", err)
	}

	cfg, err := config.Load(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to load config: %w (is .jjw.yaml present?)", err)
	}

	jjRoot := cfg.JJRoot(repoRoot)
	workspacePath := filepath.Join(repoRoot, cfg.WorkspaceDir, name)

	// Determine the bookmark name
	bookmarkName := createBookmark
	if bookmarkName == "" {
		bookmarkName = strings.ReplaceAll(cfg.BookmarkPattern, "{name}", name)
	}

	// Determine the base revision
	revision := createRevision
	if revision == "" {
		revision = cfg.DefaultBranch
	}

	// Create hook environment
	env := &hooks.Env{
		Name:         name,
		Path:         workspacePath,
		Bookmark:     bookmarkName,
		RepoRoot:     repoRoot,
		JJRoot:       jjRoot,
		WorkspaceDir: cfg.WorkspaceDir,
	}

	// Run pre-create hooks
	if err := hooks.RunPreCreate(cfg, env); err != nil {
		return fmt.Errorf("pre-create hook failed: %w", err)
	}

	// Create the workspace
	if createBookmark != "" {
		// Use existing bookmark as the base revision
		localName, remote, isRemote := jj.ParseBookmarkRef(createBookmark)
		if isRemote {
			if !jj.RemoteBookmarkExists(jjRoot, localName, remote) {
				return fmt.Errorf("bookmark %q does not exist on remote %q", localName, remote)
			}
			bookmarkName = localName
			env.Bookmark = bookmarkName
		} else {
			if !jj.BookmarkExists(jjRoot, createBookmark) {
				return fmt.Errorf("bookmark %q does not exist", createBookmark)
			}
		}
		cmd.Printf("Creating workspace %q from bookmark %q...\n", name, createBookmark)
		if err := jj.WorkspaceAdd(jjRoot, workspacePath, name, createBookmark); err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}
	} else {
		// Check if bookmark already exists
		if jj.BookmarkExists(jjRoot, bookmarkName) {
			return fmt.Errorf("bookmark %q already exists (use --bookmark to use an existing bookmark)", bookmarkName)
		}
		cmd.Printf("Creating workspace %q from %q...\n", name, revision)
		if err := jj.WorkspaceAdd(jjRoot, workspacePath, name, revision); err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}

		// Create the bookmark pointing at the new workspace's working copy
		cmd.Printf("Creating bookmark %q...\n", bookmarkName)
		if err := jj.BookmarkCreate(workspacePath, bookmarkName, "@"); err != nil {
			cmd.Printf("Warning: failed to create bookmark: %v\n", err)
		}
	}

	// Track bookmark on remote if configured
	if cfg.TrackRemote != "" {
		cmd.Printf("Tracking bookmark %q on %q...\n", bookmarkName, cfg.TrackRemote)
		if err := jj.BookmarkTrack(jjRoot, bookmarkName, cfg.TrackRemote); err != nil {
			cmd.Printf("Warning: failed to track bookmark on %s: %v\n", cfg.TrackRemote, err)
		}
	}

	// Store creation metadata
	if err := jj.SetCreatedAt(repoRoot, name, time.Now()); err != nil {
		cmd.Printf("Warning: could not store creation time: %v\n", err)
	}

	// Allocate and store workspace index
	index, err := jj.AllocateIndex(repoRoot, cfg.Index.Max)
	if err != nil {
		cmd.Printf("Warning: could not allocate index: %v\n", err)
	} else {
		if err := jj.SetIndex(repoRoot, name, index); err != nil {
			cmd.Printf("Warning: could not store index: %v\n", err)
		} else {
			env.Index = index
		}
	}

	// Run post-create hooks
	if err := hooks.RunPostCreate(cfg, env); err != nil {
		cmd.Printf("Warning: post-create hook failed: %v\n", err)
	}

	cmd.Printf("Workspace %q created successfully\n", name)

	// Output the path for shell wrapper
	if cdFile := os.Getenv("JJW_CD_FILE"); cdFile != "" {
		_ = os.WriteFile(cdFile, []byte(workspacePath+"\n"), 0600)
	} else {
		cmd.Printf("\nRun `cd %s` to open your new workspace\n", workspacePath)
	}

	return nil
}
