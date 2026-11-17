// Package api provides the Qobuz API client and related utilities.
// It handles authentication, request signing, and all API interactions.
package api

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

// API constants for Qobuz service.
const (
	BaseURLProxy  = "https://qobuz.wenqi.icu/api.json/0.2/" // Cloudflare Workers proxy
	BaseURLDirect = "https://www.qobuz.com/api.json/0.2/"   // Direct Qobuz API
	UserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:83.0) Gecko/20100101 Firefox/83.0"
)

// Client is the Qobuz API client that handles all API requests.
// It manages authentication state and request signing.
type Client struct {
	HTTP        *req.Client // HTTP client with configured defaults
	AppID       string      // Application ID obtained from Qobuz web player
	AppSecret   string      // Application secret for request signing
	UserToken   string      // User authentication token
	UseProxy    bool        // Whether to use proxy site (default true)
	currentBase string      // Current base URL in use
	directBase  string      // Direct API base URL used for request-scoped fallback
}

// NewClient creates a new Qobuz API client with the given credentials.
// The client is configured with default headers and base URL.
// By default, it tries the proxy site first.
func NewClient(appID, appSecret string) *Client {
	c := &Client{
		AppID:      appID,
		AppSecret:  appSecret,
		HTTP:       req.NewClient(),
		UseProxy:   true,
		directBase: BaseURLDirect,
	}

	// Start with proxy by default
	c.currentBase = BaseURLProxy
	c.HTTP.SetBaseURL(c.currentBase).
		SetUserAgent(UserAgent).
		SetCommonHeader("X-App-Id", appID).
		SetCommonHeader("Content-Type", "application/json;charset=UTF-8")

	return c
}

// NewClientDirect creates a client that uses direct Qobuz API without proxy.
func NewClientDirect(appID, appSecret string) *Client {
	c := NewClient(appID, appSecret)
	c.SetUseProxy(false)
	return c
}

// SetUseProxy enables or disables the proxy site.
func (c *Client) SetUseProxy(useProxy bool) {
	c.UseProxy = useProxy
	if useProxy {
		c.currentBase = BaseURLProxy
	} else {
		c.currentBase = BaseURLDirect
	}
	c.HTTP.SetBaseURL(c.currentBase)
}

// SetDirectBaseURL overrides the direct API endpoint, primarily for isolated tests.
func (c *Client) SetDirectBaseURL(baseURL string) {
	c.directBase = strings.TrimRight(baseURL, "/")
}

// GetCurrentBaseURL returns the current API base URL.
func (c *Client) GetCurrentBaseURL() string {
	return c.currentBase
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
	if token == "" {
		c.HTTP.Headers.Del("X-User-Auth-Token")
		return
	}
	c.HTTP.SetCommonHeader("X-User-Auth-Token", token)
}

// Login performs the user login and stores the UserAuthToken.
// If UseProxy is true, tries proxy first then falls back to direct.
func (c *Client) Login(email, password string) (*LoginResponse, error) {
	return c.loginWithParams(map[string]string{
		"email":    email,
		"password": password,
		"app_id":   c.AppID,
	})
}

// LoginWithToken validates a user token and refreshes the authenticated session.
func (c *Client) LoginWithToken(token string) (*LoginResponse, error) {
	result, err := c.loginWithParams(map[string]string{
		"user_auth_token": token,
		"app_id":          c.AppID,
	})
	if err != nil {
		var statusErr *StatusError
		if errors.As(err, &statusErr) && statusErr.Code == http.StatusUnauthorized {
			if statusErr.Kind == ErrorKindBadCredentials {
				statusErr.Kind = ErrorKindTokenInvalid
			}
			c.SetUserToken("")
		}
	}
	return result, err
}

func (c *Client) loginInternal(email, password string) (*LoginResponse, error) {
	return c.loginAtBase(map[string]string{
		"email":    email,
		"password": password,
		"app_id":   c.AppID,
	}, "")
}

func (c *Client) loginWithParams(params map[string]string) (*LoginResponse, error) {
	result, err := c.loginAtBase(params, "")
	if err != nil && c.UseProxy && c.currentBase == BaseURLProxy && isTransportOrServerError(err) {
		fmt.Fprintln(os.Stderr, "Proxy failed, trying direct API for this login request...")
		return c.loginAtBase(params, c.directBase)
	}
	return result, err
}

func (c *Client) loginAtBase(params map[string]string, baseURL string) (*LoginResponse, error) {
	request := c.HTTP.R().SetQueryParams(params)
	endpoint := "user/login"
	if baseURL != "" {
		endpoint = strings.TrimRight(baseURL, "/") + "/user/login"
	}

	resp, err := request.Get(endpoint)

	if err != nil {
		return nil, err
	}

	if resp.IsErrorState() {
		return nil, responseError(resp, "user/login")
	}

	var result LoginResponse
	if err := resp.UnmarshalJson(&result); err != nil {
		return nil, invalidResponseError("user/login", "login returned a non-JSON response")
	}
	if strings.TrimSpace(result.UserAuthToken) == "" {
		return nil, invalidResponseError("user/login", "login response did not contain a user auth token")
	}

	c.SetUserToken(result.UserAuthToken)

	return &result, nil
}

type SecretStatus int

const (
	SecretStatusUnknown SecretStatus = iota
	SecretStatusValid
	SecretStatusInvalid
)

// ValidateSecret checks if the current AppSecret is valid by testing the API.
// Returns true if the secret works, false otherwise.
func (c *Client) ValidateSecret() bool {
	status, _ := c.ValidateSecretDetailed()
	return status == SecretStatusValid
}

