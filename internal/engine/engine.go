// Package engine provides the core download and processing functionality.
// It orchestrates API calls, file downloads, and metadata tagging.
package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/imroc/req/v3"

	"qobuz-dl-go/internal/api"
)

// Engine is the core download engine that coordinates API calls,
// file downloads, and metadata tagging operations.
type Engine struct {
	Client      *api.Client
	Tagger      *Tagger
	Concurrency int // Number of concurrent downloads (default: 3)
}

// New creates a new Engine instance with the given API client.
func New(client *api.Client) *Engine {
	return &Engine{
		Client:      client,
		Tagger:      NewTagger(),
		Concurrency: 3, // Default concurrency
	}
}

// SetConcurrency sets the number of concurrent download threads.
func (e *Engine) SetConcurrency(n int) {
	if n < 1 {
		n = 1
	}
	if n > 10 {
		n = 10 // Cap at 10 to avoid API rate limiting
	}
	e.Concurrency = n
}

// ProgressCallback is invoked during download with current bytes and total size.
type ProgressCallback func(current, total int64)

// illegalCharsRegex matches characters that are not allowed in file/folder names.
var illegalCharsRegex = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// sanitizeFilename removes or replaces characters that are illegal in file names.
func sanitizeFilename(name string) string {
	name = illegalCharsRegex.ReplaceAllString(name, "_")
	name = strings.TrimSpace(name)
	// Limit length to avoid path issues (Windows max path component is 255)
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

// trackTask represents a single track download task.
type trackTask struct {
	Track     api.TrackMetadata
	TrackPath string
	FileName  string
	Index     int
}

// TrackStatus represents the download status of a track.
type TrackStatus int

const (
	StatusQueued TrackStatus = iota
	StatusDownloading
	StatusComplete
	StatusFailed
)

// trackState holds the current state of a track for display.
type trackState struct {
	FileName string
	Status   TrackStatus
	Progress int // 0-100
}

// displayConfig holds display configuration for cross-platform compatibility.
type displayConfig struct {
	Width        int  // Display width
	UseANSI      bool // Whether ANSI escape codes are supported
	MaxSongLines int  // Maximum song lines to display (0 = all)
}

// getDisplayConfig returns display configuration based on platform.
func getDisplayConfig() displayConfig {
	cfg := displayConfig{
		Width:        70,
		UseANSI:      true,
		MaxSongLines: 0,
	}

	// Windows cmd.exe before Windows 10 may not support ANSI
	// Most modern terminals support ANSI, so we default to true
	// Users can set NO_COLOR or TERM=dumb to disable
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		cfg.UseANSI = false
	}

	return cfg
}

// runeWidth returns the display width of a rune (CJK = 2, others = 1).
func runeWidth(r rune) int {
	// CJK characters and fullwidth forms take 2 columns
	if r >= 0x1100 && r <= 0x115F || // Hangul Jamo
		r >= 0x2E80 && r <= 0x9FFF || // CJK ranges (including radicals, ideographs)
		r >= 0xA960 && r <= 0xA97F || // Hangul Jamo Extended-A
		r >= 0xAC00 && r <= 0xD7FF || // Hangul Syllables + Jamo Extended-B
		r >= 0xF900 && r <= 0xFAFF || // CJK Compatibility Ideographs
		r >= 0xFE10 && r <= 0xFE1F || // Vertical Forms
		r >= 0xFE30 && r <= 0xFE6F || // CJK Compatibility Forms
		r >= 0xFF00 && r <= 0xFF60 || // Fullwidth ASCII
		r >= 0xFFE0 && r <= 0xFFE6 || // Fullwidth symbols
		r >= 0x20000 && r <= 0x2FFFF || // CJK Extension B-F
		r >= 0x30000 && r <= 0x3FFFF { // CJK Extension G+
		return 2
	}
	// Control characters and zero-width
	if r < 0x20 || r == 0x7F || // Control chars
		r >= 0x200B && r <= 0x200F || // Zero-width chars
		r >= 0x2028 && r <= 0x202E || // Line/paragraph separators
		r >= 0xFE00 && r <= 0xFE0F || // Variation Selectors
		r == 0xFEFF { // BOM
		return 0
	}
	return 1
}

