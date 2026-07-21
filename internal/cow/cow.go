// Package cow provides copy-on-write directory cloning, ported from rift
// (https://github.com/anomalyco/rift). On macOS it uses APFS clonefile(2);
// on Linux it uses the FICLONE ioctl (reflinks, supported by Btrfs, XFS,
// and others).
package cow

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// excludedComponents mirrors rift's filter: heavyweight regenerable
// dependency/build artifacts are skipped at any depth.
var excludedComponents = map[string]bool{
	"node_modules": true, ".pnpm-store": true,
	"target": true,
	".venv":  true, "venv": true, ".tox": true, ".nox": true,
	"__pycache__": true, ".pytest_cache": true, ".mypy_cache": true, ".ruff_cache": true,
	".next": true, ".nuxt": true, ".svelte-kit": true,
	".turbo": true, ".vite": true, ".parcel-cache": true, ".cache": true,
	"dist": true, "build": true, "coverage": true,
}

// excludesRel reports whether a workspace-relative path is filtered out.
func excludesRel(rel string) bool {
	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		if excludedComponents[part] {
			return true
		}
		// rift: Yarn artifacts (.yarn/cache, .yarn/unplugged, ...)
		if part == ".yarn" && i+1 < len(parts) {
			switch parts[i+1] {
			case "cache", "unplugged", "install-state.gz", "build-state.yml":
				return true
			}
		}
	}
	return false
}

type fileID struct {
	dev uint64
	ino uint64
}

type cloneTask struct {
	src   string
	dst   string
	mode  fs.FileMode
	atime time.Time
	mtime time.Time
}

type linkTask struct {
	target string
	link   string
}

// CloneDir clones the directory tree at src into dst (which may already
// exist as an empty or near-empty directory) using copy-on-write clones.
// Top-level entries whose name appears in excludeTop are skipped, as are
// rift-style regenerable artifacts at any depth. Symlinks are recreated;
// hard links are preserved.
func CloneDir(src, dst string, excludeTop map[string]bool) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	if err := precheck(src, dst); err != nil {
		return err
	}
	// Create writable; real permissions applied after the walk. The
	// destination may already exist (e.g. created by `jj workspace add`).
	if err := os.Mkdir(dst, srcInfo.Mode()|0o700); err != nil && !os.IsExist(err) {
		return err
	}

	seen := make(map[fileID]string)
	var dirs []struct {
		path string
		mode fs.FileMode
	}
	dirs = append(dirs, struct {
		path string
		mode fs.FileMode
	}{dst, srcInfo.Mode()})
	var clones []cloneTask
	var links []linkTask

	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			// Never descend into the destination if it lives inside the source.
			if pathAbs, aerr := filepath.Abs(path); aerr == nil && pathAbs == dstAbs {
				return filepath.SkipDir
			}
			if !strings.ContainsRune(rel, filepath.Separator) && excludeTop[rel] {
				return filepath.SkipDir
			}
			if excludesRel(rel) {
				return filepath.SkipDir
			}
		} else if excludesRel(rel) {
			return nil
		}

		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}

		switch {
		case d.IsDir():
			if err := os.MkdirAll(target, info.Mode()|0o700); err != nil {
				return err
			}
			dirs = append(dirs, struct {
				path string
				mode fs.FileMode
			}{target, info.Mode()})
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.Symlink(link, target); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if id, ok := fileIDOf(info); ok {
				if first, ok := seen[id]; ok {
					// Linked after the clone tasks run, when the
					// link target is guaranteed to exist.
					links = append(links, linkTask{target: first, link: target})
					return nil
				}
				seen[id] = target
			}
			atime, mtime := fileTimes(info)
			clones = append(clones, cloneTask{src: path, dst: target, mode: info.Mode(), atime: atime, mtime: mtime})
		default:
			// Skip fifos, sockets, devices: not meaningful in a workspace clone.
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err := runClones(clones); err != nil {
		return err
	}
	for _, l := range links {
		if err := os.Link(l.target, l.link); err != nil {
			return err
		}
	}

	// Apply directory permissions deepest-first so children of read-only
	// directories have already been written.
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chmod(dirs[i].path, dirs[i].mode); err != nil {
			return err
		}
	}
	return nil
}

// runClones executes file clone tasks. Measured on APFS, concurrent
// clonefile(2) calls contend and are slower than a serial loop.
func runClones(tasks []cloneTask) error {
	for _, t := range tasks {
		if err := cloneFile(t.src, t.dst); err != nil {
			return err
		}
		if clonePreservesMeta {
			continue
		}
		if err := os.Chmod(t.dst, t.mode); err != nil {
			return err
		}
		_ = os.Chtimes(t.dst, t.atime, t.mtime)
	}
	return nil
}
