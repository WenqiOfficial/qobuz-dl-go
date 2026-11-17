package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/imroc/req/v3"
)

func newTestClient(serverURL string) *Client {
	client := NewClient("test-app-id", "test-secret")
	client.SetUseProxy(false)
	client.HTTP.SetBaseURL(serverURL)
	client.currentBase = serverURL
	return client
}

func TestValidateSecretDetailedStatusMatrix(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       SecretStatus
		wantKind   ErrorKind
	}{
		{name: "success", statusCode: http.StatusOK, body: `{"url":"https://example.com/track"}`, want: SecretStatusValid},
		{name: "bad signature", statusCode: http.StatusBadRequest, body: `{"status":"error","code":400,"message":"Invalid Request Signature"}`, want: SecretStatusInvalid, wantKind: ErrorKindInvalidSecret},
		{name: "authentication required", statusCode: http.StatusUnauthorized, body: `{"status":"error","code":401,"message":"User authentication is required"}`, want: SecretStatusValid},
		{name: "subscription forbidden", statusCode: http.StatusForbidden, body: `{"message":"Forbidden"}`, want: SecretStatusValid},
		{name: "test track removed", statusCode: http.StatusNotFound, body: `{"message":"Track not found"}`, want: SecretStatusUnknown, wantKind: ErrorKindNotFound},
		{name: "rate limited", statusCode: http.StatusTooManyRequests, body: `{"message":"Too many requests"}`, want: SecretStatusUnknown, wantKind: ErrorKindRateLimited},
		{name: "server error", statusCode: http.StatusBadGateway, body: `bad gateway`, want: SecretStatusUnknown, wantKind: ErrorKindTransient},
		{name: "other bad request", statusCode: http.StatusBadRequest, body: `{"message":"Missing track_id"}`, want: SecretStatusUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/track/getFileUrl" {
					t.Errorf("unexpected path: %s", request.URL.Path)
				}
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(test.statusCode)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()

			status, err := newTestClient(server.URL).ValidateSecretDetailed()
			if status != test.want {
				t.Fatalf("status = %v, want %v (error: %v)", status, test.want, err)
			}
			if test.wantKind != ErrorKindUnknown {
				var statusErr *StatusError
				if !errors.As(err, &statusErr) || statusErr.Kind != test.wantKind {
					t.Fatalf("error = %#v, want kind %q", err, test.wantKind)
				}
			}
		})
	}
}

func TestValidateSecretDetailedTransportErrorIsUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()

	status, err := newTestClient(server.URL).ValidateSecretDetailed()
	if status != SecretStatusUnknown || err == nil {
		t.Fatalf("status = %v, error = %v; want unknown transport failure", status, err)
	}
}

func TestValidateSecretDetailedRejectedAppIDIsUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"Invalid app_id"}`))
	}))
	defer server.Close()

	status, err := newTestClient(server.URL).ValidateSecretDetailed()
	if status != SecretStatusUnknown || !errors.Is(err, ErrAppIDRejected) {
		t.Fatalf("status = %v, error = %v; want unknown App ID rejection", status, err)
	}
}

func TestFindValidSecretStopsAfterInconclusiveResult(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.FindValidSecret([]string{"first", "second"})
	if err == nil || requests.Load() != 1 {
		t.Fatalf("error = %v, requests = %d; want one inconclusive request", err, requests.Load())
	}
}

func TestLoginRejectsNonJSONAndEmptyToken(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{name: "html", contentType: "text/html", body: `<html><body>proxy error</body></html>`},
		{name: "json html", contentType: "application/json", body: `<html><body>proxy error</body></html>`},
		{name: "empty token", contentType: "application/json", body: `{"user_auth_token":"","user":{"id":1}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", test.contentType)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()

			client := newTestClient(server.URL)
			result, err := client.Login("user@example.com", "password")
			if result != nil || !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("result = %#v, error = %v; want invalid response", result, err)
			}
			if client.UserToken != "" {
				t.Fatalf("token = %q; want empty", client.UserToken)
			}
		})
	}
}

