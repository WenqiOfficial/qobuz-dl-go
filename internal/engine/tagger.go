// tagger.go provides audio metadata tagging functionality.
// Supports FLAC (Vorbis Comments) and MP3 (ID3v2.4) formats.
package engine

import (
	"fmt"
	"strings"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"

	"github.com/go-flac/go-flac"
)

// Tagger handles metadata embedding for audio files.
type Tagger struct{}

// NewTagger creates a new Tagger instance.
func NewTagger() *Tagger {
	return &Tagger{}
}

// WriteTags writes metadata tags and optional cover art to an audio file.
// It automatically detects the file format based on extension and uses
// the appropriate tagging method (Vorbis Comments for FLAC, ID3v2 for MP3).
func (t *Tagger) WriteTags(filePath string, track *api.TrackMetadata, album *api.AlbumMetadata, coverData []byte) error {
	lowerPath := strings.ToLower(filePath)

	switch {
	case strings.HasSuffix(lowerPath, ".mp3"):
		return t.WriteMp3Tags(filePath, track, album, coverData)
	case strings.HasSuffix(lowerPath, ".flac"):
		return t.WriteFlacTags(filePath, track, album, coverData)
	default:
		// Default to FLAC for unknown extensions
		return t.WriteFlacTags(filePath, track, album, coverData)
	}
}

// WriteFlacTags writes Vorbis Comments and Picture block to a FLAC file.
// Implements Vorbis Comment specification (https://xiph.org/vorbis/doc/v-comment.html).
func (t *Tagger) WriteFlacTags(filePath string, track *api.TrackMetadata, album *api.AlbumMetadata, coverData []byte) error {
	f, err := flac.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse flac file: %w", err)
	}

	// 1. Vorbis Comments
	var cmts *VorbisComment
	var cmtsIndex int
	foundCmts := false

	for i, block := range f.Meta {
		if block.Type == flac.VorbisComment {
			cmts, err = ParseVorbisComment(block.Data)
			if err != nil {
				return fmt.Errorf("failed to parse existing comments: %w", err)
			}
			foundCmts = true
			cmtsIndex = i
			break
		}
	}

	if cmts == nil {
		cmts = NewVorbisComment()
	}

	// === Basic Tags ===
	addTag(cmts, "TITLE", track.Title)
	addTag(cmts, "VERSION", track.Version)
	addTag(cmts, "ARTIST", track.Performer.Name)
	addTag(cmts, "ALBUM", album.Title)
	addTag(cmts, "ALBUMARTIST", album.Artist.Name)

	// === Track/Disc Numbers ===
	addTag(cmts, "TRACKNUMBER", fmt.Sprintf("%d", track.TrackNumber))
	addTag(cmts, "TRACKTOTAL", fmt.Sprintf("%d", album.TracksCount))
	addTag(cmts, "DISCNUMBER", fmt.Sprintf("%d", track.MediaNumber))
	if album.MediaCount > 0 {
		addTag(cmts, "DISCTOTAL", fmt.Sprintf("%d", album.MediaCount))
	}

	// === Genre ===
	if album.Genre != nil && album.Genre.Name != "" {
		addTag(cmts, "GENRE", album.Genre.Name)
	}
	// Also add full genres list if available
	if len(album.GenresList) > 0 {
		addTag(cmts, "GENRES", formatGenresList(album.GenresList))
	}

	// === Dates ===
	if album.ReleaseDateOrg != "" {
		addTag(cmts, "DATE", album.ReleaseDateOrg)
		addTag(cmts, "ORIGINALDATE", album.ReleaseDateOrg)
		// Extract year
		if len(album.ReleaseDateOrg) >= 4 {
			addTag(cmts, "YEAR", album.ReleaseDateOrg[:4])
		}
	} else if album.ReleaseDateStream != "" {
		addTag(cmts, "DATE", album.ReleaseDateStream)
	}

	// === Composer (important for classical music) ===
	// First try track.Composer, then extract from performers string
	if track.Composer != nil && track.Composer.Name != "" {
		addTag(cmts, "COMPOSER", track.Composer.Name)
	} else if composers := extractRoles(track.Performers, "Composer"); composers != "" {
		addTag(cmts, "COMPOSER", composers)
	} else if album.Composer != nil && album.Composer.Name != "" && album.Composer.Name != "Various Composers" {
		addTag(cmts, "COMPOSER", album.Composer.Name)
	}

	// === Work (classical music) ===
	if track.Work != "" {
		addTag(cmts, "WORK", track.Work)
	}

	// === Performers (full credits) ===
	if track.Performers != "" {
		addTag(cmts, "PERFORMER", track.Performers)

		// Extract specific roles from performers string
		addTag(cmts, "LYRICIST", extractRoles(track.Performers, "Lyricist"))
		addTag(cmts, "CONDUCTOR", extractRoles(track.Performers, "Conductor"))
		addTag(cmts, "PRODUCER", extractRoles(track.Performers, "Producer"))
		addTag(cmts, "ARRANGER", extractRoles(track.Performers, "Arranger"))
		addTag(cmts, "ENGINEER", extractRoles(track.Performers, "Engineer"))
		addTag(cmts, "MIXER", extractRoles(track.Performers, "Mixer"))
	}

	// === Label ===
	if album.Label != nil && album.Label.Name != "" {
		addTag(cmts, "LABEL", album.Label.Name)
		addTag(cmts, "ORGANIZATION", album.Label.Name)
	}

	// === Copyright ===
	if track.Copyright != "" {
		addTag(cmts, "COPYRIGHT", formatCopyright(track.Copyright))
	} else if album.Copyright != "" {
		addTag(cmts, "COPYRIGHT", formatCopyright(album.Copyright))
	}

	// === Identifiers ===
	if track.ISRC != "" {
		addTag(cmts, "ISRC", track.ISRC)
	}
	if album.UPC != "" {
		addTag(cmts, "BARCODE", album.UPC)
		addTag(cmts, "UPC", album.UPC)
	}

	// === Release Type ===
	if album.ReleaseType != "" {
		addTag(cmts, "RELEASETYPE", album.ReleaseType)
	}

	// === ReplayGain (follows ReplayGain 2.0 specification) ===
	// Gain format: "-6.47 dB", Peak format: "0.967834"
	if track.AudioInfo != nil {
		if track.AudioInfo.ReplayGainTrackGain != 0 {
			addTag(cmts, "REPLAYGAIN_TRACK_GAIN", fmt.Sprintf("%.2f dB", track.AudioInfo.ReplayGainTrackGain))
		}
		if track.AudioInfo.ReplayGainTrackPeak != 0 {
			addTag(cmts, "REPLAYGAIN_TRACK_PEAK", fmt.Sprintf("%.6f", track.AudioInfo.ReplayGainTrackPeak))
		}
	}

	// === Description/Comment ===
	if album.Description != "" {
		addTag(cmts, "DESCRIPTION", album.Description)
	}

	// Re-serialize comments block
	resCmts := cmts.Marshal()

	if foundCmts {
		f.Meta[cmtsIndex].Data = resCmts
	} else {
		f.Meta = append(f.Meta, &flac.MetaDataBlock{
			Type: flac.VorbisComment,
			Data: resCmts,
		})
	}

	// 2. Cover Art (Picture Block)
	if len(coverData) > 0 {
		pic := NewPicture()
		pic.MIME = "image/jpeg"
		pic.Description = "Cover"
		pic.PictureType = PictureTypeCoverFront
		pic.ImageData = coverData

		f.Meta = append(f.Meta, &flac.MetaDataBlock{
			Type: flac.Picture,
			Data: pic.Marshal(),
		})
	}

	if err = f.Save(filePath); err != nil {
		return fmt.Errorf("failed to save tags: %w", err)
	}
	return nil
}

