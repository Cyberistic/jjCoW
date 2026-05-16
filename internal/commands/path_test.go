package commands

import "testing"

func TestIsPathWithin(t *testing.T) {
	tests := []struct {
		name string
		path string
		base string
		want bool
	}{
		{
			name: "same path",
			path: "/repo/workspaces/foo",
			base: "/repo/workspaces/foo",
			want: true,
		},
		{
			name: "child path",
			path: "/repo/workspaces/foo/src/file.go",
			base: "/repo/workspaces/foo",
			want: true,
		},
		{
			name: "sibling with shared prefix",
			path: "/repo/workspaces/foo2",
			base: "/repo/workspaces/foo",
			want: false,
		},
		{
			name: "parent path",
			path: "/repo/workspaces",
			base: "/repo/workspaces/foo",
			want: false,
		},
		{
			name: "different absolute path",
			path: "/other/workspaces/foo",
			base: "/repo/workspaces/foo",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathWithin(tt.path, tt.base)
			if got != tt.want {
				t.Fatalf("isPathWithin(%q, %q) = %v, want %v", tt.path, tt.base, got, tt.want)
			}
		})
	}
}
