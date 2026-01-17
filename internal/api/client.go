// Package api provides the Qobuz API client and related utilities.
// It handles authentication, request signing, and all API interactions.
package api

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/imroc/req/v3"
)

// API constants for Qobuz service.
const (
	BaseURL   = "https://www.qobuz.com/api.json/0.2/"
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:83.0) Gecko/20100101 Firefox/83.0"
)

// Client is the Qobuz API client that handles all API requests.
// It manages authentication state and request signing.
type Client struct {
	HTTP      *req.Client // HTTP client with configured defaults
	AppID     string      // Application ID obtained from Qobuz web player
	AppSecret string      // Application secret for request signing
	UserToken string      // User authentication token
}

// NewClient creates a new Qobuz API client with the given credentials.
// The client is configured with default headers and base URL.
func NewClient(appID, appSecret string) *Client {
	c := &Client{
		AppID:     appID,
		AppSecret: appSecret,
		HTTP:      req.NewClient(),
	}

	c.HTTP.SetBaseURL(BaseURL).
		SetUserAgent(UserAgent).
		SetCommonHeader("X-App-Id", appID).
		SetCommonHeader("Content-Type", "application/json;charset=UTF-8")

	return c
}

// SetProxy configures the HTTP client to use the specified proxy URL.
// Supports http, https, and socks5 schemes.
func (c *Client) SetProxy(proxyURL string) error {
	if proxyURL == "" {
		return nil
	}
	// Validate URL format
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "socks5" {
		return fmt.Errorf("unsupported proxy scheme: %s (use http, https, or socks5)", parsed.Scheme)
	}
	// req/v3 automatically handles http, https, socks5 if the scheme is provided
	c.HTTP.SetProxyURL(proxyURL)
	return nil
}

// SetUserToken sets the user authentication token for subsequent requests.
func (c *Client) SetUserToken(token string) {
	c.UserToken = token
	c.HTTP.SetCommonHeader("X-User-Auth-Token", token)
}

// Login performs the user login and stores the UserAuthToken
func (c *Client) Login(email, password string) (*LoginResponse, error) {
	var result LoginResponse
	resp, err := c.HTTP.R().
		SetQueryParams(map[string]string{
			"email":    email,
			"password": password,
			"app_id":   c.AppID,
		}).
		SetSuccessResult(&result).
		Get("user/login")

	if err != nil {
		return nil, err
	}

	if resp.IsErrorState() {
		return nil, fmt.Errorf("login failed: %s", resp.String())
	}

	c.SetUserToken(result.UserAuthToken)

	return &result, nil
}

// ValidateSecret checks if the current AppSecret is valid by testing the API.
// Returns true if the secret works, false otherwise.
func (c *Client) ValidateSecret() bool {
	if c.AppSecret == "" {
		return false
	}
	// Test track ID: Daft Punk - Technologic (public track for validation)
	testTrackID := "5966783"
	formatID := 5 // MP3 quality for quick validation

	_, err := c.GetTrackURL(testTrackID, formatID)
	return err == nil
}

// FindValidSecret iterates through potential secrets and finds one that works.
// It validates each secret by attempting to sign a request for a known test track.
// Returns the first valid secret found, or an error if none are valid.
func (c *Client) FindValidSecret(secrets []string) (string, error) {
	// Test track ID: Daft Punk - Technologic (public track for validation)
	testTrackID := "5966783"
	formatID := 5 // MP3 quality for quick validation

	for _, sec := range secrets {
		// Temporary set secret
		c.AppSecret = sec

		// Try to get URL
		_, err := c.GetTrackURL(testTrackID, formatID)
		if err == nil {
			// Found it!
			return sec, nil
		}
	}

	c.AppSecret = ""
	return "", fmt.Errorf("no valid secret found in provided list")
}

// GetTrackURL retrieves the download URL for a track with the specified quality.
// Quality IDs: 5=MP3, 6=FLAC 16-bit, 7=FLAC 24-bit ≤96kHz, 27=FLAC 24-bit >96kHz.
// This endpoint requires a signed request using the app secret.
func (c *Client) GetTrackURL(trackID string, formatID int) (*TrackURLResponse, error) {
	ts := time.Now().Unix()

	// Build signature: concatenate endpoint, params, timestamp, and secret
	rawSig := fmt.Sprintf("trackgetFileUrlformat_id%dintentstreamtrack_id%s%d%s",
		formatID, trackID, ts, c.AppSecret)

	hash := md5.Sum([]byte(rawSig))
	sig := hex.EncodeToString(hash[:])

	params := map[string]string{
		"request_ts":  strconv.FormatInt(ts, 10),
		"request_sig": sig,
		"track_id":    trackID,
		"format_id":   strconv.Itoa(formatID),
		"intent":      "stream",
	}

	var result TrackURLResponse
	resp, err := c.HTTP.R().
		SetQueryParams(params).
		SetSuccessResult(&result).
		Get("track/getFileUrl")

	if err != nil {
		return nil, err
	}

	if resp.IsErrorState() {
		return nil, errors.New(resp.String())
	}

	return &result, nil
}

// GetTrack retrieves metadata for a single track by its ID.
func (c *Client) GetTrack(trackID string) (*TrackMetadata, error) {
	var result TrackMetadata
	resp, err := c.HTTP.R().
		SetQueryParam("track_id", trackID).
		SetSuccessResult(&result).
		Get("track/get")

	if err != nil {
		return nil, err
	}

	if resp.IsErrorState() {
		return nil, errors.New(resp.String())
	}

	return &result, nil
}

// GetAlbum retrieves metadata for an album by its ID, including all tracks.
func (c *Client) GetAlbum(albumID string) (*AlbumMetadata, error) {
	var result AlbumMetadata
	resp, err := c.HTTP.R().
		SetQueryParam("album_id", albumID).
		SetSuccessResult(&result).
		Get("album/get")

	if err != nil {
		return nil, err
	}

	if resp.IsErrorState() {
		return nil, errors.New(resp.String())
	}

	return &result, nil
}