// ValidateSecretDetailed distinguishes a bad signature from auth and transport failures.
func (c *Client) ValidateSecretDetailed() (SecretStatus, error) {
	if c.AppSecret == "" {
		return SecretStatusInvalid, fmt.Errorf("app secret is empty: %w", ErrInvalidSecret)
	}

	_, err := c.GetTrackURL("5966783", 5)
	if err == nil {
		return SecretStatusValid, nil
	}

	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		switch statusErr.Code {
		case http.StatusOK, http.StatusForbidden:
			return SecretStatusValid, nil
		case http.StatusUnauthorized:
			if statusErr.Kind == ErrorKindAppIDRejected {
				return SecretStatusUnknown, err
			}
			return SecretStatusValid, nil
		case http.StatusBadRequest:
			if statusErr.Kind == ErrorKindInvalidSecret {
				return SecretStatusInvalid, err
			}
		}
	}
	return SecretStatusUnknown, err
}

// FindValidSecret iterates through potential secrets and finds one that works.
// It validates each secret by attempting to sign a request for a known test track.
// Returns the first valid secret found, or an error if none are valid.
func (c *Client) FindValidSecret(secrets []string) (string, error) {
	for _, sec := range secrets {
		c.AppSecret = sec
		status, err := c.ValidateSecretDetailed()
		if status == SecretStatusValid {
			return sec, nil
		}
		if status == SecretStatusUnknown {
			c.AppSecret = ""
			return "", fmt.Errorf("secret validation was inconclusive: %w", err)
		}
	}

	c.AppSecret = ""
	return "", fmt.Errorf("no valid secret found in provided list: %w", ErrInvalidSecret)
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
		return nil, responseError(resp, "track/getFileUrl")
	}

	return &result, nil
}

// qualityOrder defines the quality hierarchy from highest to lowest.
// 27=Hi-Res (24-bit >96kHz), 7=24-bit ≤96kHz, 6=16-bit, 5=MP3
var qualityOrder = []int{27, 7, 6, 5}

// GetTrackURLWithFallback tries the requested quality first, then falls back to lower qualities only.
// Returns the first successful TrackURLResponse and the quality ID actually used.
// Example: request 7 → tries 7 → 6 → 5 (never tries 27 which is higher).
func (c *Client) GetTrackURLWithFallback(trackID string, requestedFormatID int) (*TrackURLResponse, int, error) {
	// Find starting index in quality hierarchy
	startIdx := 0
	for i, q := range qualityOrder {
		if q == requestedFormatID {
			startIdx = i
			break
		}
	}

	// If requested quality not in our list, start from it then fall through our list
	var qualities []int
	found := false
	for _, q := range qualityOrder {
		if q == requestedFormatID {
			found = true
		}
	}

	if found {
		// Start from requested quality, only try equal or lower
		qualities = qualityOrder[startIdx:]
	} else {
		// Unknown quality ID: try it first, then all known qualities from highest to lowest
		qualities = append([]int{requestedFormatID}, qualityOrder...)
	}

	var lastErr error
	for _, q := range qualities {
		info, err := c.GetTrackURL(trackID, q)
		if err == nil {
			return info, q, nil
		}
		lastErr = err
		if preventsQualityFallback(err) {
			return nil, 0, err
		}
	}

	return nil, 0, fmt.Errorf("all quality fallbacks failed (tried %v): %w", qualities, lastErr)
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
		return nil, responseError(resp, "track/get")
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
		return nil, responseError(resp, "album/get")
	}

	return &result, nil
}

// SearchAlbums searches for albums matching the given query.
func (c *Client) SearchAlbums(query string, limit int) (*SearchAlbumsResponse, error) {
	var result SearchAlbumsResponse
	resp, err := c.HTTP.R().
		SetQueryParams(map[string]string{
			"query": query,
			"limit": strconv.Itoa(limit),
		}).
		SetSuccessResult(&result).
		Get("album/search")

	if err != nil {
		return nil, err
	}
	if resp.IsErrorState() {
		return nil, responseError(resp, "album/search")
	}
	return &result, nil
}

// SearchTracks searches for tracks matching the given query.
func (c *Client) SearchTracks(query string, limit int) (*SearchTracksResponse, error) {
	var result SearchTracksResponse
	resp, err := c.HTTP.R().
		SetQueryParams(map[string]string{
			"query": query,
			"limit": strconv.Itoa(limit),
		}).
		SetSuccessResult(&result).
		Get("track/search")

	if err != nil {
		return nil, err
	}
	if resp.IsErrorState() {
		return nil, responseError(resp, "track/search")
	}
	return &result, nil
}

// SearchArtists searches for artists matching the given query.
func (c *Client) SearchArtists(query string, limit int) (*SearchArtistsResponse, error) {
	var result SearchArtistsResponse
	resp, err := c.HTTP.R().
		SetQueryParams(map[string]string{
			"query": query,
			"limit": strconv.Itoa(limit),
		}).
		SetSuccessResult(&result).
		Get("artist/search")

	if err != nil {
		return nil, err
	}
	if resp.IsErrorState() {
		return nil, responseError(resp, "artist/search")
	}
	return &result, nil
}

// GetArtistAlbums retrieves an artist's albums by artist ID.
func (c *Client) GetArtistAlbums(artistID string, limit, offset int) (*ArtistAlbumsResponse, error) {
	var result ArtistAlbumsResponse
	resp, err := c.HTTP.R().
		SetQueryParams(map[string]string{
			"artist_id": artistID,
			"extra":     "albums",
			"limit":     strconv.Itoa(limit),
			"offset":    strconv.Itoa(offset),
		}).
		SetSuccessResult(&result).
		Get("artist/get")

	if err != nil {
		return nil, err
	}
	if resp.IsErrorState() {
		return nil, responseError(resp, "artist/get")
	}
	return &result, nil
}
