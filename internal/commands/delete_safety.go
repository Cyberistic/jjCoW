package commands

import (
	"fmt"

	"github.com/Cyberistic/jjCoW/internal/jj"
)

func deletionWarnings(jjRoot, repoRoot, workspacePath, workspaceName, bookmarkName, defaultBranch string) []string {
	var warnings []string

	if !jj.IsWorkingCopyEmpty(workspacePath) {
		warnings = append(warnings, "working copy is not empty")
	}

	if jj.BookmarkExists(jjRoot, bookmarkName) {
		status := jj.GetWorkspaceStatus(jjRoot, repoRoot, workspacePath, workspaceName, bookmarkName, defaultBranch)
		if !status.IsMerged || status.CommitsAhead > 0 {
			warnings = append(warnings, fmt.Sprintf("bookmark %q does not appear to be merged into %q", bookmarkName, defaultBranch))
		}
	} else if !jj.IsWorkingCopyMerged(workspacePath, defaultBranch) {
		warnings = append(warnings, fmt.Sprintf("working copy parent does not appear to be merged into %q", defaultBranch))
	}

	return warnings
}
