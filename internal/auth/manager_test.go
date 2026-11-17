package auth

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/config"
)

type testEnv struct {
	Manager       *Manager
	Server        *httptest.Server
	Saves         atomic.Int32
	Fetches       atomic.Int32
	FetchProxy    string
	FetchUseProxy bool
	FetchFunc     func(string, bool) (string, []api.SecretCandidate, error)
	LoadError     error
}

func newTestEnv(t *testing.T, handler http.HandlerFunc, account *config.Account, options Options) *testEnv {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	env := &testEnv{Server: server}
	if options.Input == nil {
		options.Input = strings.NewReader("")
	}
	if options.Output == nil {
		options.Output = io.Discard
	}
	deps := Dependencies{
		LoadAccount: func() (*config.Account, error) {
			if env.LoadError != nil {
				return nil, env.LoadError
			}
			if account == nil {
				return &config.Account{}, nil
			}
			return account, nil
		},
		SaveAccount: func(*config.Account) error {
			env.Saves.Add(1)
			return nil
		},
		FetchSecretCandidates: func(proxyURL string, useProxySite bool) (string, []api.SecretCandidate, error) {
			env.Fetches.Add(1)
			env.FetchProxy = proxyURL
			env.FetchUseProxy = useProxySite
			if env.FetchFunc != nil {
				return env.FetchFunc(proxyURL, useProxySite)
			}
			return "bundle-app", []api.SecretCandidate{{Timezone: "berlin", Value: "fresh-secret"}}, nil
		},
		NewClient: func(appID, appSecret string) (*api.Client, error) {
			client := api.NewClient(appID, appSecret)
			client.SetUseProxy(false)
			client.HTTP.SetBaseURL(server.URL)
			return client, nil
		},
		Sleep: func(time.Duration) {},
	}
	env.Manager = NewManagerWithDependencies(options, deps)
	return env
}

func TestPrepareDiagnosticsTreatsUnauthorizedAsValidSecret(t *testing.T) {
	var requests atomic.Int32
	env := newTestEnv(t, func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
	}, &config.Account{AppID: "saved-app", AppSecret: "saved-secret"}, Options{
		DisableProxySite: true,
		Quiet:            true,
	})

	client, _, err := env.Manager.PrepareDiagnostics()
	if err != nil {
		t.Fatalf("PrepareDiagnostics() error = %v", err)
	}
	if client.AppID != "saved-app" || client.AppSecret != "saved-secret" || requests.Load() != 1 {
		t.Fatalf("client credentials = %q/%q, requests = %d", client.AppID, client.AppSecret, requests.Load())
	}
	if env.Manager.State() != StateSecretValid {
		t.Fatalf("state = %q; want %q", env.Manager.State(), StateSecretValid)
	}
}

func TestPrepareRefreshesOnlyRejectedSignature(t *testing.T) {
	env := newTestEnv(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-App-Id") == "old-app" {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"message":"Invalid Request Signature"}`))
			return
		}
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
	}, &config.Account{AppID: "old-app", AppSecret: "old-secret", UserToken: "old-token", TokenAppID: "old-app"}, Options{
		DisableProxySite: true,
		Quiet:            true,
	})

	client, account, err := env.Manager.PrepareDiagnostics()
	if err != nil {
		t.Fatalf("PrepareDiagnostics() error = %v", err)
	}
	if client.AppID != "bundle-app" || client.AppSecret != "fresh-secret" {
		t.Fatalf("credentials = %q/%q", client.AppID, client.AppSecret)
	}
	if account.UserToken != "" || account.TokenAppID != "" || account.TokenAppSecret != "" {
		t.Fatal("rotating the App ID must invalidate the old token")
	}
	if env.Fetches.Load() != 1 {
		t.Fatalf("fetches = %d; want 1", env.Fetches.Load())
	}
}

func TestPrepareDoesNotRefreshAfterNetworkFailure(t *testing.T) {
	env := newTestEnv(t, nil, &config.Account{AppID: "saved-app", AppSecret: "saved-secret"}, Options{
		DisableProxySite: true,
		Quiet:            true,
	})
	env.Server.Close()

	_, _, err := env.Manager.PrepareDiagnostics()
	if err == nil || !strings.Contains(err.Error(), "could not validate app secret after retry") {
		t.Fatalf("error = %v", err)
	}
	if env.Fetches.Load() != 0 {
		t.Fatal("a transport failure must not refresh app credentials")
	}
}

