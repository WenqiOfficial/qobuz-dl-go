package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/imroc/req/v3"

	"qobuz-dl-go/internal/api"
)

type Engine struct {
	Client *api.Client
}

func New(client *api.Client) *Engine {
	return &Engine{Client: client}
}

// ProgressCallback is called with the number of bytes read and the total size
type ProgressCallback func(current, total int64)

// DownloadTrack downloads a track by ID to a local file.
func (e *Engine) DownloadTrack(ctx context.Context, trackID string, quality int, outputDir string, onProgress ProgressCallback) error {
	// 1. Get Track URL
	info, err := e.Client.GetTrackURL(trackID, quality)
	if err != nil {
		return fmt.Errorf("failed to get track URL: %w", err)
	}

	// 2. Prepare Output File
	fileName := fmt.Sprintf("%s.flac", trackID) // Placeholder name
	outputPath := filepath.Join(outputDir, fileName)
	
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// 3. Start Download
	resp, err := e.Client.HTTP.R().
		SetContext(ctx).
		SetOutputFile(outputPath).
		SetDownloadCallback(func(info req.DownloadInfo) {
			if onProgress != nil {
				onProgress(info.DownloadedSize, info.Response.ContentLength)
			}
		}).
		Get(info.URL)

	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}

	if resp.IsError() {
		return fmt.Errorf("download returned error: %s", resp.Status)
	}

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