// stringDisplayWidth calculates the display width of a string.
func stringDisplayWidth(s string) int {
	width := 0
	for _, r := range s {
		width += runeWidth(r)
	}
	return width
}

// padRight pads a string to a fixed display width using spaces.
// Handles CJK and other wide characters correctly.
func padRight(s string, targetWidth int) string {
	currentWidth := stringDisplayWidth(s)
	if currentWidth >= targetWidth {
		return truncateToWidth(s, targetWidth)
	}
	// Fill remaining space with spaces
	return s + strings.Repeat(" ", targetWidth-currentWidth)
}

// padLeft pads a string to a fixed display width with leading spaces.
func padLeft(s string, targetWidth int) string {
	currentWidth := stringDisplayWidth(s)
	if currentWidth >= targetWidth {
		return truncateToWidth(s, targetWidth)
	}
	return strings.Repeat(" ", targetWidth-currentWidth) + s
}

// truncateToWidth truncates a string to fit within a display width.
// Adds "..." suffix if truncation occurs.
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}

	currentWidth := stringDisplayWidth(s)
	if currentWidth <= maxWidth {
		return s
	}

	// Need to truncate
	targetWidth := maxWidth - 3 // Reserve space for "..."
	width := 0
	var result []rune

	for _, r := range s {
		w := runeWidth(r)
		if width+w > targetWidth {
			break
		}
		result = append(result, r)
		width += w
	}

	return string(result) + "..."
}

// printBox prints a nicely formatted box with proper alignment.
func printBox(lines []string, width int) {
	border := strings.Repeat("═", width-2)
	fmt.Printf("╔%s╗\n", border)
	for _, line := range lines {
		fmt.Printf("║ %s ║\n", padRight(line, width-4))
	}
	fmt.Printf("╚%s╝\n", border)
}

// makeProgressBar creates a text progress bar.
func makeProgressBar(percent int, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := width * percent / 100
	empty := width - filled
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", empty) + "]"
}

// displayState manages the terminal display state.
type displayState struct {
	buffer    bytes.Buffer
	mu        sync.Mutex
	config    displayConfig
	lastLines int // Number of lines in last render
}

// newDisplayState creates a new display state.
func newDisplayState() *displayState {
	return &displayState{
		config:    getDisplayConfig(),
		lastLines: 0,
	}
}

// clearAndRender clears previous output and renders new content.
func (d *displayState) clearAndRender(content string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.buffer.Reset()

	// Move cursor up and clear previous lines if ANSI supported
	if d.config.UseANSI && d.lastLines > 0 {
		// Move up N lines and clear each
		for i := 0; i < d.lastLines; i++ {
			d.buffer.WriteString("\033[A")  // Move up
			d.buffer.WriteString("\033[2K") // Clear line
		}
	}

	// Write new content
	d.buffer.WriteString(content)

	// Count new lines for next clear
	d.lastLines = strings.Count(content, "\n")

	// Output buffer atomically
	fmt.Print(d.buffer.String())
}

// renderFinal renders final output without cursor manipulation.
func (d *displayState) renderFinal(content string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Clear previous display first
	if d.config.UseANSI && d.lastLines > 0 {
		var clearBuf bytes.Buffer
		for i := 0; i < d.lastLines; i++ {
			clearBuf.WriteString("\033[A")
			clearBuf.WriteString("\033[2K")
		}
		fmt.Print(clearBuf.String())
	}

	fmt.Print(content)
	d.lastLines = 0
}