func TestPrepareReportsRejectedAppIDWithoutRefreshing(t *testing.T) {
	env := newTestEnv(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"Invalid app_id"}`))
	}, &config.Account{AppID: "rejected-app", AppSecret: "secret"}, Options{
		DisableProxySite: true,
		Quiet:            true,
	})

	_, _, err := env.Manager.PrepareDiagnostics()
	if err == nil || !errors.Is(err, api.ErrAppIDRejected) {
		t.Fatalf("error = %v; want App ID rejection", err)
	}
	if env.Fetches.Load() != 0 {
		t.Fatal("App ID rejection must not rotate credentials")
	}
}

func TestPrepareDoesNotReplaceExplicitInvalidSecret(t *testing.T) {
	env := newTestEnv(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"message":"Invalid Request Signature"}`))
	}, &config.Account{}, Options{
		AppID:            "custom-app",
		AppSecret:        "bad-secret",
		DisableProxySite: true,
		Quiet:            true,
	})

	_, _, err := env.Manager.PrepareDiagnostics()
	if err == nil || !strings.Contains(err.Error(), "explicit credentials are not replaced") {
		t.Fatalf("error = %v", err)
	}
	if env.Fetches.Load() != 0 {
		t.Fatal("explicit invalid credentials must not trigger a bundle fetch")
	}
}

func TestPrepareExplicitEmailForcesPasswordLogin(t *testing.T) {
	var tokenValidations, passwordLogins atomic.Int32
	env := newTestEnv(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			if request.URL.Query().Get("user_auth_token") != "" {
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
	}, &config.Account{
		Email:          "old@example.com",
		Password:       "old-password",
		UserToken:      "saved-token",
		TokenAppID:     "app-id",
		TokenAppSecret: "secret",
		AppID:          "app-id",
		AppSecret:      "secret",
	}, Options{
		Email:            "new@example.com",
		Password:         "new-password",
		DisableProxySite: true,
		Quiet:            true,
		Save:             true,
	})

	client, account, err := env.Manager.Prepare()
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if tokenValidations.Load() != 0 || passwordLogins.Load() != 1 {
		t.Fatalf("token/password requests = %d/%d; want 0/1", tokenValidations.Load(), passwordLogins.Load())
	}
	if client.UserToken != "fresh-token" || account.Email != "new@example.com" || account.Password != "new-password" {
		t.Fatalf("client token = %q, account = %+v", client.UserToken, account)
	}
	if env.Manager.State() != StateAuthenticated || env.Saves.Load() != 1 {
		t.Fatalf("state/saves = %q/%d", env.Manager.State(), env.Saves.Load())
	}
}

func TestPrepareExpiredTokenUsesSavedPasswordWithoutRefreshingSecret(t *testing.T) {
	var trackRequests, tokenValidations, passwordLogins atomic.Int32
	env := newTestEnv(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			trackRequests.Add(1)
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			if request.URL.Query().Get("user_auth_token") == "expired-token" {
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
	}, &config.Account{
		Email:          "user@example.com",
		Password:       "saved-password",
		UserToken:      "expired-token",
		TokenAppID:     "app-id",
		TokenAppSecret: "secret",
		AppID:          "app-id",
		AppSecret:      "secret",
	}, Options{
		DisableProxySite: true,
		AllowPrompt:      true,
		Quiet:            true,
		Save:             true,
	})
	env.FetchFunc = func(string, bool) (string, []api.SecretCandidate, error) {
		return "", nil, errors.New("must not refresh secret for an expired token")
	}

	client, account, err := env.Manager.Prepare()
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if env.Fetches.Load() != 0 || trackRequests.Load() != 1 || tokenValidations.Load() != 1 || passwordLogins.Load() != 1 {
		t.Fatalf("fetch/track/token/password = %d/%d/%d/%d", env.Fetches.Load(), trackRequests.Load(), tokenValidations.Load(), passwordLogins.Load())
	}
	if client.UserToken != "renewed-token" || account.UserToken != "renewed-token" || account.TokenAppID != "app-id" {
		t.Fatalf("client token = %q, account = %+v", client.UserToken, account)
	}
	if env.Manager.State() != StateAuthenticated {
		t.Fatalf("state = %q; want authenticated", env.Manager.State())
	}
}

