// mp3_metadata.go provides MP3 ID3v2 metadata tagging functionality.
package engine

import (
	"fmt"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"

	"github.com/bogem/id3v2/v2"
)

// WriteMp3Tags writes ID3v2 metadata tags and optional cover art to an MP3 file.
func (t *Tagger) WriteMp3Tags(filePath string, track *api.TrackMetadata, album *api.AlbumMetadata, coverData []byte) error {
	// Open MP3 file for tag editing
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("failed to open mp3 file: %w", err)
	}
	defer tag.Close()

	// Set encoding to UTF-8 for proper unicode support
	tag.SetDefaultEncoding(id3v2.EncodingUTF8)

	// Set text frames
	tag.SetTitle(track.Title)
	tag.SetArtist(track.Performer.Name)
	tag.SetAlbum(album.Title)

	// Album artist (TPE2)
	if album.Artist.Name != "" {
		tag.AddTextFrame("TPE2", id3v2.EncodingUTF8, album.Artist.Name)
	}

	// Track number (TRCK)
	if track.TrackNumber > 0 {
		tag.AddTextFrame("TRCK", id3v2.EncodingUTF8, fmt.Sprintf("%d", track.TrackNumber))
	}

	// Disc number (TPOS)
	if track.MediaNumber > 0 {
		tag.AddTextFrame("TPOS", id3v2.EncodingUTF8, fmt.Sprintf("%d", track.MediaNumber))
	}

	// Genre (TCON)
	if album.Genre != nil && album.Genre.Name != "" {
		tag.SetGenre(album.Genre.Name)
	}

	// Year/Date (TDRC for ID3v2.4, TYER for ID3v2.3)
	if album.ReleaseDateOrg != "" {
		tag.SetYear(album.ReleaseDateOrg)
	} else if album.ReleaseDateStream != "" {
		tag.SetYear(album.ReleaseDateStream)
	}

	// Version/Subtitle (TIT3)
	if track.Version != "" {
		tag.AddTextFrame("TIT3", id3v2.EncodingUTF8, track.Version)
	}

	// Cover art (APIC - Attached Picture)
	if len(coverData) > 0 {
		pic := id3v2.PictureFrame{
			Encoding:    id3v2.EncodingUTF8,
			MimeType:    "image/jpeg",
			PictureType: id3v2.PTFrontCover,
			Description: "Cover",
			Picture:     coverData,
		}
		tag.AddAttachedPicture(pic)
	}

	// Save the tags
	if err := tag.Save(); err != nil {
		return fmt.Errorf("failed to save mp3 tags: %w", err)
	}

	return nil
}
