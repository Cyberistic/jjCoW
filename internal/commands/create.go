package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Cyberistic/jjCoW/internal/config"
	"github.com/Cyberistic/jjCoW/internal/cow"
	"github.com/Cyberistic/jjCoW/internal/hooks"
	"github.com/Cyberistic/jjCoW/internal/jj"

	"github.com/spf13/cobra"
)

var (
	createBookmark string
	createRevision string
	createNoCow    bool
	createLazy     bool
)

func init() {
	createCmd.Flags().StringVarP(&createBookmark, "bookmark", "b", "", "Use existing bookmark instead of creating a new one")
	createCmd.Flags().StringVarP(&createRevision, "revision", "r", "", "Base revision for the new workspace (default: default_branch from config)")
	createCmd.Flags().BoolVar(&createNoCow, "no-cow", false, "Disable copy-on-write cloning (let jj perform a full checkout)")
	createCmd.Flags().BoolVar(&createLazy, "lazy", false, "Defer jj adoption of cloned files (fastest; you must run `jj sparse reset` in the workspace before using jj)")
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

	// jj requires the workspace's parent directory to exist
	if err := os.MkdirAll(filepath.Dir(workspacePath), 0o755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

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
	useCow, err := cowRequested(cfg)
	if err != nil {
		return err
	}
	lazy := createLazy || cfg.CowLazy
	adoptPending := false
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
		if adoptPending, err = addWorkspace(cmd, jjRoot, workspacePath, name, createBookmark, cfg, useCow, lazy); err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}
	} else {
		// Check if bookmark already exists
		if jj.BookmarkExists(jjRoot, bookmarkName) {
			return fmt.Errorf("bookmark %q already exists (use --bookmark to use an existing bookmark)", bookmarkName)
		}
		cmd.Printf("Creating workspace %q from %q...\n", name, revision)
		if adoptPending, err = addWorkspace(cmd, jjRoot, workspacePath, name, revision, cfg, useCow, lazy); err != nil {
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

	if adoptPending {
		cmd.Printf("\nWarning: cloned files are not adopted by jj yet (lazy mode).\n")
		cmd.Printf("Before running jj commands in the workspace, run:\n\n    jj -R %s sparse reset\n\n", workspacePath)
	}

	// Output the path for shell wrapper
	if cdFile := os.Getenv("JJW_CD_FILE"); cdFile != "" {
		_ = os.WriteFile(cdFile, []byte(workspacePath+"\n"), 0600)
	} else {
		cmd.Printf("\nRun `cd %s` to open your new workspace\n", workspacePath)
	}

	return nil
}

// cowRequested reports whether copy-on-write workspace cloning should be
// used. It can be disabled by the --no-cow flag, NO_COW=1 in the
// environment, or `cow: false` in .jjw.yaml. When requested on an
// unsupported operating system, a clear error is returned.
func cowRequested(cfg *config.Config) (bool, error) {
	disabled := createNoCow || !cfg.Cow
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NO_COW"))) {
	case "1", "true", "yes", "on":
		disabled = true
	}
	if disabled {
		return false, nil
	}
	if !cow.Supported() {
		return false, fmt.Errorf(
			"copy-on-write workspaces are not supported on %s (supported: macOS, Linux); "+
				"disable with --no-cow, NO_COW=1, or `cow: false` in .jjw.yaml", runtime.GOOS)
	}
	return true, nil
}

// addWorkspace creates the jj workspace, optionally using copy-on-write
// cloning of the main working copy instead of a full jj checkout. The clone
// carries over untracked and ignored files (e.g. .env) while skipping
// regenerable artifacts (node_modules, target, dist, ...). It reports
// whether jj still has to adopt the cloned files (lazy mode).
func addWorkspace(cmd *cobra.Command, jjRoot, workspacePath, name, revision string, cfg *config.Config, useCow, lazy bool) (bool, error) {
	if !useCow {
		return false, jj.WorkspaceAdd(jjRoot, workspacePath, name, revision)
	}

	if err := jj.WorkspaceAddEmpty(jjRoot, workspacePath, name, revision); err != nil {
		cmd.Printf("Warning: sparse-empty workspace add failed, falling back to full checkout: %v\n", err)
		return false, jj.WorkspaceAdd(jjRoot, workspacePath, name, revision)
	}

	excludeTop := map[string]bool{
		".jj":            true,
		".git":           true,
		".jjw":           true,
		cfg.WorkspaceDir: true,
	}
	if err := cow.CloneDir(jjRoot, workspacePath, excludeTop); err != nil {
		// Keep whatever jj can materialize; a sparse reset below performs a
		// full checkout of anything the clone missed.
		cmd.Printf("Warning: copy-on-write clone failed (%v), falling back to jj checkout\n", err)
		lazy = false
	}
	if lazy {
		return true, nil
	}
	// Fast path: hand the workspace the main working copy's tree_state so jj
	// trusts the cloned files without re-hashing them. Fall back to a sparse
	// reset (full content verification) whenever that is not possible.
	if err := jj.AdoptClonedWorkspace(jjRoot, workspacePath); err != nil {
		if err := jj.SparseReset(workspacePath); err != nil {
			return false, err
		}
	}
	return false, nil
}