func TestPrepareServerWithoutUserCredentialsStartsWithoutLogin(t *testing.T) {
	var logins atomic.Int32
	env := newTestEnv(t, func(writer http.ResponseWriter, request *http.Request) {
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
	}, &config.Account{}, Options{
		Server:           true,
		AllowPrompt:      false,
		DisableProxySite: true,
		Quiet:            true,
		Save:             true,
	})

	client, account, err := env.Manager.Prepare()
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if logins.Load() != 0 || client.UserToken != "" {
		t.Fatalf("logins = %d, token = %q; server should start unauthenticated", logins.Load(), client.UserToken)
	}
	if account.AppID != "bundle-app" || account.AppSecret != "fresh-secret" || account.UserToken != "" {
		t.Fatalf("saved account = %+v", account)
	}
	if env.Manager.State() != StateUnauthenticated || env.Saves.Load() != 1 {
		t.Fatalf("state/saves = %q/%d", env.Manager.State(), env.Saves.Load())
	}
}

func TestPrepareExplicitEmailDoesNotReuseOtherSavedPassword(t *testing.T) {
	var logins atomic.Int32
	env := newTestEnv(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			logins.Add(1)
			_, _ = writer.Write([]byte(`{"user_auth_token":"unexpected"}`))
		}
	}, &config.Account{
		Email:     "old@example.com",
		Password:  "old-password",
		AppID:     "app-id",
		AppSecret: "secret",
	}, Options{
		Email:            "new@example.com",
		AllowPrompt:      false,
		DisableProxySite: true,
		Quiet:            true,
	})

	_, _, err := env.Manager.Prepare()
	if err == nil || !strings.Contains(err.Error(), "authentication required") {
		t.Fatalf("error = %v; want authentication required", err)
	}
	if logins.Load() != 0 {
		t.Fatalf("logins = %d; must not reuse another account's saved password", logins.Load())
	}
}

func TestValidateUserTokenUsesIssuingAppIDAffinity(t *testing.T) {
	var appIDs []string
	env := newTestEnv(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			appID := request.Header.Get("X-App-Id")
			appIDs = append(appIDs, appID)
			if appID == "current-app" {
				writer.WriteHeader(http.StatusUnauthorized)
				_, _ = writer.Write([]byte(`{"message":"Invalid app_id"}`))
				return
			}
			_, _ = writer.Write([]byte(`{"user_auth_token":"token","user":{"id":42}}`))
		}
	}, &config.Account{
		AppID:          "current-app",
		AppSecret:      "current-secret",
		TokenAppID:     "issuing-app",
		TokenAppSecret: "issuing-secret",
	}, Options{
		DisableProxySite: true,
		Quiet:            true,
	})

	client, _, err := env.Manager.PrepareDiagnostics()
	if err != nil {
		t.Fatalf("PrepareDiagnostics() error = %v", err)
	}
	response, err := env.Manager.ValidateUserToken("token")
	if err != nil || response.UserAuthToken != "token" {
		t.Fatalf("response = %#v, error = %v", response, err)
	}
	if client.AppID != "issuing-app" || env.Manager.Client().AppID != "issuing-app" {
		t.Fatalf("client App ID = %q", client.AppID)
	}
	if strings.Join(appIDs, ",") != "current-app,issuing-app" {
		t.Fatalf("App ID attempts = %v", appIDs)
	}
	if env.Manager.State() != StateTokenValid {
		t.Fatalf("state = %q; want token valid", env.Manager.State())
	}
}

func TestPrepareHonorsSaveFlag(t *testing.T) {
	handler := func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
	}
	account := &config.Account{AppID: "app-id", AppSecret: "secret"}
	noSave := newTestEnv(t, handler, account, Options{
		Server:           true,
		DisableProxySite: true,
		Quiet:            true,
		Save:             false,
	})
	if _, _, err := noSave.Manager.Prepare(); err != nil {
		t.Fatalf("Prepare(no save) error = %v", err)
	}
	if noSave.Saves.Load() != 0 {
		t.Fatalf("saves = %d; want 0", noSave.Saves.Load())
	}

	save := newTestEnv(t, handler, account, Options{
		Server:           true,
		DisableProxySite: true,
		Quiet:            true,
		Save:             true,
	})
	if _, _, err := save.Manager.Prepare(); err != nil {
		t.Fatalf("Prepare(save) error = %v", err)
	}
	if save.Saves.Load() != 1 {
		t.Fatalf("saves = %d; want 1", save.Saves.Load())
	}
}

