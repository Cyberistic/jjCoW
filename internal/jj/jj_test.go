package jj

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseBookmarkRef(t *testing.T) {
	tests := []struct {
		ref      string
		name     string
		remote   string
		isRemote bool
	}{
		{"foo", "foo", "", false},
		{"foo@origin", "foo", "origin", true},
		{"feature@x@upstream", "feature@x", "upstream", true},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			name, remote, isRemote := ParseBookmarkRef(tt.ref)
			if name != tt.name || remote != tt.remote || isRemote != tt.isRemote {
				t.Fatalf("ParseBookmarkRef(%q) = (%q, %q, %v)", tt.ref, name, remote, isRemote)
			}
		})
	}
}

func TestMetadataCreatedAtAndIndex(t *testing.T) {
	dir := t.TempDir()
	created := time.Unix(1700000000, 0)
	if err := SetCreatedAt(dir, "one", created); err != nil {
		t.Fatal(err)
	}
	gotCreated, err := GetCreatedAt(dir, "one")
	if err != nil {
		t.Fatal(err)
	}
	if !gotCreated.Equal(created) {
		t.Fatalf("created = %v, want %v", gotCreated, created)
	}

	if err := SetIndex(dir, "one", 3); err != nil {
		t.Fatal(err)
	}
	idx, err := GetIndex(dir, "one")
	if err != nil {
		t.Fatal(err)
	}
	if idx != 3 {
		t.Fatalf("index = %d, want 3", idx)
	}
}

func TestGetCreatedAtInvalid(t *testing.T) {
	dir := t.TempDir()
	meta := metadataDir(dir, "bad")
	if err := os.MkdirAll(meta, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(meta, "created-at"), []byte("nope\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := GetCreatedAt(dir, "bad"); err == nil {
		t.Fatal("expected error for invalid created-at")
	}
}

func TestAllocateIndex(t *testing.T) {
	dir := t.TempDir()
	if idx, err := AllocateIndex(dir, 0); err != nil || idx != 1 {
		t.Fatalf("empty AllocateIndex = %d, %v; want 1, nil", idx, err)
	}
	if err := SetIndex(dir, "one", 1); err != nil {
		t.Fatal(err)
	}
	if err := SetIndex(dir, "three", 3); err != nil {
		t.Fatal(err)
	}
	if idx, err := AllocateIndex(dir, 0); err != nil || idx != 2 {
		t.Fatalf("AllocateIndex gap = %d, %v; want 2, nil", idx, err)
	}
	if _, err := AllocateIndex(dir, 1); err == nil {
		t.Fatal("expected max index error")
	}
}

func TestCleanupMetadata(t *testing.T) {
	dir := t.TempDir()
	if err := SetIndex(dir, "one", 1); err != nil {
		t.Fatal(err)
	}
	if err := CleanupMetadata(dir, "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(metadataDir(dir, "one")); !os.IsNotExist(err) {
		t.Fatalf("metadata still exists or unexpected error: %v", err)
	}
}
