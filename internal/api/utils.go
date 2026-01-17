package api

import (
	"fmt"
	"regexp"
)

// urlRegex matches various Qobuz URL formats and extracts resource type and ID.
// Supports: www.qobuz.com, open.qobuz.com, play.qobuz.com with optional locale prefix.
var urlRegex = regexp.MustCompile(`(?:https:\/\/(?:w{3}|open|play)\.qobuz\.com)?` +
	`(?:\/[a-z]{2}-[a-z]{2})?\/(album|artist|track|playlist|label)(?:\/[-\w\d]+)?\/([\w\d]+)`)

// ResourceType represents the type of Qobuz resource (album, track, etc.).
type ResourceType string

// Supported Qobuz resource types.
const (
	TypeAlbum    ResourceType = "album"
	TypeArtist   ResourceType = "artist"
	TypeTrack    ResourceType = "track"
	TypePlaylist ResourceType = "playlist"
	TypeLabel    ResourceType = "label"
)

// ParseURL extracts the resource type and ID from a Qobuz URL.
// Supports URLs from www.qobuz.com, open.qobuz.com, and play.qobuz.com.
// Returns an error if the URL format is not recognized.
func ParseURL(input string) (ResourceType, string, error) {
	matches := urlRegex.FindStringSubmatch(input)
	if len(matches) == 3 {
		return ResourceType(matches[1]), matches[2], nil
	}
	return "", "", fmt.Errorf("invalid Qobuz URL format")
}