// buildThreadLine builds a single thread status line with fixed width.
func buildThreadLine(workerID int, songName string, progress int, isWorking bool, width int) string {
	// Layout: "  Thread N: " (fixed 12) + songName (variable) + " " + bar (12) + " " + percent (4)
	// Example: "  Thread 1: Song Name Here      [####----] 100%"

	// Fixed widths
	const prefixFmt = "  Thread %d: "
	prefix := fmt.Sprintf(prefixFmt, workerID+1)
	prefixWidth := len(prefix) // ASCII only

	if !isWorking {
		// Idle state: fill rest with spaces for consistent width
		idleText := "Idle"
		remaining := width - prefixWidth - len(idleText)
		if remaining < 0 {
			remaining = 0
		}
		return prefix + idleText + strings.Repeat(" ", remaining)
	}

	// Working state layout
	const barWidth = 12    // [##########]
	const percentWidth = 5 // " 100%"
	const spacing = 2      // spaces between elements
	songWidth := width - prefixWidth - barWidth - percentWidth - spacing

	if songWidth < 8 {
		songWidth = 8
	}

	// Build components with exact widths
	songPadded := padRight(songName, songWidth)
	bar := makeProgressBar(progress, barWidth)
	percentStr := fmt.Sprintf("%4d%%", progress) // Right-aligned percentage

	return prefix + songPadded + " " + bar + percentStr
}

// buildSongLine builds a single song status line with fixed width.
func buildSongLine(songName string, status TrackStatus, progress int, width int) string {
	// Layout: "  " + songName (variable) + "  " + status (fixed 10)
	// Example: "  01. Song Name Here              v Complete"

	const statusWidth = 10 // "v Complete" or "x Failed  " etc.
	const margins = 4      // "  " prefix + "  " separator
	songWidth := width - margins - statusWidth

	if songWidth < 10 {
		songWidth = 10
	}

	songPadded := padRight(songName, songWidth)

	var statusStr string
	switch status {
	case StatusQueued:
		statusStr = "o Queued  "
	case StatusDownloading:
		statusStr = fmt.Sprintf("> %3d%%    ", progress)
	case StatusComplete:
		statusStr = "v Complete"
	case StatusFailed:
		statusStr = "x Failed  "
	default:
		statusStr = "  Unknown "
	}

	return "  " + songPadded + "  " + statusStr
}

// buildDisplayContent builds the entire display content as a string.
func buildDisplayContent(
	numWorkers int,
	threadTasks []int,
	threadProgress []int,
	tasks []trackTask,
	trackStates []trackState,
	width int,
) string {
	var buf bytes.Buffer

	separator := strings.Repeat("-", width)

	// Thread Status Section
	buf.WriteString(separator + "\n")
	buf.WriteString("  THREAD STATUS\n")
	buf.WriteString(separator + "\n")

	for i := range numWorkers {
		taskIdx := threadTasks[i]
		isWorking := taskIdx >= 0 && taskIdx < len(tasks)

		var songName string
		var progress int
		if isWorking {
			songName = tasks[taskIdx].FileName
			progress = threadProgress[i]
		}

		line := buildThreadLine(i, songName, progress, isWorking, width)
		buf.WriteString(line + "\n")
	}

	// Song Status Section
	buf.WriteString(separator + "\n")
	buf.WriteString("  SONG STATUS\n")
	buf.WriteString(separator + "\n")

	for _, ts := range trackStates {
		line := buildSongLine(ts.FileName, ts.Status, ts.Progress, width)
		buf.WriteString(line + "\n")
	}

	buf.WriteString(separator + "\n")

	return buf.String()
}

