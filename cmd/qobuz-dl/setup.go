package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/config"
)

const secretValidationRetryDelay = 500 * time.Millisecond

type fetchSecretCandidatesFunc func(string, bool) (string, []api.SecretCandidate, error)

type setupOptions struct {
	isServer    bool
	allowPrompt bool
	quiet       bool
}

var fetchSecretCandidates fetchSecretCandidatesFunc = api.FetchSecretCandidates
var loadAccount = config.LoadAccount
var saveAccount = config.SaveAccount
var newAPIClient = func(appID, appSecret string) *api.Client {
	return api.NewClient(appID, appSecret)
}
var applyClientProxySettings = func(client *api.Client) {
	client.SetUseProxy(!flagNoCDN)
}

func newConfiguredClient(appID, appSecret string) (*api.Client, error) {
	client := newAPIClient(appID, appSecret)
	applyClientProxySettings(client)
	if flagProxy != "" {
		if err := client.SetProxy(flagProxy); err != nil {
			return nil, fmt.Errorf("configure proxy: %w", err)
		}
	}
	return client, nil
}

func setupClient(isServer bool) (*api.Client, *config.Account, error) {
	return setupClientWithOptions(setupOptions{
		isServer:    isServer,
		allowPrompt: !isServer,
	})
}

func setupClientWithOptions(options setupOptions) (*api.Client, *config.Account, error) {
	if strings.TrimSpace(flagEmail) != "" && strings.TrimSpace(flagToken) != "" {
		return nil, nil, fmt.Errorf("--email and --token are mutually exclusive; use --email to force a fresh password login")
	}

	acc, err := loadAccount()
	if err != nil {
		return nil, nil, fmt.Errorf("load account %s: %w", config.GetAccountPath(), err)
	}
	if acc == nil {
		acc = &config.Account{}
	}

	client, err := setupAppCredentials(acc, options)
	if err != nil {
		return nil, nil, err
	}

	forcePasswordLogin := shouldRefreshLogin(flagEmail, acc)
	if forcePasswordLogin {
		acc.UserToken = ""
		acc.TokenAppID = ""
		acc.TokenAppSecret = ""
		client.SetUserToken("")
	}

	userToken := strings.TrimSpace(flagToken)
	if userToken == "" && !forcePasswordLogin {
		userToken = acc.UserToken
	}

	if userToken != "" && !forcePasswordLogin {
		printUnlessQuiet(options, "Validating saved user token...")
		response, tokenErr := validateTokenWithAppAffinity(client, acc, userToken)
		if tokenErr == nil {
			if flagPassword != "" {
				acc.Password = flagPassword
			}
			updateAccountFromLogin(acc, response, client.AppID, client.AppSecret)
		} else if errors.Is(tokenErr, api.ErrTokenInvalid) {
			printUnlessQuiet(options, "Saved user token is invalid or expired. Signing in again...")
			acc.UserToken = ""
			acc.TokenAppID = ""
			acc.TokenAppSecret = ""
			client.SetUserToken("")
			userToken = ""
		} else {
			return nil, nil, fmt.Errorf("validate user token: %w", tokenErr)
		}
	}

	if client.UserToken == "" {
		email, password := resolveLoginCredentials(acc)
		hasCredentials := email != "" && password != ""
		if options.isServer && !hasCredentials && !options.allowPrompt && flagEmail == "" && flagPassword == "" {
			printUnlessQuiet(options, "Warning: Starting server without user authentication. Some features may fail.")
		} else {
			response, loginErr := loginWithPassword(client, acc, options)
			if loginErr != nil {
				return nil, nil, loginErr
			}
			updateAccountFromLogin(acc, response, client.AppID, client.AppSecret)
		}
	}

	if client.UserToken == "" {
		acc.TokenAppID = ""
		acc.TokenAppSecret = ""
	}

	acc.AppID = client.AppID
	acc.AppSecret = client.AppSecret
	acc.PendingSecrets = nil
	if !flagNoSave {
		if err := saveAccount(acc); err != nil {
			return nil, nil, fmt.Errorf("save account credentials: %w", err)
		}
	}
	return client, acc, nil
}

func setupAppCredentials(acc *config.Account, options setupOptions) (*api.Client, error) {
	appID := strings.TrimSpace(flagAppID)
	appSecret := strings.TrimSpace(flagAppSecret)
	pinnedAppID := appID
	explicitSecret := appSecret != ""
	if appID == "" {
		appID = acc.AppID
	}
	if appSecret == "" && acc.AppID == appID {
		appSecret = acc.AppSecret
	}
	if explicitSecret && appID == "" {
		return nil, fmt.Errorf("--app-secret requires --app-id or a saved App ID")
	}
	if appID == "" {
		return refreshAppCredentials(acc, "", options)
	}

	client, err := newConfiguredClient(appID, appSecret)
	if err != nil {
		return nil, err
	}
	status, validationErr := validateSecretWithRetry(client)
	if status == api.SecretStatusValid {
		return client, nil
	}
	if status == api.SecretStatusUnknown {
		if errors.Is(validationErr, api.ErrAppIDRejected) {
			return nil, fmt.Errorf("saved App ID was rejected by Qobuz; provide a matching --app-id/--app-secret pair or remove the saved App ID: %w", validationErr)
		}
		return nil, fmt.Errorf("could not validate app secret after retry: %w", validationErr)
	}
	if explicitSecret {
		return nil, fmt.Errorf("custom App ID/App Secret failed validation; explicit credentials are not replaced automatically: %w", validationErr)
	}
	return refreshAppCredentials(acc, pinnedAppID, options)
}

