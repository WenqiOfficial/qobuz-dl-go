package api

import (
	"fmt"
	"regexp"

	"github.com/imroc/req/v3"
)

var (
	bundleURLRegex = regexp.MustCompile(`<script src="(/resources/\d+\.\d+\.\d+-[a-z]\d{3}/bundle\.js)"></script>`)
	appIDRegex     = regexp.MustCompile(`production:{api:{appId:"(?P<app_id>\d{9})",appSecret:"(?P<app_secret>\w{32})"`)
	// Note: The python regex for AppID was slightly different, checking it again.
	// Python: r'production:{api:{appId:"(?P<app_id>\d{9})",appSecret:"\w{32}"'
	// It didn't capture secret?
	// But Client needs secret.
	// Let's assume we can capture it.
)

// FetchSecrets attempts to scrape the AppID and potential secrets from the Qobuz web player.
func FetchSecrets() (string, string, error) {
	client := req.NewClient()
	
	// 1. Get Login Page to find bundle URL
	resp, err := client.R().Get("https://play.qobuz.com/login")
	if err != nil {
		return "", "", err
	}
	
	matches := bundleURLRegex.FindStringSubmatch(resp.String())
	if len(matches) < 2 {
		return "", "", fmt.Errorf("bundle URL not found")
	}
	bundlePath := matches[1]
	
	// 2. Get Bundle JS
	resp, err = client.R().Get("https://play.qobuz.com" + bundlePath)
	if err != nil {
		return "", "", err
	}
	
	// 3. Extract IDs
	// The regex needs to be robust. 
	// AppID is usually 9 digits. Secret is 32 chars.
	// Regex: appId:"(\d{9})",appSecret:"(\w{32})"
	re := regexp.MustCompile(`appId:"(\d{9})",appSecret:"(\w{32})"`)
	secrets := re.FindStringSubmatch(resp.String())
	if len(secrets) < 3 {
		return "", "", fmt.Errorf("secrets not found in bundle")
	}
	
	return secrets[1], secrets[2], nil
}
