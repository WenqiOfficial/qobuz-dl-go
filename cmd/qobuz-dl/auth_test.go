package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/auth"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/config"
)

type authDoctorTestEnv struct {
	Server    *httptest.Server
	Saves     atomic.Int32
	Fetches   atomic.Int32
	LoadError error
	FetchFunc func() (string, []api.SecretCandidate, error)
}

func setupAuthDoctorTest(t *testing.T, handler http.HandlerFunc, account *config.Account) *authDoctorTestEnv {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	env := &authDoctorTestEnv{Server: server}
	oldFactory := newAuthManager
	oldAppID, oldAppSecret := flagAppID, flagAppSecret
	oldEmail, oldPassword, oldToken := flagEmail, flagPassword, flagToken
	oldProxy, oldNoCDN, oldNoSave := flagProxy, flagNoCDN, flagNoSave
	t.Cleanup(func() {
		newAuthManager = oldFactory
		flagAppID, flagAppSecret = oldAppID, oldAppSecret
		flagEmail, flagPassword, flagToken = oldEmail, oldPassword, oldToken
		flagProxy, flagNoCDN, flagNoSave = oldProxy, oldNoCDN, oldNoSave
	})
	flagAppID, flagAppSecret = "", ""
	flagEmail, flagPassword, flagToken = "", "", ""
	flagProxy, flagNoCDN = "", true
	flagNoSave = true

	newAuthManager = func(options auth.Options) *auth.Manager {
		options.Input = strings.NewReader("")
		options.Output = io.Discard
		deps := auth.Dependencies{
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
			FetchSecretCandidates: func(string, bool) (string, []api.SecretCandidate, error) {
				env.Fetches.Add(1)
				if env.FetchFunc != nil {
					return env.FetchFunc()
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
		return auth.NewManagerWithDependencies(options, deps)
	}
	return env
}

func TestAuthDoctorReportsMissingTokenWithoutPromptingOrSaving(t *testing.T) {
	var logins atomic.Int32
	env := setupAuthDoctorTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			logins.Add(1)
			_, _ = writer.Write([]byte(`{"user_auth_token":"unexpected"}`))
		}
	}, &config.Account{AppID: "app-id", AppSecret: "secret-value", Email: "user@example.com", Password: "saved-password"})

	var output bytes.Buffer
	if err := runAuthDoctor(&output, false); err != nil {
		t.Fatalf("runAuthDoctor() error = %v", err)
	}
	if logins.Load() != 0 || env.Saves.Load() != 0 {
		t.Fatalf("logins/saves = %d/%d; doctor must be read-only", logins.Load(), env.Saves.Load())
	}
	text := output.String()
	if !strings.Contains(text, "[MISSING] token") {
		t.Fatalf("output = %q", text)
	}
	if strings.Contains(text, "secret-value") || strings.Contains(text, "saved-password") {
		t.Fatalf("output leaked credentials: %q", text)
	}
}

