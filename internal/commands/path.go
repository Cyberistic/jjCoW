package commands

import (
	"path/filepath"
	"strings"
)

// isPathWithin reports whether path is base itself or is contained within base.
func isPathWithin(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}
