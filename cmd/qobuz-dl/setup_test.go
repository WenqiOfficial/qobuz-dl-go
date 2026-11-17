package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/config"
)

func setupCommandTest(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldAppID, oldAppSecret := flagAppID, flagAppSecret
	oldEmail, oldPassword, oldToken := flagEmail, flagPassword, flagToken
	oldProxy, oldNoCDN, oldNoSave := flagProxy, flagNoCDN, flagNoSave
	oldFetch := fetchSecretCandidates
	oldNewAPIClient := newAPIClient
	oldApplyClientProxySettings := applyClientProxySettings
	oldLoadAccount := loadAccount
	oldSaveAccount := saveAccount
	t.Cleanup(func() {
		flagAppID, flagAppSecret = oldAppID, oldAppSecret
		flagEmail, flagPassword, flagToken = oldEmail, oldPassword, oldToken
		flagProxy, flagNoCDN, flagNoSave = oldProxy, oldNoCDN, oldNoSave
		fetchSecretCandidates = oldFetch
		newAPIClient = oldNewAPIClient
		applyClientProxySettings = oldApplyClientProxySettings
		loadAccount = oldLoadAccount
		saveAccount = oldSaveAccount
	})
	flagAppID, flagAppSecret = "", ""
	flagEmail, flagPassword, flagToken = "", "", ""
	flagProxy, flagNoCDN = "", true
	flagNoSave = true
	loadAccount = func() (*config.Account, error) {
		return &config.Account{}, nil
	}
	saveAccount = func(*config.Account) error {
		return nil
	}
	newAPIClient = func(appID, appSecret string) *api.Client {
		client := api.NewClient(appID, appSecret)
		client.SetUseProxy(false)
		client.HTTP.SetBaseURL(server.URL)
		return client
	}
	applyClientProxySettings = func(client *api.Client) {
		client.SetUseProxy(false)
		client.HTTP.SetBaseURL(server.URL)
	}
	fetchSecretCandidates = func(string, bool) (string, []api.SecretCandidate, error) {
		return "bundle-app", []api.SecretCandidate{{Timezone: "berlin", Value: "fresh-secret"}}, nil
	}
	return server
}

func useAccount(acc *config.Account) {
	loadAccount = func() (*config.Account, error) {
		return acc, nil
	}
}

func useTestAPIBase(client *api.Client, serverURL string) {
	client.HTTP.SetBaseURL(serverURL)
}

func TestSetupAppCredentialsTreatsUnauthorizedAsValidSecret(t *testing.T) {
	var requests atomic.Int32
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
	})
	acc := &config.Account{AppID: "saved-app", AppSecret: "saved-secret"}
	client, err := setupAppCredentials(acc, setupOptions{quiet: true})
	if err != nil {
		t.Fatalf("setupAppCredentials() error = %v", err)
	}
	if client.AppID != "saved-app" || client.AppSecret != "saved-secret" || requests.Load() != 1 {
		t.Fatalf("client credentials = %q/%q, requests = %d", client.AppID, client.AppSecret, requests.Load())
	}
}

func TestSetupAppCredentialsRefreshesOnlyRejectedSignature(t *testing.T) {
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-App-Id") == "old-app" {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"message":"Invalid Request Signature"}`))
			return
		}
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
	})
	acc := &config.Account{AppID: "old-app", AppSecret: "old-secret", UserToken: "old-token", TokenAppID: "old-app"}
	client, err := setupAppCredentials(acc, setupOptions{quiet: true})
	if err != nil {
		t.Fatalf("setupAppCredentials() error = %v", err)
	}
	if client.AppID != "bundle-app" || client.AppSecret != "fresh-secret" {
		t.Fatalf("credentials = %q/%q", client.AppID, client.AppSecret)
	}
	if acc.UserToken != "" || acc.TokenAppID != "" {
		t.Fatal("rotating the App ID must invalidate the old token")
	}
}

func TestSetupAppCredentialsDoesNotRefreshAfterNetworkFailure(t *testing.T) {
	server := setupCommandTest(t, nil)
	server.Close()
	var fetches atomic.Int32
	fetchSecretCandidates = func(string, bool) (string, []api.SecretCandidate, error) {
		fetches.Add(1)
		return "", nil, errors.New("must not fetch")
	}
	acc := &config.Account{AppID: "saved-app", AppSecret: "saved-secret"}
	_, err := setupAppCredentials(acc, setupOptions{quiet: true})
	if err == nil || !strings.Contains(err.Error(), "could not validate app secret after retry") {
		t.Fatalf("error = %v", err)
	}
	if fetches.Load() != 0 {
		t.Fatal("a transport failure must not refresh app credentials")
	}
}