// DownloadAlbum downloads an entire album with concurrent workers and progress display.
func (e *Engine) DownloadAlbum(ctx context.Context, albumID string, quality int, outputDir string) error {
	// 1. Get Album Metadata
	album, err := e.Client.GetAlbum(albumID)
	if err != nil {
		return fmt.Errorf("failed to get album metadata: %w", err)
	}

	totalTracks := len(album.Tracks.Items)

	// Print header with proper alignment
	fmt.Println()
	boxWidth := 74
	headerLines := []string{
		fmt.Sprintf("Album:  %s", truncateToWidth(album.Title, boxWidth-14)),
		fmt.Sprintf("Artist: %s", truncateToWidth(album.Artist.Name, boxWidth-14)),
		fmt.Sprintf("Tracks: %d", totalTracks),
		fmt.Sprintf("Threads: %d", e.Concurrency),
	}
	printBox(headerLines, boxWidth)
	fmt.Println()

	// 2. Prepare Album Directory
	folderName := sanitizeFilename(fmt.Sprintf("%s - %s", album.Artist.Name, album.Title))
	albumDir := filepath.Join(outputDir, folderName)
	if err := os.MkdirAll(albumDir, 0755); err != nil {
		return err
	}

	// 3. Download Cover Art first
	var coverData []byte
	if album.Image.Large != "" {
		fmt.Print("[Cover] Downloading... ")
		coverData, err = e.downloadCover(album.Image.Large)
		if err == nil {
			_ = e.saveCoverFile(albumDir, coverData)
			fmt.Println("Done")
		} else {
			fmt.Println("Failed (continuing without cover)")
		}
	}
	fmt.Println()

	// 4. Build task queue
	var tasks []trackTask
	skipped := 0
	for i, track := range album.Tracks.Items {
		trackFileName := sanitizeFilename(fmt.Sprintf("%02d. %s", track.TrackNumber, track.Title)) + ".flac"
		trackPath := filepath.Join(albumDir, trackFileName)

		// Check if already exists
		if _, err := os.Stat(trackPath); err == nil {
			skipped++
			continue
		}

		tasks = append(tasks, trackTask{
			Track:     track,
			TrackPath: trackPath,
			FileName:  trackFileName,
			Index:     i + 1,
		})
	}

	if skipped > 0 {
		fmt.Printf("[Skip] %d tracks already exist\n\n", skipped)
	}

	if len(tasks) == 0 {
		fmt.Println("[Done] All tracks already downloaded!")
		return nil
	}

	// 5. Initialize track states for display
	trackStates := make([]trackState, len(tasks))
	for i, task := range tasks {
		trackStates[i] = trackState{
			FileName: task.FileName,
			Status:   StatusQueued,
			Progress: 0,
		}
	}

	// Thread states: which song each thread is working on (-1 = rest)
	threadTasks := make([]int, e.Concurrency) // index into tasks array, -1 = rest
	threadProgress := make([]int, e.Concurrency)
	for i := range threadTasks {
		threadTasks[i] = -1
	}

	var stateMu sync.Mutex
	numWorkers := e.Concurrency
	if numWorkers > len(tasks) {
		numWorkers = len(tasks)
	}

	// Initialize display state
	display := newDisplayState()
	displayWidth := display.config.Width

	// 6. Start display goroutine
	stopDisplay := make(chan struct{})
	displayDone := make(chan struct{})

	go func() {
		defer close(displayDone)
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stopDisplay:
				return
			case <-ticker.C:
				stateMu.Lock()
				content := buildDisplayContent(numWorkers, threadTasks, threadProgress, tasks, trackStates, displayWidth)
				stateMu.Unlock()
				display.clearAndRender(content)
			}
		}
	}()

	// 7. Create worker pool
	taskChan := make(chan int, len(tasks)) // send task index
	var wg sync.WaitGroup

	for w := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for taskIdx := range taskChan {
				task := tasks[taskIdx]

				// Update state: downloading
				stateMu.Lock()
				threadTasks[workerID] = taskIdx
				threadProgress[workerID] = 0
				trackStates[taskIdx].Status = StatusDownloading
				trackStates[taskIdx].Progress = 0
				stateMu.Unlock()

				// Get track URL
				urlInfo, err := e.Client.GetTrackURL(strconv.Itoa(task.Track.ID), quality)
				if err != nil {
					stateMu.Lock()
					trackStates[taskIdx].Status = StatusFailed
					threadTasks[workerID] = -1
					stateMu.Unlock()
					continue
				}

				// Download with progress callback
				err = e.downloadFileWithProgress(ctx, urlInfo.URL, task.TrackPath, func(percent int) {
					stateMu.Lock()
					threadProgress[workerID] = percent
					trackStates[taskIdx].Progress = percent
					stateMu.Unlock()
				})

				if err != nil {
					stateMu.Lock()
					trackStates[taskIdx].Status = StatusFailed
					threadTasks[workerID] = -1
					stateMu.Unlock()
					continue
				}

				// Tag the file
				track := task.Track
				_ = e.Tagger.WriteTags(task.TrackPath, &track, album, coverData)

				// Update state: complete
				stateMu.Lock()
				trackStates[taskIdx].Status = StatusComplete
				trackStates[taskIdx].Progress = 100
				threadTasks[workerID] = -1
				stateMu.Unlock()
			}
		}(w)
	}

	// Send tasks by index
	for i := range tasks {
		taskChan <- i
	}
	close(taskChan)

	// Wait for completion
	wg.Wait()
	close(stopDisplay)
	<-displayDone

	// Render final status
	stateMu.Lock()
	finalContent := buildDisplayContent(numWorkers, threadTasks, threadProgress, tasks, trackStates, displayWidth)
	stateMu.Unlock()
	display.renderFinal(finalContent)

	// Print summary
	fmt.Println()
	successCount := 0
	failCount := 0
	for _, ts := range trackStates {
		if ts.Status == StatusComplete {
			successCount++
		} else if ts.Status == StatusFailed {
			failCount++
		}
	}

	summaryLines := []string{
		"Download Complete!",
		fmt.Sprintf("Success: %d  |  Failed: %d  |  Skipped: %d", successCount, failCount, skipped),
	}
	printBox(summaryLines, boxWidth)

	return nil
}

