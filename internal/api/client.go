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

func (c *Client) SetUserToken(token string) {
	c.UserToken = token
	c.HTTP.SetCommonHeader("X-User-Auth-Token", token)
}

// Login performs the user login and stores the UserAuthToken
func (c *Client) Login(email, password string) error {
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
		return err
	}

	if resp.IsError() {
		return fmt.Errorf("login failed: %s", resp.String())
	}

	c.SetUserToken(result.UserAuthToken)

	return nil
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

