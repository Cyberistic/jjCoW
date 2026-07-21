package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cyberistic/jjCoW/internal/config"
	"github.com/Cyberistic/jjCoW/internal/jj"

	"github.com/spf13/cobra"
)

var verboseFlag bool

func init() {
	listCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "Show detailed status for each workspace")
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all workspaces",
	Long: `List all managed jj workspaces with their status.

Shows each workspace with:
- Name and associated bookmark
- Commits ahead/behind the default branch
- Status indicators: [empty], [merged], [ahead]

Use -v/--verbose for detailed multi-line output.`,
	RunE: runList,
}

type workspaceInfo struct {
	name     string
	bookmark string
	path     string
	isCwd    bool
	status   *jj.WorkspaceStatus
	index    int
}

func runList(cmd *cobra.Command, args []string) error {
	repoRoot, err := config.GetMainRepoRoot()
	if err != nil {
		return fmt.Errorf("not in a jjw-enabled repository: %w", err)
	}

	cfg, err := config.Load(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	jjRoot := cfg.JJRoot(repoRoot)
	cwd, _ := os.Getwd()
	workspacesDir := filepath.Join(repoRoot, cfg.WorkspaceDir)

	// List workspace directories on disk
	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(cmd.OutOrStdout(), "No workspaces")
			return nil
		}
		return fmt.Errorf("failed to read workspace directory: %w", err)
	}

	var managed []workspaceInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		wsPath := filepath.Join(workspacesDir, name)
		bookmarkName := strings.ReplaceAll(cfg.BookmarkPattern, "{name}", name)

		// Check if this workspace dir has a .jj (is actually a jj workspace)
		if _, err := os.Stat(filepath.Join(wsPath, ".jj")); os.IsNotExist(err) {
			continue
		}

		isCwd := isPathWithin(cwd, wsPath)

		var status *jj.WorkspaceStatus
		if jj.BookmarkExists(jjRoot, bookmarkName) {
			status = jj.GetWorkspaceStatus(jjRoot, repoRoot, wsPath, name, bookmarkName, cfg.DefaultBranch)
		} else if jj.IsWorkingCopyMerged(wsPath, cfg.DefaultBranch) {
			status = &jj.WorkspaceStatus{IsMerged: true}
		}

		idx, _ := jj.GetIndex(repoRoot, name)

		managed = append(managed, workspaceInfo{
			name:     name,
			bookmark: bookmarkName,
			path:     wsPath,
			isCwd:    isCwd,
			status:   status,
			index:    idx,
		})
	}

	if len(managed) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No workspaces")
		return nil
	}

	if verboseFlag {
		printVerboseWorkspaces(cmd, managed)
	} else {
		printCompactWorkspaces(cmd, managed)
	}

	return nil
}

func printCompactWorkspaces(cmd *cobra.Command, workspaces []workspaceInfo) {
	out := cmd.OutOrStdout()

	// Calculate column widths
	nameWidth := len("NAME")
	bookmarkWidth := len("BOOKMARK")
	for _, ws := range workspaces {
		if len(ws.name) > nameWidth {
			nameWidth = len(ws.name)
		}
		if len(ws.bookmark) > bookmarkWidth {
			bookmarkWidth = len(ws.bookmark)
		}
	}

	// Header
	fmt.Fprintf(out, "  %-*s  %5s  %-*s  %s\n", nameWidth, "NAME", "INDEX", bookmarkWidth, "BOOKMARK", "STATUS")

	for _, ws := range workspaces {
		marker := "  "
		if ws.isCwd {
			marker = "* "
		}

		indexStr := "-"
		if ws.index > 0 {
			indexStr = fmt.Sprintf("%d", ws.index)
		}

		statusStr := formatStatus(ws.status)
		fmt.Fprintf(out, "%s%-*s  %5s  %-*s  %s\n", marker, nameWidth, ws.name, indexStr, bookmarkWidth, ws.bookmark, statusStr)
	}
}

func printVerboseWorkspaces(cmd *cobra.Command, workspaces []workspaceInfo) {
	out := cmd.OutOrStdout()
	sep := strings.Repeat("=", 60)

	for _, ws := range workspaces {
		fmt.Fprintln(out, sep)

		marker := " "
		if ws.isCwd {
			marker = "*"
		}

		fmt.Fprintf(out, "%s %s\n", marker, ws.name)
		fmt.Fprintf(out, "  Bookmark: %s\n", ws.bookmark)
		fmt.Fprintf(out, "  Path:     %s\n", ws.path)

		if ws.index > 0 {
			fmt.Fprintf(out, "  Index:    %d\n", ws.index)
		}

		if ws.status != nil {
			if !ws.status.CreatedAt.IsZero() {
				fmt.Fprintf(out, "  Created:  %s (%s ago)\n",
					ws.status.CreatedAt.Format("2006-01-02 15:04"),
					formatDuration(time.Since(ws.status.CreatedAt)))
			}
			fmt.Fprintf(out, "  Status:   %s\n", formatStatus(ws.status))
			if ws.status.CommitsAhead > 0 || ws.status.CommitsBehind > 0 {
				fmt.Fprintf(out, "  Commits:  ↑%d ↓%d\n", ws.status.CommitsAhead, ws.status.CommitsBehind)
			}
		}
	}
	fmt.Fprintln(out, sep)
}

func formatStatus(status *jj.WorkspaceStatus) string {
	if status == nil {
		return "[no bookmark]"
	}

	var parts []string
	if status.IsMerged {
		parts = append(parts, "merged")
	}
	if status.IsEmpty {
		parts = append(parts, "empty")
	}
	if status.CommitsAhead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", status.CommitsAhead))
	}
	if status.CommitsBehind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", status.CommitsBehind))
	}

	if len(parts) == 0 {
		return "[ok]"
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", m)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		if h == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", h)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}
