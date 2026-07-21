package commands

import (
	"testing"
	"time"

	"github.com/Cyberistic/jjCoW/internal/jj"
)

func TestFormatStatus(t *testing.T) {
	tests := []struct {
		name   string
		status *jj.WorkspaceStatus
		want   string
	}{
		{"nil", nil, "[no bookmark]"},
		{"ok", &jj.WorkspaceStatus{}, "[ok]"},
		{"merged empty ahead behind", &jj.WorkspaceStatus{IsMerged: true, IsEmpty: true, CommitsAhead: 2, CommitsBehind: 1}, "[merged empty ↑2 ↓1]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatStatus(tt.status); got != tt.want {
				t.Fatalf("formatStatus = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{time.Minute, "1 minute"},
		{2 * time.Minute, "2 minutes"},
		{time.Hour, "1 hour"},
		{3 * time.Hour, "3 hours"},
		{24 * time.Hour, "1 day"},
		{48 * time.Hour, "2 days"},
	}
	for _, tt := range tests {
		if got := formatDuration(tt.d); got != tt.want {
			t.Fatalf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