func TestLoginWithTokenAndClearsInvalidToken(t *testing.T) {
	var queryToken string
	var requestHeader string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		queryToken = request.URL.Query().Get("user_auth_token")
		requestHeader = request.Header.Get("X-User-Auth-Token")
		writer.Header().Set("Content-Type", "application/json")
		if queryToken == "expired" {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message":"Invalid user auth token"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"user_auth_token":"fresh-token","user":{"id":42,"email":"user@example.com"}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := client.LoginWithToken("saved-token")
	if err != nil || result.UserAuthToken != "fresh-token" || client.UserToken != "fresh-token" {
		t.Fatalf("result = %#v, error = %v, token = %q", result, err, client.UserToken)
	}
	if queryToken != "saved-token" || requestHeader != "" {
		t.Fatalf("query token = %q, header = %q; stale token must not be sent", queryToken, requestHeader)
	}

	_, err = client.LoginWithToken("expired")
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("error = %v; want ErrTokenInvalid", err)
	}
	if client.UserToken != "" || client.HTTP.Headers.Get("X-User-Auth-Token") != "" {
		t.Fatalf("token/header = %q/%q; want cleared", client.UserToken, client.HTTP.Headers.Get("X-User-Auth-Token"))
	}
}

func TestLoginWithTokenTreatsUnlabelledUnauthorizedAsTokenInvalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.LoginWithToken("expired")
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("error = %v; want ErrTokenInvalid", err)
	}
	if client.UserToken != "" {
		t.Fatalf("token = %q; want cleared", client.UserToken)
	}
}

func TestLoginFallbackOnlyForTransportAndServerErrors(t *testing.T) {
	var proxyRequests atomic.Int32
	var directRequests atomic.Int32
	direct := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		directRequests.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"user_auth_token":"direct-token","user":{"id":1}}`))
	}))
	defer direct.Close()

	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		proxyRequests.Add(1)
		writer.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()

	client := newTestClient(proxy.URL)
	client.UseProxy = true
	client.currentBase = BaseURLProxy
	client.SetDirectBaseURL(direct.URL)
	result, err := client.Login("user@example.com", "password")
	if err != nil || result.UserAuthToken != "direct-token" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	if proxyRequests.Load() != 1 || directRequests.Load() != 1 {
		t.Fatalf("proxy/direct requests = %d/%d; want 1/1", proxyRequests.Load(), directRequests.Load())
	}
	if client.GetCurrentBaseURL() != BaseURLProxy || client.HTTP.BaseURL != proxy.URL {
		t.Fatalf("client base changed: current=%q HTTP=%q", client.GetCurrentBaseURL(), client.HTTP.BaseURL)
	}
}

func TestLoginDoesNotRetryRejectedCredentials(t *testing.T) {
	for _, statusCode := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests} {
		t.Run(fmt.Sprintf("status_%d", statusCode), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requests.Add(1)
				writer.WriteHeader(statusCode)
			}))
			defer server.Close()

			client := newTestClient(server.URL)
			client.UseProxy = true
			client.currentBase = BaseURLProxy
			client.SetDirectBaseURL(server.URL)
			_, err := client.Login("user@example.com", "wrong-password")
			if err == nil || requests.Load() != 1 {
				t.Fatalf("error = %v, requests = %d; want one rejected request", err, requests.Load())
			}
		})
	}
}

func TestQualityFallbackStopsOnAuthAndRateLimitErrors(t *testing.T) {
	for _, statusCode := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests} {
		t.Run(fmt.Sprintf("status_%d", statusCode), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requests.Add(1)
				writer.WriteHeader(statusCode)
			}))
			defer server.Close()

			_, _, err := newTestClient(server.URL).GetTrackURLWithFallback("123", 27)
			if err == nil || requests.Load() != 1 {
				t.Fatalf("error = %v, requests = %d; want no quality retries", err, requests.Load())
			}
		})
	}
}

func TestQualityFallbackStopsOnCredentialErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid secret", body: `{"message":"Invalid Request Signature"}`},
		{name: "invalid app id", body: `{"message":"Invalid app_id"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requests.Add(1)
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()

			_, _, err := newTestClient(server.URL).GetTrackURLWithFallback("123", 27)
			if err == nil || requests.Load() != 1 {
				t.Fatalf("error = %v, requests = %d; want one credential failure", err, requests.Load())
			}
		})
	}
}

