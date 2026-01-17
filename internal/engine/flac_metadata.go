// flac_metadata.go provides low-level FLAC metadata block parsing and serialization.
// It implements Vorbis Comment and Picture block structures per FLAC specification.
package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// VorbisComment represents a FLAC Vorbis Comment metadata block.
// It contains vendor info and key=value comment pairs.
type VorbisComment struct {
	Vendor   string   // Vendor/encoder identification string
	Comments []string // Comments in "KEY=VALUE" format
}

// NewVorbisComment creates a new VorbisComment with a default vendor string.
func NewVorbisComment() *VorbisComment {
	return &VorbisComment{
		Vendor:   "qobuz-dl-go 1.0.0",
		Comments: []string{},
	}
}

// ParseVorbisComment parses a Vorbis Comment block from raw bytes.
func ParseVorbisComment(data []byte) (*VorbisComment, error) {
	buf := bytes.NewReader(data)

	// Vendor Length (Little Endian)
	var vendorLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &vendorLen); err != nil {
		return nil, fmt.Errorf("failed to read vendor length: %w", err)
	}

	// Vendor String
	vendorBytes := make([]byte, vendorLen)
	if _, err := io.ReadFull(buf, vendorBytes); err != nil {
		return nil, fmt.Errorf("failed to read vendor string: %w", err)
	}
	vendor := string(vendorBytes)

	// Comment List Length (Little Endian)
	var commentListLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &commentListLen); err != nil {
		return nil, fmt.Errorf("failed to read comment list length: %w", err)
	}

	comments := make([]string, commentListLen)
	for i := 0; i < int(commentListLen); i++ {
		var commentLen uint32
		if err := binary.Read(buf, binary.LittleEndian, &commentLen); err != nil {
			return nil, fmt.Errorf("failed to read comment length at index %d: %w", i, err)
		}

		commentBytes := make([]byte, commentLen)
		if _, err := io.ReadFull(buf, commentBytes); err != nil {
			return nil, fmt.Errorf("failed to read comment string at index %d: %w", i, err)
		}
		comments[i] = string(commentBytes)
	}

	return &VorbisComment{
		Vendor:   vendor,
		Comments: comments,
	}, nil
}

// Marshal serializes the VorbisComment to bytes
func (vc *VorbisComment) Marshal() []byte {
	// Calculate size first is not strictly needed for bytes.Buffer but efficient allocation could be done.
	buf := new(bytes.Buffer)

	// Vendor
	binary.Write(buf, binary.LittleEndian, uint32(len(vc.Vendor)))
	buf.WriteString(vc.Vendor)

	// List Length
	binary.Write(buf, binary.LittleEndian, uint32(len(vc.Comments)))

	// Comments
	for _, c := range vc.Comments {
		binary.Write(buf, binary.LittleEndian, uint32(len(c)))
		buf.WriteString(c)
	}

	return buf.Bytes()
}

// Add appends a new tag
func (vc *VorbisComment) Add(key, value string) {
	if value == "" {
		return
	}
	vc.Comments = append(vc.Comments, fmt.Sprintf("%s=%s", key, value))
}

// Picture Block
type Picture struct {
	MIME        string
	Description string
	ImageData   []byte
	PictureType uint32
	Width       uint32
	Height      uint32
	Depth       uint32
	ColorCount  uint32
}

const (
	PictureTypeOther              = 0
	PictureType32x32PixelsIcon    = 1
	PictureTypeOtherIcon          = 2
	PictureTypeCoverFront         = 3
	PictureTypeCoverBack          = 4
	PictureTypeLeaflet            = 5
	PictureTypeMedia              = 6
	PictureTypeLeadArtist         = 7
	PictureTypeArtist             = 8
	PictureTypeConductor          = 9
	PictureTypeBand               = 10
	PictureTypeComposer           = 11
	PictureTypeLyricist           = 12
	PictureTypeRecordingLocation  = 13
	PictureTypeDuringRecording    = 14
	PictureTypeDuringPerformance  = 15
	PictureTypeVideoScreenCapture = 16
	PictureTypeFish               = 17
	PictureTypeIllustration       = 18
	PictureTypeBandLogotype       = 19
	PictureTypePublisherLogotype  = 20
)

func NewPicture() *Picture {
	return &Picture{
		PictureType: PictureTypeCoverFront,
		MIME:        "image/jpeg",
	}
}

func (p *Picture) Marshal() []byte {
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.BigEndian, p.PictureType)

	binary.Write(buf, binary.BigEndian, uint32(len(p.MIME)))
	buf.WriteString(p.MIME)

	binary.Write(buf, binary.BigEndian, uint32(len(p.Description)))
	buf.WriteString(p.Description)

	binary.Write(buf, binary.BigEndian, p.Width)
	binary.Write(buf, binary.BigEndian, p.Height)
	binary.Write(buf, binary.BigEndian, p.Depth)
	binary.Write(buf, binary.BigEndian, p.ColorCount)

	binary.Write(buf, binary.BigEndian, uint32(len(p.ImageData)))
	buf.Write(p.ImageData)

	return buf.Bytes()
}
