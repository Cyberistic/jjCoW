//go:build linux

package cow

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCloneOrCowError verifies Linux behavior across filesystems: on
// reflink-capable filesystems (Btrfs, XFS, ...) CloneDir produces a full
// clone; elsewhere it must fail cleanly with a CowError before copying
// anything, so callers can fall back to a regular checkout.
func TestCloneOrCowError(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := CloneDir(src, dst, nil)
	if err != nil {
		if !IsCowError(err) {
			t.Fatalf("expected CowError on non-reflink filesystem, got %T: %v", err, err)
		}
		entries, _ := os.ReadDir(dst)
		for _, e := range entries {
			if e.Name() == "a.txt" {
				t.Fatal("precheck should fail before copying any files")
			}
		}
		return
	}

	b, err := os.ReadFile(filepath.Join(dst, "a.txt"))
	if err != nil || string(b) != "hello" {
		t.Fatalf("clone content mismatch: %q, %v", b, err)
	}
}