func TestQualityFallbackStillTriesLowerQuality(t *testing.T) {
	var formats []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		formats = append(formats, request.URL.Query().Get("format_id"))
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("format_id") == "27" {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"message":"Requested quality unavailable"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"url":"https://example.com/track"}`))
	}))
	defer server.Close()

	_, quality, err := newTestClient(server.URL).GetTrackURLWithFallback("123", 27)
	if err != nil || quality != 7 || strings.Join(formats, ",") != "27,7" {
		t.Fatalf("quality = %d, formats = %v, error = %v", quality, formats, err)
	}
}

func TestAPIErrorsExposeEndpointAndStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte(`{"message":"Track not found"}`))
	}))
	defer server.Close()

	_, err := newTestClient(server.URL).GetTrack("missing")
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != http.StatusNotFound || statusErr.Endpoint != "track/get" || statusErr.Message != "Track not found" {
		t.Fatalf("error = %#v; want structured not-found error", err)
	}
}

func TestSetUserTokenEmptyRemovesHeader(t *testing.T) {
	client := NewClient("app-id", "secret")
	client.SetUserToken("token")
	client.SetUserToken("")
	if client.HTTP.Headers.Get("X-User-Auth-Token") != "" {
		t.Fatal("empty token must remove the authentication header")
	}
}

func TestResponseErrorDoesNotEchoArbitraryResponseBody(t *testing.T) {
	response := &req.Response{Response: &http.Response{StatusCode: http.StatusBadRequest}}
	response.SetBodyString(`{"password":"must-not-leak"}`)
	err := responseError(response, "user/login")
	if strings.Contains(err.Error(), "must-not-leak") {
		t.Fatalf("error leaked response body: %v", err)
	}
}

func TestResponseErrorClassification(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		body     string
		endpoint string
		kind     ErrorKind
		sentinel error
	}{
		{name: "bad signature", code: 400, body: `{"message":"Invalid Request Signature"}`, endpoint: "track/getFileUrl", kind: ErrorKindInvalidSecret, sentinel: ErrInvalidSecret},
		{name: "bad app id", code: 400, body: `{"message":"Invalid app_id"}`, endpoint: "user/login", kind: ErrorKindAppIDRejected, sentinel: ErrAppIDRejected},
		{name: "password", code: 401, body: `{"message":"Invalid email/password combination"}`, endpoint: "user/login", kind: ErrorKindBadCredentials, sentinel: ErrBadCredentials},
		{name: "token", code: 401, body: `{"message":"Invalid user auth token"}`, endpoint: "user/login", kind: ErrorKindTokenInvalid, sentinel: ErrTokenInvalid},
		{name: "forbidden", code: 403, body: `{"message":"Subscription required"}`, endpoint: "track/getFileUrl", kind: ErrorKindForbidden, sentinel: ErrForbidden},
		{name: "not found", code: 404, body: `{"message":"Track not found"}`, endpoint: "track/get", kind: ErrorKindNotFound, sentinel: ErrNotFound},
		{name: "rate limited", code: 429, body: `{"message":"Slow down"}`, endpoint: "track/get", kind: ErrorKindRateLimited, sentinel: ErrRateLimited},
		{name: "server", code: 503, body: `unavailable`, endpoint: "track/get", kind: ErrorKindTransient, sentinel: ErrTransient},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := &req.Response{Response: &http.Response{StatusCode: test.code}}
			response.SetBodyString(test.body)
			err := responseError(response, test.endpoint)
			var statusErr *StatusError
			if !errors.As(err, &statusErr) || statusErr.Kind != test.kind || !errors.Is(err, test.sentinel) {
				t.Fatalf("error = %#v; want kind %q and sentinel %v", err, test.kind, test.sentinel)
			}
		})
	}
}
