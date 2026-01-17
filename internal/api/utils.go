package api

import (
	"fmt"
	"regexp"
)

var (
	urlRegex = regexp.MustCompile(`(?:https:\/\/(?:w{3}|open|play)\.qobuz\.com)?(?:\/[a-z]{2}-[a-z]{2})?\/(album|artist|track|playlist|label)(?:\/[-\w\d]+)?\/([\w\d]+)`)
)

// ResourceType represents the type of Qobuz resource
type ResourceType string

const (
	TypeAlbum    ResourceType = "album"
	TypeArtist   ResourceType = "artist"
	TypeTrack    ResourceType = "track"
	TypePlaylist ResourceType = "playlist"
	TypeLabel    ResourceType = "label"
)

// ParseURL extracts the resource type and ID from a Qobuz URL.
// If the input is just digits, it assumes it's a Track ID (or lets the caller decide).
// But since CLI args are ambiguous, we should helper here.
// However, the Python regex is strict.
func ParseURL(input string) (ResourceType, string, error) {
	// 1. Try Regex
	matches := urlRegex.FindStringSubmatch(input)
	if len(matches) == 3 {
		return ResourceType(matches[1]), matches[2], nil
	}

	// 2. If no match, check if it is raw digits.
	// If raw digits, we default to Track? Or maybe the user has to specify.
	// For now, if it's just digits, we assume it is the ID provided for the specific command context.
	// But robustly, we only handle URL parsing here.
	return "", "", fmt.Errorf("invalid Qobuz URL")
}