func TestValidateTokenWithAppAffinityRetriesIssuingAppID(t *testing.T) {
	var appIDs []string
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		appIDs = append(appIDs, request.Header.Get("X-App-Id"))
		writer.Header().Set("Content-Type", "application/json")
		if request.Header.Get("X-App-Id") == "current-app" {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"Invalid app_id"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"user_auth_token":"token","user":{"id":42}}`))
	})
	acc := &config.Account{TokenAppID: "issuing-app", TokenAppSecret: "issuing-secret"}
	client, err := newConfiguredClient("current-app", "secret")
	if err != nil {
		t.Fatal(err)
	}
	response, err := validateTokenWithAppAffinity(client, acc, "token")
	if err != nil || response.UserAuthToken != "token" || client.AppID != "issuing-app" {
		t.Fatalf("response = %#v, error = %v, AppID = %q", response, err, client.AppID)
	}
	if strings.Join(appIDs, ",") != "current-app,issuing-app" {
		t.Fatalf("App ID attempts = %v", appIDs)
	}
}

func TestUpdateAccountFromLoginSavesTokenAppIDAndPassword(t *testing.T) {
	setupCommandTest(t, nil)
	flagEmail = "typed@example.com"
	flagPassword = "new-password"
	acc := &config.Account{Email: "old@example.com", Password: "old-password"}
	response := &api.LoginResponse{UserAuthToken: "fresh-token"}
	response.User.ID = 42
	updateAccountFromLogin(acc, response, "app-id", "app-secret")
	if acc.Email != "typed@example.com" || acc.Password != "new-password" ||
		acc.UserToken != "fresh-token" || acc.TokenAppID != "app-id" ||
		acc.TokenAppSecret != "app-secret" || acc.UserID != 42 {
		t.Fatalf("account = %+v", acc)
	}
}

func TestSetupClientExplicitEmailForcesPasswordLogin(t *testing.T) {
	var tokenValidations, passwordLogins atomic.Int32
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			query := request.URL.Query()
			if query.Get("user_auth_token") != "" {
				tokenValidations.Add(1)
				writer.WriteHeader(http.StatusUnauthorized)
				_, _ = writer.Write([]byte(`{"message":"Invalid user auth token"}`))
				return
			}
			passwordLogins.Add(1)
			_, _ = writer.Write([]byte(`{"user_auth_token":"fresh-token","user":{"id":42}}`))
		default:
			http.NotFound(writer, request)
		}
	})
	acc := &config.Account{
		Email:          "old@example.com",
		Password:       "old-password",
		UserToken:      "saved-token",
		TokenAppID:     "app-id",
		TokenAppSecret: "secret",
		AppID:          "app-id",
		AppSecret:      "secret",
	}
	useAccount(acc)
	flagEmail = "new@example.com"
	flagPassword = "new-password"

	client, saved, err := setupClientWithOptions(setupOptions{allowPrompt: false})
	if err != nil {
		t.Fatalf("setupClientWithOptions() error = %v", err)
	}
	if tokenValidations.Load() != 0 || passwordLogins.Load() != 1 {
		t.Fatalf("token/password requests = %d/%d; want 0/1", tokenValidations.Load(), passwordLogins.Load())
	}
	if client.UserToken != "fresh-token" || saved.Email != "new@example.com" || saved.Password != "new-password" {
		t.Fatalf("client token = %q, account = %+v", client.UserToken, saved)
	}
}

func TestSetupClientExpiredTokenUsesSavedPasswordWithoutRefreshingSecret(t *testing.T) {
	var trackRequests, tokenValidations, passwordLogins atomic.Int32
	var fetches atomic.Int32
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			trackRequests.Add(1)
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			query := request.URL.Query()
			if query.Get("user_auth_token") == "expired-token" {
				tokenValidations.Add(1)
				writer.WriteHeader(http.StatusUnauthorized)
				_, _ = writer.Write([]byte(`{"message":"Invalid user auth token"}`))
				return
			}
			passwordLogins.Add(1)
			_, _ = writer.Write([]byte(`{"user_auth_token":"renewed-token","user":{"id":42,"email":"user@example.com"}}`))
		default:
			http.NotFound(writer, request)
		}
	})
	fetchSecretCandidates = func(string, bool) (string, []api.SecretCandidate, error) {
		fetches.Add(1)
		return "", nil, errors.New("must not refresh secret for an expired token")
	}
	acc := &config.Account{
		Email:          "user@example.com",
		Password:       "saved-password",
		UserToken:      "expired-token",
		TokenAppID:     "app-id",
		TokenAppSecret: "secret",
		AppID:          "app-id",
		AppSecret:      "secret",
	}
	useAccount(acc)

	client, saved, err := setupClientWithOptions(setupOptions{allowPrompt: true})
	if err != nil {
		t.Fatalf("setupClientWithOptions() error = %v", err)
	}
	if fetches.Load() != 0 || trackRequests.Load() != 1 || tokenValidations.Load() != 1 || passwordLogins.Load() != 1 {
		t.Fatalf("fetch/track/token/password = %d/%d/%d/%d", fetches.Load(), trackRequests.Load(), tokenValidations.Load(), passwordLogins.Load())
	}
	if client.UserToken != "renewed-token" || saved.UserToken != "renewed-token" || saved.TokenAppID != "app-id" {
		t.Fatalf("client token = %q, account = %+v", client.UserToken, saved)
	}
}

func TestSetupServerWithoutUserCredentialsStartsWithoutLogin(t *testing.T) {
	var logins atomic.Int32
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			logins.Add(1)
			_, _ = writer.Write([]byte(`{"user_auth_token":"unexpected"}`))
		default:
			http.NotFound(writer, request)
		}
	})
	acc := &config.Account{}
	useAccount(acc)

	client, saved, err := setupClientWithOptions(setupOptions{isServer: true, allowPrompt: false, quiet: true})
	if err != nil {
		t.Fatalf("setupClientWithOptions() error = %v", err)
	}
	if logins.Load() != 0 || client.UserToken != "" {
		t.Fatalf("logins = %d, token = %q; server should start unauthenticated", logins.Load(), client.UserToken)
	}
	if saved.AppID != "bundle-app" || saved.AppSecret != "fresh-secret" || saved.UserToken != "" {
		t.Fatalf("saved account = %+v", saved)
	}
}

func TestSetupClientExplicitEmailDoesNotReuseOtherSavedPassword(t *testing.T) {
	var logins atomic.Int32
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			logins.Add(1)
			_, _ = writer.Write([]byte(`{"user_auth_token":"unexpected"}`))
		}
	})
	acc := &config.Account{
		Email:     "old@example.com",
		Password:  "old-password",
		AppID:     "app-id",
		AppSecret: "secret",
	}
	useAccount(acc)
	flagEmail = "new@example.com"

	_, _, err := setupClientWithOptions(setupOptions{allowPrompt: false, quiet: true})
	if err == nil || !strings.Contains(err.Error(), "authentication required") {
		t.Fatalf("error = %v; want authentication required", err)
	}
	if logins.Load() != 0 {
		t.Fatalf("logins = %d; must not reuse another account's saved password", logins.Load())
	}
}

func TestSetupClientExplicitInvalidSecretIsNotReplaced(t *testing.T) {
	var fetches atomic.Int32
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"message":"Invalid Request Signature"}`))
	})
	fetchSecretCandidates = func(string, bool) (string, []api.SecretCandidate, error) {
		fetches.Add(1)
		return "", nil, errors.New("must not fetch")
	}
	flagAppID = "custom-app"
	flagAppSecret = "bad-secret"

	_, err := setupAppCredentials(&config.Account{}, setupOptions{quiet: true})
	if err == nil || !strings.Contains(err.Error(), "explicit credentials are not replaced") {
		t.Fatalf("error = %v", err)
	}
	if fetches.Load() != 0 {
		t.Fatal("explicit invalid credentials must not trigger a bundle fetch")
	}
}

