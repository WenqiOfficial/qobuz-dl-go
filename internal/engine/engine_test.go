package engine

import (
	"testing"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"
)

func TestIsLikelyPreviewTrack(t *testing.T) {
	tests := []struct {
		name     string
		duration int
		preview  bool
	}{
		{name: "thirty seconds", duration: 30, preview: true},
		{name: "boundary", duration: 45, preview: true},
		{name: "zero duration", duration: 0, preview: false},
		{name: "full track", duration: 120, preview: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLikelyPreviewTrack(tt.duration); got != tt.preview {
				t.Fatalf("isLikelyPreviewTrack(%d) = %v, want %v", tt.duration, got, tt.preview)
			}
		})
	}
}

func TestFilterPreviewTasks(t *testing.T) {
	tasks := []trackTask{
		{Track: api.TrackMetadata{Title: "Preview", Duration: 30}},
		{Track: api.TrackMetadata{Title: "Full track", Duration: 180}},
		{Track: api.TrackMetadata{Title: "Rejected preview", Duration: 30}},
	}
	var prompted []string

	filtered := filterPreviewTasks(tasks, func(title string) bool {
		prompted = append(prompted, title)
		return title == "Preview"
	})

	if len(prompted) != 2 || prompted[0] != "Preview" || prompted[1] != "Rejected preview" {
		t.Fatalf("prompted for %v, want both preview tracks in order", prompted)
	}
	if len(filtered) != 2 || filtered[0].Track.Title != "Preview" || filtered[1].Track.Title != "Full track" {
		t.Fatalf("filtered tasks = %v, want approved preview and full track", filtered)
	}
}