func TestPreparePersistsExplicitAppCredentials(t *testing.T) {
	env := newTestEnv(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
	}, &config.Account{PendingSecrets: []string{"stale"}}, Options{
		AppID:            "custom-app",
		AppSecret:        "custom-secret",
		Server:           true,
		DisableProxySite: true,
		Quiet:            true,
		Save:             true,
	})

	_, account, err := env.Manager.Prepare()
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if account.AppID != "custom-app" || account.AppSecret != "custom-secret" {
		t.Fatalf("saved credentials = %q/%q; want custom pair", account.AppID, account.AppSecret)
	}
	if account.PendingSecrets != nil {
		t.Fatalf("PendingSecrets = %v; want nil", account.PendingSecrets)
	}
	if env.Saves.Load() != 1 {
		t.Fatalf("saves = %d; want 1", env.Saves.Load())
	}
}

func TestPrepareRejectsEmailAndTokenTogether(t *testing.T) {
	env := newTestEnv(t, nil, &config.Account{}, Options{
		Email: "user@example.com",
		Token: "token",
		Quiet: true,
	})

	_, _, err := env.Manager.Prepare()
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v; want mutual exclusion", err)
	}
}

func TestPrepareReportsAccountLoadFailure(t *testing.T) {
	env := newTestEnv(t, nil, nil, Options{Quiet: true})
	env.LoadError = errors.New("malformed account file")

	_, _, err := env.Manager.Prepare()
	if err == nil || !strings.Contains(err.Error(), "load account") {
		t.Fatalf("error = %v; want account load failure", err)
	}
}

func TestPreparePromptsForMissingCredentials(t *testing.T) {
	var loginEmail, loginPassword string
	output := &strings.Builder{}
	env := newTestEnv(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			loginEmail = request.URL.Query().Get("email")
			loginPassword = request.URL.Query().Get("password")
			_, _ = writer.Write([]byte(`{"user_auth_token":"prompted-token","user":{"id":42,"email":"typed@example.com"}}`))
		}
	}, &config.Account{AppID: "app-id", AppSecret: "secret"}, Options{
		AllowPrompt:      true,
		DisableProxySite: true,
		Input:            strings.NewReader("typed@example.com\nprompted-password\n"),
		Output:           output,
		Quiet:            false,
	})

	client, account, err := env.Manager.Prepare()
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if loginEmail != "typed@example.com" || loginPassword != "prompted-password" {
		t.Fatalf("login credentials = %q/%q", loginEmail, loginPassword)
	}
	if client.UserToken != "prompted-token" || account.Email != "typed@example.com" || account.Password != "prompted-password" {
		t.Fatalf("client token = %q, account = %+v", client.UserToken, account)
	}
	if !strings.Contains(output.String(), "Email:") || !strings.Contains(output.String(), "Password:") {
		t.Fatalf("prompt output = %q", output.String())
	}
	if env.Manager.State() != StateAuthenticated {
		t.Fatalf("state = %q; want authenticated", env.Manager.State())
	}
}

func TestPrepareDiagnosticsDoesNotLoginOrSave(t *testing.T) {
	var logins atomic.Int32
	env := newTestEnv(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			logins.Add(1)
			_, _ = writer.Write([]byte(`{"user_auth_token":"token","user":{"id":42}}`))
		}
	}, &config.Account{AppID: "app-id", AppSecret: "secret", UserToken: "saved-token", TokenAppID: "app-id"}, Options{
		DisableProxySite: true,
		Quiet:            true,
		Save:             true,
	})

	client, account, err := env.Manager.PrepareDiagnostics()
	if err != nil {
		t.Fatalf("PrepareDiagnostics() error = %v", err)
	}
	if logins.Load() != 0 || env.Saves.Load() != 0 || client.UserToken != "" || account.UserToken != "saved-token" {
		t.Fatalf("logins/saves/token = %d/%d/%q", logins.Load(), env.Saves.Load(), client.UserToken)
	}
	if env.Manager.State() != StateSecretValid {
		t.Fatalf("state = %q; want secret valid", env.Manager.State())
	}
}