func TestSetupAppCredentialsReportsRejectedAppIDWithoutRefreshing(t *testing.T) {
	var fetches atomic.Int32
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"Invalid app_id"}`))
	})
	fetchSecretCandidates = func(string, bool) (string, []api.SecretCandidate, error) {
		fetches.Add(1)
		return "", nil, errors.New("must not fetch")
	}

	_, err := setupAppCredentials(&config.Account{AppID: "rejected-app", AppSecret: "secret"}, setupOptions{quiet: true})
	if err == nil || !errors.Is(err, api.ErrAppIDRejected) {
		t.Fatalf("error = %v; want App ID rejection", err)
	}
	if fetches.Load() != 0 {
		t.Fatal("App ID rejection must not rotate credentials")
	}
}

func TestSetupClientReportsAccountLoadFailure(t *testing.T) {
	setupCommandTest(t, nil)
	loadAccount = func() (*config.Account, error) {
		return nil, errors.New("malformed account file")
	}

	_, _, err := setupClientWithOptions(setupOptions{quiet: true})
	if err == nil || !strings.Contains(err.Error(), "load account") {
		t.Fatalf("error = %v; want account load failure", err)
	}
}

func TestSetupClientHonorsNoSave(t *testing.T) {
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
	})
	acc := &config.Account{AppID: "app-id", AppSecret: "secret"}
	useAccount(acc)
	var saves int
	saveAccount = func(*config.Account) error {
		saves++
		return nil
	}

	flagNoSave = true
	if _, _, err := setupClientWithOptions(setupOptions{isServer: true, allowPrompt: false, quiet: true}); err != nil {
		t.Fatalf("setupClientWithOptions(--nosave) error = %v", err)
	}
	if saves != 0 {
		t.Fatalf("saves with --nosave = %d; want 0", saves)
	}

	flagNoSave = false
	if _, _, err := setupClientWithOptions(setupOptions{isServer: true, allowPrompt: false, quiet: true}); err != nil {
		t.Fatalf("setupClientWithOptions(save) error = %v", err)
	}
	if saves != 1 {
		t.Fatalf("saves without --nosave = %d; want 1", saves)
	}
}

func TestSetupClientRejectsEmailAndTokenTogether(t *testing.T) {
	setupCommandTest(t, nil)
	flagEmail = "user@example.com"
	flagToken = "token"

	_, _, err := setupClientWithOptions(setupOptions{quiet: true})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v; want mutual exclusion", err)
	}
}
