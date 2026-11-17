package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/config"
)

func TestAuthDoctorReportsMissingTokenWithoutPromptingOrSaving(t *testing.T) {
	var logins, saves int
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			logins++
			_, _ = writer.Write([]byte(`{"user_auth_token":"unexpected"}`))
		}
	})
	acc := &config.Account{AppID: "app-id", AppSecret: "secret-value", Email: "user@example.com", Password: "saved-password"}
	useAccount(acc)
	saveAccount = func(*config.Account) error {
		saves++
		return nil
	}

	var output bytes.Buffer
	if err := runAuthDoctor(&output, false); err != nil {
		t.Fatalf("runAuthDoctor() error = %v", err)
	}
	if logins != 0 || saves != 0 {
		t.Fatalf("logins/saves = %d/%d; doctor must be read-only", logins, saves)
	}
	text := output.String()
	if !strings.Contains(text, "[MISSING] token") {
		t.Fatalf("output = %q", text)
	}
	if strings.Contains(text, acc.AppSecret) || strings.Contains(text, acc.Password) {
		t.Fatalf("output leaked credentials: %q", text)
	}
}

func TestAuthDoctorReportsAccountLoadFailure(t *testing.T) {
	setupCommandTest(t, nil)
	loadAccount = func() (*config.Account, error) {
		return nil, errors.New("malformed account file")
	}

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
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
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
	})
	acc := &config.Account{
		AppID:          "app-id",
		AppSecret:      "secret-value",
		UserToken:      "saved-token-value",
		TokenAppID:     "app-id",
		TokenAppSecret: "secret-value",
	}
	useAccount(acc)

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
	for _, sensitive := range []string{acc.AppSecret, acc.UserToken, acc.TokenAppSecret} {
		if strings.Contains(output.String(), sensitive) {
			t.Fatalf("JSON output leaked sensitive value %q: %s", sensitive, output.String())
		}
	}
}

func TestAuthDoctorReportsInvalidToken(t *testing.T) {
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"Invalid user auth token"}`))
		}
	})
	acc := &config.Account{
		AppID:          "app-id",
		AppSecret:      "secret-value",
		UserToken:      "expired-token-value",
		TokenAppID:     "app-id",
		TokenAppSecret: "secret-value",
	}
	useAccount(acc)

	var output bytes.Buffer
	err := runAuthDoctor(&output, false)
	if !errors.Is(err, api.ErrTokenInvalid) {
		t.Fatalf("error = %v; want invalid token", err)
	}
	if !strings.Contains(output.String(), "[TOKEN_INVALID] token") {
		t.Fatalf("output = %q", output.String())
	}
	if strings.Contains(output.String(), acc.UserToken) || strings.Contains(output.String(), acc.AppSecret) {
		t.Fatalf("output leaked credentials: %q", output.String())
	}
}

func TestAuthDoctorJSONStaysValidWhenDiagnosisFails(t *testing.T) {
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/track/getFileUrl":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"User authentication is required"}`))
		case "/user/login":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"Invalid user auth token"}`))
		}
	})
	acc := &config.Account{
		AppID:          "app-id",
		AppSecret:      "secret-value",
		UserToken:      "expired-token-value",
		TokenAppID:     "app-id",
		TokenAppSecret: "secret-value",
	}
	useAccount(acc)

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
	var fetches int
	setupCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"message":"Invalid Request Signature"}`))
	})
	fetchSecretCandidates = func(string, bool) (string, []api.SecretCandidate, error) {
		fetches++
		return "bundle-app", []api.SecretCandidate{{Timezone: "berlin", Value: "fresh-secret-value"}}, nil
	}
	acc := &config.Account{AppID: "app-id", AppSecret: "bad-secret-value"}
	useAccount(acc)

	var output bytes.Buffer
	err := runAuthDoctor(&output, false)
	if !errors.Is(err, api.ErrInvalidSecret) {
		t.Fatalf("error = %v; want invalid secret", err)
	}
	if fetches != 1 {
		t.Fatalf("fetches = %d; want one refresh attempt", fetches)
	}
	if !strings.Contains(output.String(), "[INVALID_SECRET] authentication") {
		t.Fatalf("output = %q", output.String())
	}
	if strings.Contains(output.String(), acc.AppSecret) || strings.Contains(output.String(), "fresh-secret-value") {
		t.Fatalf("output leaked app secrets: %q", output.String())
	}
}
