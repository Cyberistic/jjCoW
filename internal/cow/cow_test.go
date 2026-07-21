package cow

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestCloneDir_Basic(t *testing.T) {
	if !Supported() {
		t.Skip("CoW not supported on this platform")
	}
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	os.MkdirAll(filepath.Join(src, "sub"), 0o755)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("world"), 0o644)
	os.MkdirAll(filepath.Join(src, "node_modules", "pkg"), 0o755)
	os.WriteFile(filepath.Join(src, "node_modules", "pkg", "c.js"), []byte("y"), 0o644)
	os.Symlink("a.txt", filepath.Join(src, "link"))

	if err := CloneDir(src, dst, map[string]bool{".jj": true}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "a.txt")); string(b) != "hello" {
		t.Fatalf("a.txt mismatch: %q", b)
	}
	if _, err := os.Lstat(filepath.Join(dst, "node_modules")); !os.IsNotExist(err) {
		t.Fatal("node_modules should be excluded")
	}
	if l, _ := os.Readlink(filepath.Join(dst, "link")); l != "a.txt" {
		t.Fatalf("symlink mismatch: %q", l)
	}
}

// Ported from rift filter.rs: excludes_artifacts_at_any_depth.
func TestExcludesRel(t *testing.T) {
	cases := []struct {
		path    string
		exclude bool
	}{
		{"packages/app/node_modules/react/index.js", true},
		{"packages/app/.yarn/cache/react.zip", true},
		{"packages/app/.yarn/unplugged/foo", true},
		{"packages/app/package-lock.json", false},
		{"src/main.go", false},
		{"target/debug/foo", true},
		{"dist/index.js", true},
		{"build/out", true},
		{"coverage/lcov.info", true},
		{".next/cache", true},
		{".venv/bin/python", true},
		{"__pycache__/x.pyc", true},
		{".jjw.yaml", false},
	}
	for _, tc := range cases {
		if got := excludesRel(filepath.FromSlash(tc.path)); got != tc.exclude {
			t.Errorf("excludesRel(%q)=%v, want %v", tc.path, got, tc.exclude)
		}
	}
}

// Ported from rift apfs.rs: filtered_strategy_preserves_included_metadata_and_hard_links.
func TestCloneDir_MetadataHardLinksAndFilter(t *testing.T) {
	if !Supported() {
		t.Skip("CoW not supported on this platform")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("unix-only metadata checks")
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	dst := filepath.Join(tmp, "destination")
	nested := filepath.Join(src, "nested")
	os.MkdirAll(nested, 0o755)
	os.Chmod(src, 0o750)
	os.Chmod(nested, 0o710)

	file := filepath.Join(nested, "file.txt")
	os.WriteFile(file, []byte("hello"), 0o640)
	os.Chmod(file, 0o640)
	if err := os.Link(file, filepath.Join(nested, "hard.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("file.txt", filepath.Join(nested, "link.txt")); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(src, "node_modules", "pkg"), 0o755)
	os.WriteFile(filepath.Join(src, "node_modules", "pkg", "index.js"), []byte("module"), 0o644)
	os.MkdirAll(filepath.Join(src, "packages", "app", "node_modules", "x"), 0o755)
	os.WriteFile(filepath.Join(src, "packages", "app", "node_modules", "x", "i.js"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(src, ".yarn", "cache"), 0o755)
	os.WriteFile(filepath.Join(src, ".yarn", "cache", "pkg.zip"), []byte("z"), 0o644)
	os.WriteFile(filepath.Join(src, "package-lock.json"), []byte("{}"), 0o644)

	if err := CloneDir(src, dst, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Lstat(filepath.Join(dst, "node_modules")); !os.IsNotExist(err) {
		t.Fatal("top-level node_modules should be excluded")
	}
	if _, err := os.Lstat(filepath.Join(dst, "packages", "app", "node_modules")); !os.IsNotExist(err) {
		t.Fatal("nested node_modules should be excluded")
	}
	if _, err := os.Lstat(filepath.Join(dst, ".yarn", "cache")); !os.IsNotExist(err) {
		t.Fatal(".yarn/cache should be excluded")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "package-lock.json")); string(b) != "{}" {
		t.Fatal("package-lock.json should be preserved")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "nested", "file.txt")); string(b) != "hello" {
		t.Fatal("file content mismatch")
	}
	if l, _ := os.Readlink(filepath.Join(dst, "nested", "link.txt")); l != "file.txt" {
		t.Fatalf("symlink: %q", l)
	}

	// Hard links must share an inode in the destination.
	fi1, err := os.Stat(filepath.Join(dst, "nested", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	fi2, err := os.Stat(filepath.Join(dst, "nested", "hard.txt"))
	if err != nil {
		t.Fatal(err)
	}
	s1, ok1 := fi1.Sys().(*syscall.Stat_t)
	s2, ok2 := fi2.Sys().(*syscall.Stat_t)
	if !ok1 || !ok2 {
		t.Fatal("expected syscall.Stat_t")
	}
	if s1.Ino != s2.Ino {
		t.Fatalf("hard links should share inode: %d vs %d", s1.Ino, s2.Ino)
	}

	mode := fi1.Mode().Perm()
	if mode != 0o640 {
		t.Fatalf("file mode: got %o want 640", mode)
	}
	// Directory modes (clonefile preserves them on darwin; linux applies via chmod).
	if di, err := os.Stat(filepath.Join(dst, "nested")); err == nil {
		if di.Mode().Perm() != 0o710 {
			t.Fatalf("nested dir mode: got %o want 710", di.Mode().Perm())
		}
	}
}

func TestCloneDir_SkipsDestinationInsideSource(t *testing.T) {
	if !Supported() {
		t.Skip("CoW not supported on this platform")
	}
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644)
	// Pre-create a nested destination (as jj workspace add does) under src.
	dst := filepath.Join(src, "workspaces", "ws1")
	os.MkdirAll(filepath.Join(dst, ".jj"), 0o755)
	os.WriteFile(filepath.Join(dst, ".jj", "repo"), []byte("x"), 0o644)

	if err := CloneDir(src, dst, map[string]bool{"workspaces": true, ".jj": true}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "a.txt")); string(b) != "a" {
		t.Fatal("expected clone of a.txt")
	}
	// Must not recurse into itself (no workspaces/ws1/workspaces/...).
	if _, err := os.Lstat(filepath.Join(dst, "workspaces")); !os.IsNotExist(err) {
		t.Fatal("should not clone workspaces/ into itself")
	}
	// Destination's own .jj must remain (not overwritten by source .jj, which is excluded).
	if _, err := os.Stat(filepath.Join(dst, ".jj", "repo")); err != nil {
		t.Fatal("destination .jj should be preserved")
	}
}

func TestCloneDir_TopLevelExclude(t *testing.T) {
	if !Supported() {
		t.Skip("CoW not supported on this platform")
	}
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	os.MkdirAll(filepath.Join(src, ".jj"), 0o755)
	os.WriteFile(filepath.Join(src, ".jj", "repo"), []byte("jj"), 0o644)
	os.WriteFile(filepath.Join(src, "keep.txt"), []byte("k"), 0o644)

	if err := CloneDir(src, dst, map[string]bool{".jj": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dst, ".jj")); !os.IsNotExist(err) {
		t.Fatal(".jj should be top-level excluded")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "keep.txt")); string(b) != "k" {
		t.Fatal("keep.txt missing")
	}
}
