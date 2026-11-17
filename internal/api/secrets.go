package api

import (
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/imroc/req/v3"
)

// Regular expressions for extracting secrets from Qobuz web player bundle.
var (
	// bundleURLRegex finds the bundle.js URL in the login page.
	bundleURLRegex = regexp.MustCompile(`(?i)<script[^>]+src=['"]([^'"]*bundle[^'"]*\.js)['"]`)
	// appIDRegex extracts the app ID from the bundle.
	appIDRegex         = regexp.MustCompile(`production:\s*\{\s*api\s*:\s*\{\s*appId\s*:\s*["'](?P<app_id>\d{9})["']\s*,\s*appSecret\s*:\s*["']\w{32}["']`)
	appIDFallbackRegex = regexp.MustCompile(`appId\s*:\s*["'](?P<app_id>[^"']+)["']\s*,\s*appSecret\s*:`)
	// privateKeyRegex extracts OAuth private_key from bundle.
	privateKeyRegex = regexp.MustCompile(`privateKey:\s*"(?P<key>[A-Za-z0-9]{6,30})"`)
	// seedTimezoneRegex finds seed values paired with timezone names.
	seedTimezoneRegex = regexp.MustCompile(`[a-z]\.initialSeed\("(?P<seed>[\w=]+)",window\.utimezone\.(?P<timezone>[a-z]+)\)`)
	// infoExtrasRegex finds additional info and extras for secret construction.
	infoExtrasRegex = regexp.MustCompile(`name:"\w+/(?P<timezone>[a-zA-Z]+)",info:"(?P<info>[\w=]*)",extras:"(?P<extras>[\w=]*)"`)
)

// SecretCandidate is a decoded app secret together with its bundle timezone.
type SecretCandidate struct {
	Timezone string
	Value    string
}

// FetchSecrets scrapes the App ID and potential secrets from the Qobuz web player.
// It fetches the login page, locates the bundle.js, and extracts credentials.
// Returns the App ID, a list of potential secrets, and any error encountered.
// proxyURL is optional; pass empty string to use direct connection.
// useProxySite controls whether to try the CDN proxy first.
func FetchSecrets(proxyURL string, useProxySite bool) (string, []string, error) {
	appID, secrets, _, err := FetchSecretsAndPrivateKey(proxyURL, useProxySite)
	return appID, secrets, err
}

// FetchSecretCandidates scrapes app credentials and keeps timezone metadata.
func FetchSecretCandidates(proxyURL string, useProxySite bool) (string, []SecretCandidate, error) {
	appID, candidates, _, err := FetchSecretCandidatesAndPrivateKey(proxyURL, useProxySite)
	return appID, candidates, err
}

// FetchSecretsAndPrivateKey scrapes app_id, potential secrets and OAuth private_key.
func FetchSecretsAndPrivateKey(proxyURL string, useProxySite bool) (string, []string, string, error) {
	appID, candidates, privateKey, err := FetchSecretCandidatesAndPrivateKey(proxyURL, useProxySite)
	secrets := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		secrets = append(secrets, candidate.Value)
	}
	return appID, secrets, privateKey, err
}

// FetchSecretCandidatesAndPrivateKey scrapes app_id, ordered secrets and OAuth private_key.
func FetchSecretCandidatesAndPrivateKey(proxyURL string, useProxySite bool) (string, []SecretCandidate, string, error) {
	client := req.NewClient()
	if proxyURL != "" {
		client.SetProxyURL(proxyURL)
	}

	// Determine base URL
	playBaseURL := "https://play.qobuz.com"
	playBaseURLProxy := "https://play-qobuz.wenqi.icu"

	// Try proxy first if enabled
	if useProxySite {
		appID, secrets, privateKey, err := fetchSecretCandidatesFromHost(client, playBaseURLProxy)
		if err == nil {
			return appID, secrets, privateKey, nil
		}
		fmt.Fprintln(os.Stderr, "CDN proxy failed for secrets, falling back to direct...")
	}

	// Direct connection
	return fetchSecretCandidatesFromHost(client, playBaseURL)
}

// fetchSecretsFromHost fetches secrets from a specific host.
func fetchSecretsFromHost(client *req.Client, baseURL string) (string, []string, string, error) {
	appID, candidates, privateKey, err := fetchSecretCandidatesFromHost(client, baseURL)
	secrets := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		secrets = append(secrets, candidate.Value)
	}
	return appID, secrets, privateKey, err
}

func fetchSecretCandidatesFromHost(client *req.Client, baseURL string) (string, []SecretCandidate, string, error) {
	// 1. Get Login Page to find bundle URL
	resp, err := client.R().Get(baseURL + "/login")
	if err != nil {
		return "", nil, "", err
	}

	matches := bundleURLRegex.FindStringSubmatch(resp.String())
	if len(matches) < 2 {
		return "", nil, "", fmt.Errorf("bundle URL not found (status %d)", resp.StatusCode)
	}
	bundleURL := matches[1]
	if !strings.HasPrefix(bundleURL, "http://") && !strings.HasPrefix(bundleURL, "https://") {
		if !strings.HasPrefix(bundleURL, "/") {
			bundleURL = "/" + bundleURL
		}
		bundleURL = baseURL + bundleURL
	}

	// 2. Get Bundle JS
	resp, err = client.R().Get(bundleURL)
	if err != nil {
		return "", nil, "", err
	}
	bundleContent := resp.String()

	appID := extractAppID(bundleContent)
	if appID == "" {
		return "", nil, "", fmt.Errorf("app ID not found in bundle")
	}

	privateKey := ""
	if m := privateKeyRegex.FindStringSubmatch(bundleContent); len(m) >= 2 {
		privateKey = m[1]
	}

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

	timezones = timezones[:0]
	for timezone := range secretsMap {
		timezones = append(timezones, timezone)
	}
	sort.Slice(timezones, func(i, j int) bool {
		if timezones[i] == "berlin" {
			return true
		}
		if timezones[j] == "berlin" {
			return false
		}
		return timezones[i] < timezones[j]
	})

	var validSecrets []SecretCandidate
	seenSecrets := make(map[string]struct{})
	for _, timezone := range timezones {
		parts := secretsMap[timezone]
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
			fmt.Fprintf(os.Stderr, "Base64 decode failed for %s: %v\n", timezone, err)
			continue
		}

		secret := string(decodedBytes)
		if secret == "" {
			continue
		}
		if _, exists := seenSecrets[secret]; exists {
			continue
		}
		seenSecrets[secret] = struct{}{}
		validSecrets = append(validSecrets, SecretCandidate{Timezone: timezone, Value: secret})
	}

	if len(validSecrets) == 0 {
		return appID, nil, privateKey, fmt.Errorf("no valid secrets extracted")
	}

	return appID, validSecrets, privateKey, nil
}

func extractAppID(bundleContent string) string {
	for _, expression := range []*regexp.Regexp{appIDRegex, appIDFallbackRegex} {
		matches := expression.FindStringSubmatch(bundleContent)
		if len(matches) >= 2 {
			return matches[1]
		}
	}
	return ""
}