func refreshAppCredentials(acc *config.Account, pinnedAppID string, options setupOptions) (*api.Client, error) {
	printUnlessQuiet(options, "App credentials are missing or invalid. Fetching fresh credentials...")
	fetchedID, candidates, fetchErr := fetchSecretCandidates(flagProxy, !flagNoCDN)
	if fetchErr != nil {
		return nil, fmt.Errorf("fetch app credentials: %w", fetchErr)
	}
	if fetchedID == "" || len(candidates) == 0 {
		return nil, fmt.Errorf("fetch app credentials: no usable App ID or Secret candidates: %w", api.ErrInvalidResponse)
	}
	targetAppID := pinnedAppID
	if targetAppID == "" {
		targetAppID = fetchedID
	}
	if targetAppID != acc.AppID {
		clearToken(acc)
	}

	client, err := newConfiguredClient(targetAppID, "")
	if err != nil {
		return nil, err
	}
	validSecret, err := client.FindValidSecret(secretCandidateValues(candidates))
	if err != nil {
		return nil, fmt.Errorf("validate refreshed app credentials: %w", err)
	}
	client.AppSecret = validSecret
	acc.AppID = targetAppID
	acc.AppSecret = validSecret
	return client, nil
}

func validateSecretWithRetry(client *api.Client) (api.SecretStatus, error) {
	status, err := client.ValidateSecretDetailed()
	if status != api.SecretStatusUnknown {
		return status, err
	}
	time.Sleep(secretValidationRetryDelay)
	return client.ValidateSecretDetailed()
}

func validateTokenWithAppAffinity(client *api.Client, acc *config.Account, token string) (*api.LoginResponse, error) {
	response, err := client.LoginWithToken(token)
	if !errors.Is(err, api.ErrAppIDRejected) || flagAppID != "" || acc.TokenAppID == "" || acc.TokenAppID == client.AppID {
		return response, err
	}
	if acc.TokenAppSecret == "" {
		return nil, fmt.Errorf("token was rejected by App ID %s and no matching saved secret exists for its issuing App ID %s: %w", client.AppID, acc.TokenAppID, err)
	}

	affinityClient, clientErr := newConfiguredClient(acc.TokenAppID, acc.TokenAppSecret)
	if clientErr != nil {
		return nil, clientErr
	}
	response, affinityErr := affinityClient.LoginWithToken(token)
	if affinityErr != nil {
		return nil, fmt.Errorf("token was rejected by App ID %s and its issuing App ID %s: %w", client.AppID, acc.TokenAppID, affinityErr)
	}
	*client = *affinityClient
	return response, nil
}

func resolveLoginCredentials(acc *config.Account) (string, string) {
	email := strings.TrimSpace(flagEmail)
	if email == "" {
		email = acc.Email
	}
	password := flagPassword
	if password == "" && (flagEmail == "" || strings.EqualFold(strings.TrimSpace(flagEmail), strings.TrimSpace(acc.Email))) {
		password = acc.Password
	}
	return email, password
}

func loginWithPassword(client *api.Client, acc *config.Account, options setupOptions) (*api.LoginResponse, error) {
	email, password := resolveLoginCredentials(acc)
	if email == "" || password == "" {
		if !options.allowPrompt {
			return nil, fmt.Errorf("authentication required: provide --token or --email/--password")
		}
		printUnlessQuiet(options, "Authentication required.")
		reader := bufio.NewReader(os.Stdin)
		if email == "" {
			fmt.Print("Email: ")
			value, err := reader.ReadString('\n')
			if err != nil && strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("read email: %w", err)
			}
			email = strings.TrimSpace(value)
		}
		if password == "" {
			value, err := readSecret(reader, "Password: ")
			if err != nil {
				return nil, fmt.Errorf("read password: %w", err)
			}
			password = value
		}
	}
	if email == "" || password == "" {
		return nil, fmt.Errorf("authentication required: provide --token or --email/--password")
	}

	acc.Password = password
	printUnlessQuiet(options, "Logging in...")
	response, err := client.Login(email, password)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}
	return response, nil
}

func updateAccountFromLogin(acc *config.Account, response *api.LoginResponse, appID, appSecret string) {
	if response == nil {
		return
	}
	if flagEmail != "" {
		acc.Email = strings.TrimSpace(flagEmail)
	}
	if response.User.Email != "" {
		acc.Email = response.User.Email
	}
	if flagPassword != "" {
		acc.Password = flagPassword
	}
	acc.UserToken = response.UserAuthToken
	acc.TokenAppID = appID
	acc.TokenAppSecret = appSecret
	acc.UserID = response.User.ID
}

func clearToken(acc *config.Account) {
	acc.UserToken = ""
	acc.TokenAppID = ""
	acc.TokenAppSecret = ""
}

func printUnlessQuiet(options setupOptions, message string) {
	if !options.quiet {
		fmt.Println(message)
	}
}

func secretCandidateValues(candidates []api.SecretCandidate) []string {
	values := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		values = append(values, candidate.Value)
	}
	return values
}