func TestAuthDoctorReportsAccountLoadFailure(t *testing.T) {
	env := setupAuthDoctorTest(t, nil, nil)
	env.LoadError = errors.New("malformed account file")

	var output bytes.Buffer
	err := runAuthDoctor(&output, false)
	if err == nil || !strings.Contains(err.Error(), "malformed account file") {
		t.Fatalf("error = %v; want account load failure", err)
	}
	if !strings.Contains(output.String(), "[ERROR] account") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestAuthDoctorJSONReportsValidTokenWithoutSecrets(t *testing.T) {
	env := setupAuthDoctorTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			if request.URL.Query().Get("user_auth_token") != "saved-token-value" {
				t.Errorf("unexpected token query: %q", request.URL.Query().Get("user_auth_token"))
			}
			_, _ = writer.Write([]byte(`{"user_auth_token":"saved-token-value","user":{"id":42}}`))
		}
	}, &config.Account{
		AppID:          "app-id",
		AppSecret:      "secret-value",
		UserToken:      "saved-token-value",
		TokenAppID:     "app-id",
		TokenAppSecret: "secret-value",
	})

	var output bytes.Buffer
	if err := runAuthDoctor(&output, true); err != nil {
		t.Fatalf("runAuthDoctor() error = %v", err)
	}
	var reports []authDiagnostic
	if err := json.Unmarshal(output.Bytes(), &reports); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, output.String())
	}
	statuses := make(map[string]string)
	for _, report := range reports {
		statuses[report.Step] = report.Status
	}
	if statuses["account"] != "ok" || statuses["app_id"] != "ok" || statuses["secret"] != "ok" || statuses["token"] != "ok" {
		t.Fatalf("statuses = %#v", statuses)
	}
	for _, sensitive := range []string{"secret-value", "saved-token-value"} {
		if strings.Contains(output.String(), sensitive) {
			t.Fatalf("JSON output leaked sensitive value %q: %s", sensitive, output.String())
		}
	}
	if env.Fetches.Load() != 0 || env.Saves.Load() != 0 {
		t.Fatalf("fetches/saves = %d/%d; doctor must not refresh or save", env.Fetches.Load(), env.Saves.Load())
	}
}

func TestAuthDoctorReportsInvalidToken(t *testing.T) {
	setupAuthDoctorTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"Invalid user auth token"}`))
		}
	}, &config.Account{
		AppID:          "app-id",
		AppSecret:      "secret-value",
		UserToken:      "expired-token-value",
		TokenAppID:     "app-id",
		TokenAppSecret: "secret-value",
	})

	var output bytes.Buffer
	err := runAuthDoctor(&output, false)
	if !errors.Is(err, api.ErrTokenInvalid) {
		t.Fatalf("error = %v; want invalid token", err)
	}
	if !strings.Contains(output.String(), "[TOKEN_INVALID] token") {
		t.Fatalf("output = %q", output.String())
	}
	if strings.Contains(output.String(), "expired-token-value") || strings.Contains(output.String(), "secret-value") {
		t.Fatalf("output leaked credentials: %q", output.String())
	}
}

func TestAuthDoctorJSONStaysValidWhenDiagnosisFails(t *testing.T) {
	setupAuthDoctorTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"Invalid user auth token"}`))
		}
	}, &config.Account{
		AppID:          "app-id",
		AppSecret:      "secret-value",
		UserToken:      "expired-token-value",
		TokenAppID:     "app-id",
		TokenAppSecret: "secret-value",
	})

	var output bytes.Buffer
	err := runAuthDoctor(&output, true)
	if !errors.Is(err, api.ErrTokenInvalid) {
		t.Fatalf("error = %v; want invalid token", err)
	}
	var reports []authDiagnostic
	if err := json.Unmarshal(output.Bytes(), &reports); err != nil {
		t.Fatalf("JSON output was not valid: %v\n%s", err, output.String())
	}
}

func TestAuthDoctorReportsInvalidSecretWithoutLeakingIt(t *testing.T) {
	env := setupAuthDoctorTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"message":"Invalid Request Signature"}`))
	}, &config.Account{AppID: "app-id", AppSecret: "bad-secret-value"})
	env.FetchFunc = func() (string, []api.SecretCandidate, error) {
		return "bundle-app", []api.SecretCandidate{{Timezone: "berlin", Value: "fresh-secret-value"}}, nil
	}

	var output bytes.Buffer
	err := runAuthDoctor(&output, false)
	if !errors.Is(err, api.ErrInvalidSecret) {
		t.Fatalf("error = %v; want invalid secret", err)
	}
	if env.Fetches.Load() != 1 {
		t.Fatalf("fetches = %d; want one refresh attempt", env.Fetches.Load())
	}
	if !strings.Contains(output.String(), "[INVALID_SECRET] authentication") {
		t.Fatalf("output = %q", output.String())
	}
	if strings.Contains(output.String(), "bad-secret-value") || strings.Contains(output.String(), "fresh-secret-value") {
		t.Fatalf("output leaked app secrets: %q", output.String())
	}
}
