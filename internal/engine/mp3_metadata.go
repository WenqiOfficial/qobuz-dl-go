// mp3_metadata.go provides MP3 ID3v2 metadata tagging functionality.
package engine

import (
	"fmt"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"

	"github.com/bogem/id3v2/v2"
)

// WriteMp3Tags writes ID3v2 metadata tags and optional cover art to an MP3 file.
// Implements ID3v2.4 specification with backwards compatibility for ID3v2.3.
func (t *Tagger) WriteMp3Tags(filePath string, track *api.TrackMetadata, album *api.AlbumMetadata, coverData []byte) error {
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("failed to open mp3 file: %w", err)
	}
	defer tag.Close()

	// Use UTF-8 encoding for proper Unicode support
	tag.SetDefaultEncoding(id3v2.EncodingUTF8)

	// === Basic Tags ===
	tag.SetTitle(track.Title)
	tag.SetArtist(track.Performer.Name)
	tag.SetAlbum(album.Title)

	// Album artist (TPE2)
	if album.Artist.Name != "" {
		tag.AddTextFrame("TPE2", id3v2.EncodingUTF8, album.Artist.Name)
	}

	// === Track/Disc Numbers ===
	// TRCK: Track number/total (e.g., "4/12")
	if track.TrackNumber > 0 {
		trackStr := fmt.Sprintf("%d", track.TrackNumber)
		if album.TracksCount > 0 {
			trackStr = fmt.Sprintf("%d/%d", track.TrackNumber, album.TracksCount)
		}
		tag.AddTextFrame("TRCK", id3v2.EncodingUTF8, trackStr)
	}

	// TPOS: Disc number/total (e.g., "1/2")
	if track.MediaNumber > 0 {
		discStr := fmt.Sprintf("%d", track.MediaNumber)
		if album.MediaCount > 0 {
			discStr = fmt.Sprintf("%d/%d", track.MediaNumber, album.MediaCount)
		}
		tag.AddTextFrame("TPOS", id3v2.EncodingUTF8, discStr)
	}

	// === Genre (TCON) ===
	if album.Genre != nil && album.Genre.Name != "" {
		tag.SetGenre(album.Genre.Name)
	}

	// === Dates ===
	// TDRC: Recording time (ID3v2.4 primary date field)
	// TDOR: Original release time
	// TYER: Year only (ID3v2.3 compatibility)
	if album.ReleaseDateOrg != "" {
		tag.SetYear(album.ReleaseDateOrg) // Sets TDRC
		tag.AddTextFrame("TDOR", id3v2.EncodingUTF8, album.ReleaseDateOrg)
		if len(album.ReleaseDateOrg) >= 4 {
			tag.AddTextFrame("TYER", id3v2.EncodingUTF8, album.ReleaseDateOrg[:4])
		}
	} else if album.ReleaseDateStream != "" {
		tag.SetYear(album.ReleaseDateStream)
	}

	// === Version/Subtitle (TIT3) ===
	if track.Version != "" {
		tag.AddTextFrame("TIT3", id3v2.EncodingUTF8, track.Version)
	}

	// === Composer (TCOM) ===
	// First try track.Composer, then extract from performers string
	if track.Composer != nil && track.Composer.Name != "" {
		tag.AddTextFrame("TCOM", id3v2.EncodingUTF8, track.Composer.Name)
	} else if composers := extractRoles(track.Performers, "Composer"); composers != "" {
		tag.AddTextFrame("TCOM", id3v2.EncodingUTF8, composers)
	} else if album.Composer != nil && album.Composer.Name != "" && album.Composer.Name != "Various Composers" {
		tag.AddTextFrame("TCOM", id3v2.EncodingUTF8, album.Composer.Name)
	}

	// === Lyricist (TEXT) - extract from performers ===
	if lyricists := extractRoles(track.Performers, "Lyricist"); lyricists != "" {
		tag.AddTextFrame("TEXT", id3v2.EncodingUTF8, lyricists)
	}

	// === Conductor (TPE3) ===
	if conductors := extractRoles(track.Performers, "Conductor"); conductors != "" {
		tag.AddTextFrame("TPE3", id3v2.EncodingUTF8, conductors)
	}

	// === Publisher/Label (TPUB) ===
	if album.Label != nil && album.Label.Name != "" {
		tag.AddTextFrame("TPUB", id3v2.EncodingUTF8, album.Label.Name)
	}

	// === Copyright (TCOP) ===
	copyright := track.Copyright
	if copyright == "" {
		copyright = album.Copyright
	}
	if copyright != "" {
		tag.AddTextFrame("TCOP", id3v2.EncodingUTF8, formatCopyright(copyright))
	}

	// === ISRC (TSRC) ===
	if track.ISRC != "" {
		tag.AddTextFrame("TSRC", id3v2.EncodingUTF8, track.ISRC)
	}

	// === Custom frames using TXXX ===

	// Barcode/UPC
	if album.UPC != "" {
		addTXXX(tag, "BARCODE", album.UPC)
	}

	// Full genres list
	if len(album.GenresList) > 0 {
		addTXXX(tag, "GENRES", formatGenresList(album.GenresList))
	}

	// Work (classical music)
	if track.Work != "" {
		addTXXX(tag, "WORK", track.Work)
	}

	// Full performers/credits
	if track.Performers != "" {
		addTXXX(tag, "PERFORMERS", track.Performers)
	}

	// Release type
	if album.ReleaseType != "" {
		addTXXX(tag, "RELEASETYPE", album.ReleaseType)
	}

	// ReplayGain (follows ReplayGain 2.0 specification format)
	if track.AudioInfo != nil {
		if track.AudioInfo.ReplayGainTrackGain != 0 {
			addTXXX(tag, "REPLAYGAIN_TRACK_GAIN", fmt.Sprintf("%.2f dB", track.AudioInfo.ReplayGainTrackGain))
		}
		if track.AudioInfo.ReplayGainTrackPeak != 0 {
			addTXXX(tag, "REPLAYGAIN_TRACK_PEAK", fmt.Sprintf("%.6f", track.AudioInfo.ReplayGainTrackPeak))
		}
	}

	// === Cover Art (APIC - Attached Picture) ===
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

	if err := tag.Save(); err != nil {
		return fmt.Errorf("failed to save mp3 tags: %w", err)
	}
	return nil
}

// addTXXX is a helper function to add a TXXX (user-defined text) frame.
func addTXXX(tag *id3v2.Tag, description, value string) {
	if value == "" {
		return
	}
	tag.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
		Encoding:    id3v2.EncodingUTF8,
		Description: description,
		Value:       value,
	})
}
