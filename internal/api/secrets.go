package api

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/imroc/req/v3"
)

// Regular expressions for extracting secrets from Qobuz web player bundle.
var (
	// bundleURLRegex finds the bundle.js URL in the login page.
	bundleURLRegex = regexp.MustCompile(`<script src="(/resources/\d+\.\d+\.\d+-[a-z]\d{3}/bundle\.js)"></script>`)
	// appIDRegex extracts the app ID from the bundle.
	appIDRegex = regexp.MustCompile(`production:{api:{appId:"(?P<app_id>\d{9})",appSecret:"\w{32}"`)
	// seedTimezoneRegex finds seed values paired with timezone names.
	seedTimezoneRegex = regexp.MustCompile(`[a-z]\.initialSeed\("(?P<seed>[\w=]+)",window\.utimezone\.(?P<timezone>[a-z]+)\)`)
	// infoExtrasRegex finds additional info and extras for secret construction.
	infoExtrasRegex = regexp.MustCompile(`name:"\w+/(?P<timezone>[a-zA-Z]+)",info:"(?P<info>[\w=]+)",extras:"(?P<extras>[\w=]+)"`)
)

// FetchSecrets scrapes the App ID and potential secrets from the Qobuz web player.
// It fetches the login page, locates the bundle.js, and extracts credentials.
// Returns the App ID, a list of potential secrets, and any error encountered.
// proxyURL is optional; pass empty string to use direct connection.
func FetchSecrets(proxyURL string) (string, []string, error) {
	client := req.NewClient()
	if proxyURL != "" {
		client.SetProxyURL(proxyURL)
	}

	// 1. Get Login Page to find bundle URL
	resp, err := client.R().Get("https://play.qobuz.com/login")
	if err != nil {
		return "", nil, err
	}

	matches := bundleURLRegex.FindStringSubmatch(resp.String())
	if len(matches) < 2 {
		return "", nil, fmt.Errorf("bundle URL not found")
	}
	bundlePath := matches[1]

	// 2. Get Bundle JS
	resp, err = client.R().Get("https://play.qobuz.com" + bundlePath)
	if err != nil {
		return "", nil, err
	}
	bundleContent := resp.String()

	// 3. Extract App ID
	appIDMatches := appIDRegex.FindStringSubmatch(bundleContent)
	if len(appIDMatches) < 2 {
		return "", nil, fmt.Errorf("app ID not found in bundle")
	}
	appID := appIDMatches[1]

	// 4. Extract Secrets
	// Logic ported from bundle.py
	// a. Find seeds and timezones
	seedMatches := seedTimezoneRegex.FindAllStringSubmatch(bundleContent, -1)

	secretsMap := make(map[string][]string) // timezone -> [seed]
	var timezones []string

	for _, m := range seedMatches {
		if len(m) < 3 {
			continue
		}
		seed := m[1]
		timezone := m[2]
		secretsMap[timezone] = []string{seed}
		timezones = append(timezones, timezone)
	}

	// b. Find info and extras
	// bundle.py constructs a regex joining capitalized timezones.
	// We just scan all and match against our map.
	infoMatches := infoExtrasRegex.FindAllStringSubmatch(bundleContent, -1)

	for _, m := range infoMatches {
		if len(m) < 4 {
			continue
		}
		// m[1] is timezone (Capitalized usually, like Berlin)
		tzCap := m[1]
		info := m[2]
		extras := m[3]

		tzLower := strings.ToLower(tzCap)
		if list, ok := secretsMap[tzLower]; ok {
			secretsMap[tzLower] = append(list, info, extras)
		}
	}

	var validSecrets []string
	for _, parts := range secretsMap {
		if len(parts) != 3 {
			// Needs seed, info, extras
			continue
		}
		seed, info, extras := parts[0], parts[1], parts[2]
		combined := seed + info + extras

		if len(combined) <= 44 {
			continue
		}
		// Python logic: base64.standard_b64decode("".join(secrets[secret_pair])[:-44])
		// Slice BEFORE decoding
		toDecode := combined[:len(combined)-44]

		decodedBytes, err := base64.StdEncoding.DecodeString(toDecode)
		if err != nil {
			fmt.Printf("Base64 decode failed for %s: %v\n", parts[0], err)
			continue
		}

		secret := string(decodedBytes)
		validSecrets = append(validSecrets, secret)
	}

	if len(validSecrets) == 0 {
		return appID, nil, fmt.Errorf("no valid secrets extracted")
	}

	return appID, validSecrets, nil
}
