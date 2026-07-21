package jj

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Workspace represents a jj workspace
type Workspace struct {
	Name     string
	Path     string // absolute path on disk
	ChangeID string
	CommitID string
}

// Bookmark represents a jj bookmark
type Bookmark struct {
	Name     string
	ChangeID string
	CommitID string
	Tracked  bool // whether it's tracked on a remote
}

// WorkspaceStatus holds status information for a managed workspace
type WorkspaceStatus struct {
	CommitsAhead  int
	CommitsBehind int
	IsMerged      bool
	IsEmpty       bool // working copy has no changes vs parent
	CreatedAt     time.Time
	Index         int
}

// WorkspaceAdd creates a new jj workspace at the given path, branching from revision.
func WorkspaceAdd(repoRoot, destPath, name, revision string) error {
	args := []string{"workspace", "add", destPath, "--name", name}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	cmd := exec.Command("jj", args...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// WorkspaceAddEmpty creates a new jj workspace with empty sparse patterns,
// so jj writes no files to the destination. Use SparseReset afterwards to
// make jj adopt whatever content is placed there.
func WorkspaceAddEmpty(repoRoot, destPath, name, revision string) error {
	args := []string{"workspace", "add", destPath, "--name", name, "--sparse-patterns", "empty"}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	cmd := exec.Command("jj", args...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SparseReset resets a workspace's sparse patterns to include all files and
// updates the working copy. Files already on disk with matching content are
// adopted without being rewritten. Output is suppressed unless it fails.
func SparseReset(workspacePath string) error {
	cmd := exec.Command("jj", "sparse", "reset")
	cmd.Dir = workspacePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj sparse reset: %w\n%s", err, output)
	}
	return nil
}

// AdoptClonedWorkspace makes jj adopt a freshly cloned workspace in O(stat)
// time by copying the main working copy's tree_state (file hashes, mtimes,
// sparse patterns) into the new workspace, instead of re-hashing every file
// via SparseReset. Because the clone preserves content, mode, and mtimes,
// the copied state matches the files on disk; a `jj st` verifies the result
// and any failure restores the original state so callers can fall back to
// SparseReset.
//
// Requires that the clone source is the main working copy at jjRoot and that
// a jj command (e.g. WorkspaceAddEmpty) just snapshotted it, so its
// tree_state is current.
func AdoptClonedWorkspace(jjRoot, workspacePath string) error {
	if watchmanEnabled(jjRoot) {
		return fmt.Errorf("watchman fsmonitor is enabled")
	}

	srcPath := filepath.Join(jjRoot, ".jj", "working_copy", "tree_state")
	dstPath := filepath.Join(workspacePath, ".jj", "working_copy", "tree_state")

	state, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read source tree_state: %w", err)
	}
	original, err := os.ReadFile(dstPath)
	if err != nil {
		return fmt.Errorf("read workspace tree_state: %w", err)
	}
	if err := os.WriteFile(dstPath, state, 0o600); err != nil {
		return fmt.Errorf("write workspace tree_state: %w", err)
	}

	cmd := exec.Command("jj", "st")
	cmd.Dir = workspacePath
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.WriteFile(dstPath, original, 0o600)
		return fmt.Errorf("validate adopted state: %w\n%s", err, output)
	}
	return nil
}

// watchmanEnabled reports whether the repository uses the watchman fsmonitor,
// whose clock state must not be shared between working copies.
func watchmanEnabled(repoRoot string) bool {
	cmd := exec.Command("jj", "config", "get", "fsmonitor.watchman.register_snapshot_trigger")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(output)) == "true"
}

// WorkspaceForget tells jj to stop tracking a workspace.
func WorkspaceForget(repoRoot, name string) error {
	cmd := exec.Command("jj", "workspace", "forget", name)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// WorkspaceList returns all workspaces known to jj.
func WorkspaceList(repoRoot string) ([]Workspace, error) {
	// Use jj workspace list with a template for structured output
	cmd := exec.Command("jj", "workspace", "list")
	cmd.Dir = repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("jj workspace list: %w", err)
	}

	var workspaces []Workspace
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// jj workspace list output format:
		// default: rlvkpnrz a8c43455 (empty) (no description set)
		// feature-x: stmvosmo 12345678 some description
		colonIdx := strings.Index(line, ": ")
		if colonIdx == -1 {
			continue
		}

		name := line[:colonIdx]
		rest := line[colonIdx+2:]

		// Parse change ID and commit ID from the rest
		fields := strings.Fields(rest)
		var changeID, commitID string
		if len(fields) >= 1 {
			changeID = fields[0]
		}
		if len(fields) >= 2 {
			commitID = fields[1]
		}

		workspaces = append(workspaces, Workspace{
			Name:     name,
			ChangeID: changeID,
			CommitID: commitID,
		})
	}

	return workspaces, scanner.Err()
}