// downloadFileWithProgress downloads a file and reports progress as percentage.
func (e *Engine) downloadFileWithProgress(ctx context.Context, url, outputPath string, onProgress func(int)) error {
	var contentLength int64 = 0

	resp, err := e.Client.HTTP.R().
		SetContext(ctx).
		SetOutputFile(outputPath).
		SetDownloadCallback(func(info req.DownloadInfo) {
			if info.Response.ContentLength > 0 {
				contentLength = info.Response.ContentLength
				percent := int(float64(info.DownloadedSize) / float64(contentLength) * 100)
				if percent > 100 {
					percent = 100
				}
				if onProgress != nil {
					onProgress(percent)
				}
			}
		}).
		Get(url)

	if err != nil {
		return err
	}
	if resp.IsErrorState() {
		return fmt.Errorf("http error: %s", resp.Status)
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
	if resp.IsErrorState() {
		return fmt.Errorf("http error: %s", resp.Status)
	}
	return nil
}

func (e *Engine) downloadCover(url string) ([]byte, error) {
	// Try maximum quality (original)
	maxUrl := strings.Replace(url, "_600.", "_org.", 1)

	// Try downloading max quality
	resp, err := e.Client.HTTP.R().Get(maxUrl)
	if err == nil && !resp.IsErrorState() {
		return resp.Bytes(), nil
	}

	// Fallback to provided URL if max fails
	resp, err = e.Client.HTTP.R().Get(url)
	if err != nil {
		return nil, err
	}
	if resp.IsErrorState() {
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
	// 1. Fetch Track Metadata first
	track, err := e.Client.GetTrack(trackID)
	if err != nil {
		return fmt.Errorf("failed to get track metadata: %w", err)
	}

	// 2. Fetch Track URL
	info, err := e.Client.GetTrackURL(trackID, quality)
	if err != nil {
		return fmt.Errorf("failed to get track URL: %w", err)
	}

	// 3. Prepare Directory & Filename
	// For single tracks, use format: "Artist - Title.flac"
	fileName := sanitizeFilename(fmt.Sprintf("%s - %s", track.Performer.Name, track.Title)) + ".flac"
	outputPath := filepath.Join(outputDir, fileName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// 4. Download Audio
	err = e.downloadFile(ctx, info.URL, outputPath, onProgress)
	if err != nil {
		return err
	}

	// 5. Download Cover Art (if available)
	var coverData []byte
	if track.Album != nil && track.Album.Image.Large != "" {
		coverData, _ = e.downloadCover(track.Album.Image.Large)
	}

	// 6. Tagging
	// Note: TrackMetadata has 'Album' embedded usually if fetched via GetTrack
	// But our model definition in models.go might need checking if GetTrack response structure embeds full album.
	// API response usually embeds partial album info.
	if track.Album == nil {
		// Fallback create dummy album
		track.Album = &api.AlbumMetadata{Title: "Unknown Album"}
	}

	err = e.Tagger.WriteTags(outputPath, track, track.Album, coverData)
	if err != nil {
		// Just warn, don't fail download
		fmt.Printf("Warning: Failed to tag file: %v\n", err)
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

	if resp.IsErrorState() {
		return fmt.Errorf("stream returned error: %s", resp.Status)
	}

	return nil
}
