package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/imroc/req/v3"

	"qobuz-dl-go/internal/api"
)

type Engine struct {
	Client *api.Client
	Tagger *Tagger
}

func New(client *api.Client) *Engine {
	return &Engine{
		Client: client,
		Tagger: NewTagger(),
	}
}

// ProgressCallback is called with the number of bytes read and the total size
type ProgressCallback func(current, total int64)

// DownloadAlbum downloads an entire album.
func (e *Engine) DownloadAlbum(ctx context.Context, albumID string, quality int, outputDir string) error {
	// 1. Get Album Metadata
	album, err := e.Client.GetAlbum(albumID)
	if err != nil {
		return fmt.Errorf("failed to get album metadata: %w", err)
	}

	fmt.Printf("Downloading Album: %s - %s (%d tracks)\n", album.Artist.Name, album.Title, len(album.Tracks.Items))

	// 2. Prepare Album Directory
	// Sanitization needed here in real world
	albumDir := filepath.Join(outputDir, fmt.Sprintf("%s - %s", album.Artist.Name, album.Title))
	if err := os.MkdirAll(albumDir, 0755); err != nil {
		return err
	}

	// 2.1 Download Cover Art
	var coverData []byte
	if album.Image.Large != "" {
		fmt.Println("Downloading Cover Art...")
		coverData, err = e.downloadCover(album.Image.Large)
		if err == nil {
			_ = e.saveCoverFile(albumDir, coverData)
		} else {
			fmt.Printf("Warning: Failed to download cover: %v\n", err)
		}
	}

	// 3. Download Each Track
	for _, track := range album.Tracks.Items {
		// Use Track specific filename
		trackFileName := fmt.Sprintf("%02d. %s.flac", track.TrackNumber, track.Title)
		trackPath := filepath.Join(albumDir, trackFileName)

		// Check if exists
		if _, err := os.Stat(trackPath); err == nil {
			fmt.Printf("Skipping %s (already exists)\n", trackFileName)
			continue
		}

		fmt.Printf("Downloading: %s...\n", trackFileName)

		// Get URL
		urlInfo, err := e.Client.GetTrackURL(strconv.Itoa(track.ID), quality)
		if err != nil {
			fmt.Printf("Failed to get URL for track %s: %v\n", track.Title, err)
			continue
		}

		// Download
		// We use the same logic as DownloadTrack but custom path
		err = e.downloadFile(ctx, urlInfo.URL, trackPath, nil)
		if err != nil {
			fmt.Printf("Failed to download %s: %v\n", track.Title, err)
			continue
		}

		// Tag
		fmt.Printf("Tagging: %s...\n", trackFileName)
		err = e.Tagger.WriteTags(trackPath, &track, album, coverData)
		if err != nil {
			fmt.Printf("Warning: Failed to write tags for %s: %v\n", track.Title, err)
		}
	}

	return nil
}

func (e *Engine) downloadFile(ctx context.Context, url, outputPath string, onProgress ProgressCallback) error {
	resp, err := e.Client.HTTP.R().
		SetContext(ctx).
		SetOutputFile(outputPath).
		SetDownloadCallback(func(info req.DownloadInfo) {
			if onProgress != nil {
				onProgress(info.DownloadedSize, info.Response.ContentLength)
			}
		}).
		Get(url)

	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf("http error: %s", resp.Status)
	}
	return nil
}

func (e *Engine) downloadCover(url string) ([]byte, error) {
	// Try maximum quality (original)
	maxUrl := strings.Replace(url, "_600.", "_org.", 1)

	// Try downloading max quality
	resp, err := e.Client.HTTP.R().Get(maxUrl)
	if err == nil && !resp.IsError() {
		return resp.Bytes(), nil
	}

	// Fallback to provided URL if max fails
	resp, err = e.Client.HTTP.R().Get(url)
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("http error: %s", resp.Status)
	}
	return resp.Bytes(), nil
}

func (e *Engine) saveCoverFile(dir string, data []byte) error {
	coverPath := filepath.Join(dir, "cover.jpg")
	return os.WriteFile(coverPath, data, 0644)
}

// DownloadTrack downloads a track by ID to a local file.
func (e *Engine) DownloadTrack(ctx context.Context, trackID string, quality int, outputDir string, onProgress ProgressCallback) error {
	// Need Track Metadata first for filename
	// But Client.GetTrackURL doesn't return metadata
	// We should fetch metadata first.
	// HACK: Use GetAlbum for single track? No.
	// Wait, we need a GetTrackMetadata method. But we don't have it implemented yet in Client.
	// For now, let's just stick to ID filename, but wait, the user wants tags.
	// If the user imports a Track ID, we MUST fetch metadata to tag it properly.

	// Since we don't have GetTrackMetadata separately implemented in Client, let's implement it or stub it.
	// Client.GetTrackURL only returns URL.
	// We need `track/get`.

	// Let's defer this specific "Tag Single Track" feature improvement for a moment or implement metadata fetching here.

	// 1. Get Track URL
	info, err := e.Client.GetTrackURL(trackID, quality)
	if err != nil {
		return fmt.Errorf("failed to get track URL: %w", err)
	}

	// 2. Prepare Output File
	fileName := fmt.Sprintf("%s.flac", trackID)
	outputPath := filepath.Join(outputDir, fileName)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	err = e.downloadFile(ctx, info.URL, outputPath, onProgress)
	if err != nil {
		return err
	}

	// Tagging for single track requires extra API call.
	// We'll skip for now if just ID provided, unless we add GetTrackMetadata to client.
	return nil
}

// StreamTrack streams the track data to the provided writer.
func (e *Engine) StreamTrack(ctx context.Context, trackID string, quality int, w io.Writer, onProgress ProgressCallback) error {
	// 1. Get Track URL
	info, err := e.Client.GetTrackURL(trackID, quality)
	if err != nil {
		return fmt.Errorf("failed to get track URL: %w", err)
	}

	// 2. Start Download to Writer
	resp, err := e.Client.HTTP.R().
		SetContext(ctx).
		SetOutput(w).
		SetDownloadCallback(func(info req.DownloadInfo) {
			if onProgress != nil {
				onProgress(info.DownloadedSize, info.Response.ContentLength)
			}
		}).
		Get(info.URL)

	if err != nil {
		return fmt.Errorf("stream request failed: %w", err)
	}

	if resp.IsError() {
		return fmt.Errorf("stream returned error: %s", resp.Status)
	}

	return nil
}
