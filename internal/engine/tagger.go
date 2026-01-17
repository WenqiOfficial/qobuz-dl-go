package engine

import (
	"fmt"
	"strings"

	"qobuz-dl-go/internal/api"

	"github.com/go-flac/go-flac"
)

// Tagger handles metadata embedding
type Tagger struct{}

func NewTagger() *Tagger {
	return &Tagger{}
}

// WriteTags executes the tagging process on a FLAC file
func (t *Tagger) WriteTags(filePath string, track *api.TrackMetadata, album *api.AlbumMetadata, coverData []byte) error {
	f, err := flac.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse flac file: %w", err)
	}

	// 1. Vorbis Comments (Text Tags)
	var cmts *VorbisComment

	// Check if exists
	foundCmts := false
	var cmtsIndex int

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

	// If not, create new
	if cmts == nil {
		cmts = NewVorbisComment()
	}

	// Add Tags
	addTag(cmts, "TITLE", track.Title)
	addTag(cmts, "VERSION", track.Version)
	addTag(cmts, "ARTIST", track.Performer.Name)
	addTag(cmts, "ALBUM", album.Title)
	addTag(cmts, "ALBUMARTIST", album.Artist.Name)
	addTag(cmts, "TRACKNUMBER", fmt.Sprintf("%d", track.TrackNumber))
	addTag(cmts, "DISCNUMBER", fmt.Sprintf("%d", track.MediaNumber))

	if album.Genre != nil {
		addTag(cmts, "GENRE", album.Genre.Name)
	}
	if album.ReleaseDateOrg != "" {
		addTag(cmts, "DATE", album.ReleaseDateOrg)
	} else if album.ReleaseDateStream != "" {
		addTag(cmts, "DATE", album.ReleaseDateStream)
	}

	// Re-serialize comments block
	resCmts := cmts.Marshal()

	// Update or Append block
	if foundCmts {
		f.Meta[cmtsIndex].Data = resCmts
	} else {
		f.Meta = append(f.Meta, &flac.MetaDataBlock{
			Type: flac.VorbisComment, // 4
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

		picBlock := pic.Marshal()

		f.Meta = append(f.Meta, &flac.MetaDataBlock{
			Type: flac.Picture, // 6
			Data: picBlock,
		})
	}

	// 3. Save
	err = f.Save(filePath)
	if err != nil {
		return fmt.Errorf("failed to save tags: %w", err)
	}

	return nil
}

func addTag(cmts *VorbisComment, key, value string) {
	if value == "" {
		return
	}
	cmts.Add(strings.ToUpper(key), value)
}
