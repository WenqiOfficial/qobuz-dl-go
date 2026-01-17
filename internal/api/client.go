package api

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/imroc/req/v3"
)

const (
	BaseURL   = "https://www.qobuz.com/api.json/0.2/"
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:83.0) Gecko/20100101 Firefox/83.0"
)

type Client struct {
	AppID     string
	AppSecret string
	UserToken string
	HTTP      *req.Client
}

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

func (c *Client) SetProxy(proxyURL string) error {
	if proxyURL == "" {
		return nil
	}
	// req/v3 automatically handles http, https, socks5 if the scheme is provided
	c.HTTP.SetProxyURL(proxyURL)
	return nil
}

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

	if resp.IsError() {
		return nil, fmt.Errorf("login failed: %s", resp.String())
	}

	c.SetUserToken(result.UserAuthToken)

	return &result, nil
}

// FindValidSecret iterates through a list of potential secrets and finds the one that works.
// It does this by attempting to sign a request for a known public track.
func (c *Client) FindValidSecret(secrets []string) (string, error) {
	// Test track ID from qopy.py (Daft Punk - Technologic approx?)
	// qopy uses 5966783
	testTrackID := "5966783"
	formatID := 5 // MP3

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

	// Reset secret if none found
	c.AppSecret = ""
	return "", fmt.Errorf("no valid secret found in provided list")
}

func (c *Client) GetTrackURL(trackID string, formatID int) (*TrackURLResponse, error) {
	ts := time.Now().Unix()

	// Signature construction
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

	if resp.IsError() {
		return nil, errors.New(resp.String())
	}

	return &result, nil
}

func (c *Client) GetAlbum(albumID string) (*AlbumMetadata, error) {
	// album/get does not require signature, just app_id (which is in common header)
	var result AlbumMetadata
	resp, err := c.HTTP.R().
		SetQueryParam("album_id", albumID).
		SetSuccessResult(&result).
		Get("album/get")

	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		return nil, errors.New(resp.String())
	}

	return &result, nil
}