// BookmarkCreate creates a new bookmark pointing at the given revision.
// workDir should be the workspace directory so @ resolves correctly.
func BookmarkCreate(workDir, name, revision string) error {
	args := []string{"bookmark", "create", name}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	cmd := exec.Command("jj", args...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// BookmarkTrack sets up tracking between a local bookmark and a remote bookmark.
func BookmarkTrack(repoRoot, name, remote string) error {
	ref := fmt.Sprintf("%s@%s", name, remote)
	cmd := exec.Command("jj", "bookmark", "track", ref)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// BookmarkDelete deletes a local bookmark.
func BookmarkDelete(repoRoot, name string) error {
	cmd := exec.Command("jj", "bookmark", "delete", name)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// BookmarkExists checks if a bookmark with the given name exists.
func BookmarkExists(repoRoot, name string) bool {
	cmd := exec.Command("jj", "bookmark", "list", name)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

// RemoteBookmarkExists checks if a remote bookmark (name@remote) exists.
func RemoteBookmarkExists(repoRoot, name, remote string) bool {
	cmd := exec.Command("jj", "bookmark", "list", "--remote", remote, name)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

// ParseBookmarkRef splits a bookmark reference into name and optional remote.
// "foo@origin" returns ("foo", "origin", true).
// "foo" returns ("foo", "", false).
func ParseBookmarkRef(ref string) (name, remote string, isRemote bool) {
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		return ref[:idx], ref[idx+1:], true
	}
	return ref, "", false
}

// BookmarkList returns all local bookmarks.
func BookmarkList(repoRoot string) ([]Bookmark, error) {
	cmd := exec.Command("jj", "bookmark", "list")
	cmd.Dir = repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("jj bookmark list: %w", err)
	}

	var bookmarks []Bookmark
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format: bookmark_name: changeID commitID description
		// or: bookmark_name (tracked): changeID commitID description
		colonIdx := strings.Index(line, ": ")
		if colonIdx == -1 {
			continue
		}

		nameField := line[:colonIdx]
		rest := line[colonIdx+2:]

		tracked := false
		name := nameField
		if strings.Contains(nameField, " (tracked)") {
			tracked = true
			name = strings.TrimSuffix(nameField, " (tracked)")
		}
		// Handle asterisk suffix (indicates local differs from remote)
		name = strings.TrimSuffix(name, "*")

		fields := strings.Fields(rest)
		var changeID, commitID string
		if len(fields) >= 1 {
			changeID = fields[0]
		}
		if len(fields) >= 2 {
			commitID = fields[1]
		}

		bookmarks = append(bookmarks, Bookmark{
			Name:     name,
			ChangeID: changeID,
			CommitID: commitID,
			Tracked:  tracked,
		})
	}

	return bookmarks, scanner.Err()
}

// GetCommitsAhead returns the number of commits ahead of the comparison ref.
// This counts revisions in bookmark..comparisonRef range.
func GetCommitsAhead(repoRoot, bookmarkName, comparisonRef string) int {
	revset := fmt.Sprintf("%s..%s", comparisonRef, bookmarkName)
	return countRevisions(repoRoot, revset)
}

// GetCommitsBehind returns the number of commits behind the comparison ref.
func GetCommitsBehind(repoRoot, bookmarkName, comparisonRef string) int {
	revset := fmt.Sprintf("%s..%s", bookmarkName, comparisonRef)
	return countRevisions(repoRoot, revset)
}

// IsMerged checks if a bookmark's target is an ancestor of the comparison ref.
func IsMerged(repoRoot, bookmarkName, comparisonRef string) bool {
	// If the bookmark is an ancestor of the comparison ref, it's merged.
	// We check: are there zero commits in comparisonRef..bookmarkName?
	// If bookmark is behind or at comparisonRef, it's merged.
	// More precisely: a bookmark is "merged" if all its commits are reachable from comparisonRef.
	revset := fmt.Sprintf("(%s ~ ::(%s))", bookmarkName, comparisonRef)
	return countRevisions(repoRoot, revset) == 0
}

// IsWorkingCopyMerged checks if the workspace's working copy parent(s) are ancestors of the comparison ref.
// This is useful when the bookmark has been deleted (e.g., after a remote merge).
func IsWorkingCopyMerged(workspaceDir, comparisonRef string) bool {
	// Check if the parents of @ are all ancestors of the comparison ref.
	// If so, the work from this workspace has been merged.
	revset := fmt.Sprintf("(@- ~ ::(%s))", comparisonRef)
	return countRevisions(workspaceDir, revset) == 0
}

// IsWorkingCopyEmpty checks if the working copy commit in a workspace has changes.
func IsWorkingCopyEmpty(workspaceDir string) bool {
	cmd := exec.Command("jj", "log", "-r", "@", "--no-graph", "--template", `if(empty, "true", "false")`)
	cmd.Dir = workspaceDir
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// GetRepoRoot returns the jj repository root for the current directory.
func GetRepoRoot(dir string) (string, error) {
	cmd := exec.Command("jj", "root")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a jj repository: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GitFetch runs jj git fetch to update remote state.
func GitFetch(repoRoot string) error {
	cmd := exec.Command("jj", "git", "fetch")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GitFetchQuiet runs jj git fetch without output.
func GitFetchQuiet(repoRoot string) error {
	cmd := exec.Command("jj", "git", "fetch")
	cmd.Dir = repoRoot
	return cmd.Run()
}

// countRevisions counts the number of revisions matching a revset.
func countRevisions(repoRoot, revset string) int {
	cmd := exec.Command("jj", "log", "-r", revset, "--no-graph", "--template", `commit_id ++ "\n"`)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return 0
	}
	return len(strings.Split(text, "\n"))
}

// --- Metadata storage ---
// Metadata is stored in .jjw/ at the repo root, per workspace.

func metadataDir(repoRoot, workspaceName string) string {
	return filepath.Join(repoRoot, ".jjw", "workspaces", workspaceName)
}

// SetCreatedAt stores the creation timestamp for a workspace.
func SetCreatedAt(repoRoot, workspaceName string, t time.Time) error {
	dir := metadataDir(repoRoot, workspaceName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "created-at"), []byte(strconv.FormatInt(t.Unix(), 10)+"\n"), 0644)
}

// GetCreatedAt retrieves the creation timestamp for a workspace.
func GetCreatedAt(repoRoot, workspaceName string) (time.Time, error) {
	data, err := os.ReadFile(filepath.Join(metadataDir(repoRoot, workspaceName), "created-at"))
	if err != nil {
		return time.Time{}, err
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(ts, 0), nil
}

// SetIndex stores the workspace index.
func SetIndex(repoRoot, workspaceName string, index int) error {
	dir := metadataDir(repoRoot, workspaceName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "index"), []byte(strconv.Itoa(index)+"\n"), 0644)
}

// GetIndex retrieves the workspace index.
func GetIndex(repoRoot, workspaceName string) (int, error) {
	data, err := os.ReadFile(filepath.Join(metadataDir(repoRoot, workspaceName), "index"))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// AllocateIndex finds the lowest unused index for a new workspace.
func AllocateIndex(repoRoot string, maxIndex int) (int, error) {
	used := make(map[int]bool)

	workspacesDir := filepath.Join(repoRoot, ".jjw", "workspaces")
	entries, err := os.ReadDir(workspacesDir)
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}

	for _, entry := range entries {
		if idx, err := GetIndex(repoRoot, entry.Name()); err == nil {
			used[idx] = true
		}
	}

	for i := 1; ; i++ {
		if maxIndex > 0 && i > maxIndex {
			return 0, fmt.Errorf("no available index: all indexes 1-%d are in use", maxIndex)
		}
		if !used[i] {
			return i, nil
		}
	}
}

// CleanupMetadata removes stored metadata for a workspace.
func CleanupMetadata(repoRoot, workspaceName string) error {
	return os.RemoveAll(metadataDir(repoRoot, workspaceName))
}

// GetWorkspaceStatus gathers status information for a workspace.
// jjRoot is the jj repository root (for jj CLI commands).
// repoRoot is the jjw root (for metadata storage).
func GetWorkspaceStatus(jjRoot, repoRoot, workspacePath, workspaceName, bookmarkName, comparisonRef string) *WorkspaceStatus {
	status := &WorkspaceStatus{}

	status.CommitsAhead = GetCommitsAhead(jjRoot, bookmarkName, comparisonRef)
	status.CommitsBehind = GetCommitsBehind(jjRoot, bookmarkName, comparisonRef)
	status.IsMerged = IsMerged(jjRoot, bookmarkName, comparisonRef)
	status.IsEmpty = IsWorkingCopyEmpty(workspacePath)

	if t, err := GetCreatedAt(repoRoot, workspaceName); err == nil {
		status.CreatedAt = t
	}
	if idx, err := GetIndex(repoRoot, workspaceName); err == nil {
		status.Index = idx
	}

	return status
}