// addTag adds a Vorbis Comment tag. Skips empty values.
func addTag(cmts *VorbisComment, key, value string) {
	if value == "" {
		return
	}
	cmts.Add(strings.ToUpper(key), value)
}

// formatGenresList extracts unique genres from Qobuz's hierarchical genre list.
// Input format: ["Pop/Rock", "Pop/Rock→Rock", "Pop/Rock→Rock→Alternative"]
// Output: "Pop, Rock, Alternative"
func formatGenresList(genres []string) string {
	if len(genres) == 0 {
		return ""
	}

	seen := make(map[string]bool)
	var result []string

	for _, g := range genres {
		parts := strings.FieldsFunc(g, func(r rune) bool {
			return r == '→' || r == '/'
		})
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" && !seen[p] {
				seen[p] = true
				result = append(result, p)
			}
		}
	}

	return strings.Join(result, ", ")
}

// formatCopyright normalizes copyright strings by converting (P) and (C) to proper symbols.
func formatCopyright(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "(P)", "℗")
	s = strings.ReplaceAll(s, "(C)", "©")
	return s
}

// extractRoles extracts all performer names for a specific role from the performers string.
// Input format: "Mili, MainArtist - YAMATO KASAI, Composer - Cassie Wei, Composer, Lyricist"
// Returns names separated by semicolons for the given role.
// Example: extractRoles(performers, "Composer") -> "YAMATO KASAI; Cassie Wei"
func extractRoles(performers, role string) string {
	if performers == "" || role == "" {
		return ""
	}

	var names []string

	// Split by " - " to get individual performer entries
	parts := strings.Split(performers, " - ")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split by ", " to separate name from roles
		// Format: "Name, Role1, Role2" or "Name, Role"
		fields := strings.Split(part, ", ")
		if len(fields) < 2 {
			continue
		}

		name := strings.TrimSpace(fields[0])
		roles := fields[1:]

		// Check if any of the roles match
		for _, r := range roles {
			if strings.EqualFold(strings.TrimSpace(r), role) {
				names = append(names, name)
				break
			}
		}
	}

	return strings.Join(names, "; ")
}
