// Package updater provides self-update functionality for qobuz-dl-go.
// It checks for new releases on GitHub and handles downloading and applying updates
// using the minio/selfupdate library for atomic, cross-platform binary replacement.
package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"github.com/minio/selfupdate"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/version"
)

const (
	// GitHubRepo is the repository path for releases
	GitHubRepo = "WenqiOfficial/qobuz-dl-go"
	// ReleaseAPICDN is the CDN proxy endpoint for GitHub API
	ReleaseAPICDN = "https://api.hubproxy.wenqi.icu/repos/" + GitHubRepo + "/releases/latest"
	// ReleaseAPIDirect is the direct GitHub API endpoint
	ReleaseAPIDirect = "https://api.github.com/repos/" + GitHubRepo + "/releases/latest"
)

// httpClient is the package-level HTTP client (can be configured with proxy)
var httpClient = &http.Client{}

// ReleaseInfo contains information about a GitHub release
type ReleaseInfo struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Assets  []Asset `json:"assets"`
	HTMLURL string  `json:"html_url"`
}

// Asset represents a release asset (binary download)
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// UpdateResult contains the result of an update check
type UpdateResult struct {
	CurrentVersion string
	LatestVersion  string
	HasUpdate      bool
	ReleaseInfo    *ReleaseInfo
}

// SetProxy configures the HTTP client to use the specified proxy URL.
// Supports http, https, and socks5 schemes.
func SetProxy(proxyURL string) error {
	if proxyURL == "" {
		return nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}
	httpClient.Transport = &http.Transport{
		Proxy: http.ProxyURL(parsed),
	}
	return nil
}

// CheckForUpdate checks GitHub for the latest release and compares versions.
// If useCDN is true, tries CDN first then falls back to direct API.
func CheckForUpdate(useCDN bool) (*UpdateResult, error) {
	currentVersion := version.Version

	var release ReleaseInfo
	var err error

	if useCDN {
		// Try CDN first
		release, err = fetchReleaseInfo(ReleaseAPICDN)
		if err != nil {
			// Fallback to direct API
			release, err = fetchReleaseInfo(ReleaseAPIDirect)
		}
	} else {
		// Direct API only
		release, err = fetchReleaseInfo(ReleaseAPIDirect)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}

	// Extract version number (remove 'v' prefix if present)
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	hasUpdate := compareVersions(currentVersion, latestVersion) < 0

	return &UpdateResult{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		HasUpdate:      hasUpdate,
		ReleaseInfo:    &release,
	}, nil
}

// fetchReleaseInfo fetches release info from the given API URL
func fetchReleaseInfo(apiURL string) (ReleaseInfo, error) {
	var release ReleaseInfo

	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return release, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return release, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return release, fmt.Errorf("failed to parse release info: %w", err)
	}

	return release, nil
}

// GetPlatformAsset returns the appropriate asset for the current platform
func (r *ReleaseInfo) GetPlatformAsset() (*Asset, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Determine expected file extension
	var ext string
	if goos == "windows" {
		ext = ".zip"
	} else {
		ext = ".tar.gz"
	}

	// Build expected asset name pattern
	pattern := fmt.Sprintf("qobuz-dl-go-%s-%s-%s%s", r.TagName, goos, goarch, ext)

	for _, asset := range r.Assets {
		if asset.Name == pattern {
			return &asset, nil
		}
	}

	return nil, fmt.Errorf("no release found for %s/%s", goos, goarch)
}

// DownloadAndApply downloads the release and applies it atomically using selfupdate
func DownloadAndApply(asset *Asset, tagName string, progressFn func(current, total int64)) error {
	// Download the archive (uses httpClient which respects proxy settings)
	resp, err := httpClient.Get(asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Read entire archive into memory (releases are small, ~6MB)
	var buf bytes.Buffer
	if progressFn != nil {
		var written int64
		tmpBuf := make([]byte, 32*1024)
		for {
			n, readErr := resp.Body.Read(tmpBuf)
			if n > 0 {
				buf.Write(tmpBuf[:n])
				written += int64(n)
				progressFn(written, asset.Size)
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return readErr
			}
		}
	} else {
		if _, err := io.Copy(&buf, resp.Body); err != nil {
			return fmt.Errorf("failed to read archive: %w", err)
		}
	}

	// Extract binary from archive
	var binaryReader io.Reader
	if strings.HasSuffix(asset.Name, ".zip") {
		binaryReader, err = extractFromZip(buf.Bytes(), tagName)
	} else {
		binaryReader, err = extractFromTarGz(buf.Bytes(), tagName)
	}
	if err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	// Apply update atomically using selfupdate
	if err := selfupdate.Apply(binaryReader, selfupdate.Options{}); err != nil {
		// Attempt rollback on failure
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("update failed and rollback also failed: %w", rerr)
		}
		return fmt.Errorf("update failed (rolled back): %w", err)
	}

	return nil
}

// extractFromZip extracts the binary from a zip archive
func extractFromZip(data []byte, tagName string) (io.Reader, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	// Look for the executable in the expected path
	// Archive structure: qobuz-dl-go-v{version}-{os}-{arch}/qobuz-dl-go.exe
	expectedName := "qobuz-dl-go.exe"

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// Check if this is the binary we're looking for
		if strings.HasSuffix(f.Name, expectedName) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()

			var buf bytes.Buffer
			if _, err := io.Copy(&buf, rc); err != nil {
				return nil, err
			}
			return bytes.NewReader(buf.Bytes()), nil
		}
	}

	return nil, fmt.Errorf("binary not found in archive")
}

// extractFromTarGz extracts the binary from a tar.gz archive
func extractFromTarGz(data []byte, tagName string) (io.Reader, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	// Look for the executable in the expected path
	// Archive structure: qobuz-dl-go-v{version}-{os}-{arch}/qobuz-dl-go
	expectedName := "qobuz-dl-go"

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Check if this is the binary we're looking for
		if strings.HasSuffix(header.Name, "/"+expectedName) {
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, tr); err != nil {
				return nil, err
			}
			return bytes.NewReader(buf.Bytes()), nil
		}
	}

	return nil, fmt.Errorf("binary not found in archive")
}

// compareVersions compares two semantic version strings
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {
	// Strip 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Handle dev versions
	if strings.HasPrefix(v1, "dev") {
		return -1 // dev is always older than any release
	}
	if strings.HasPrefix(v2, "dev") {
		return 1 // any release is newer than dev
	}

	// Parse version parts
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Compare each part
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}

		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	return 0
}
