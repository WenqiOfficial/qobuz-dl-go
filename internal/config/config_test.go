package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAccountObfuscationRoundTrip(t *testing.T) {
	original := &Account{
		Email:          "user@example.com",
		Password:       "p@ssw0rd 密码",
		UserToken:      "user-token",
		TokenAppID:     "123456789",
		TokenAppSecret: "token-app-secret",
		AppID:          "123456789",
		AppSecret:      "app-secret",
		UserID:         42,
	}

	data, err := marshalAccount(original)
	if err != nil {
		t.Fatalf("marshal account: %v", err)
	}
	for _, plaintext := range []string{original.Password, original.UserToken, original.AppSecret, original.TokenAppSecret} {
		if strings.Contains(string(data), plaintext) {
			t.Fatalf("stored account contains plaintext credential %q", plaintext)
		}
	}

	var stored map[string]any
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("decode stored JSON: %v", err)
	}
	for _, field := range []string{"password", "user_auth_token", "app_secret", "token_app_secret"} {
		value, ok := stored[field].(string)
		if !ok || !strings.HasPrefix(value, obfuscationPrefix) {
			t.Fatalf("stored field %q is not obfuscated: %v", field, stored[field])
		}
	}

	loaded, err := unmarshalAccount(data)
	if err != nil {
		t.Fatalf("unmarshal account: %v", err)
	}
	if loaded.Email != original.Email || loaded.Password != original.Password ||
		loaded.UserToken != original.UserToken || loaded.AppID != original.AppID ||
		loaded.AppSecret != original.AppSecret || loaded.UserID != original.UserID ||
		loaded.TokenAppID != original.TokenAppID || loaded.TokenAppSecret != original.TokenAppSecret {
		t.Fatalf("round trip mismatch: got %+v, want %+v", loaded, original)
	}
	if original.Password != "p@ssw0rd 密码" || original.UserToken != "user-token" || original.AppSecret != "app-secret" {
		t.Fatal("marshaling mutated the in-memory account")
	}
}

func TestAccountAcceptsLegacyPlaintext(t *testing.T) {
	data := []byte(`{"email":"user@example.com","password":"legacy-password","user_auth_token":"legacy-token","app_id":"123456789","app_secret":"legacy-secret"}`)
	loaded, err := unmarshalAccount(data)
	if err != nil {
		t.Fatalf("unmarshal legacy account: %v", err)
	}
	if loaded.Password != "legacy-password" || loaded.UserToken != "legacy-token" || loaded.AppSecret != "legacy-secret" {
		t.Fatalf("legacy credentials changed: %+v", loaded)
	}
}

func TestAccountRejectsMalformedObfuscation(t *testing.T) {
	for _, field := range []string{"password", "user_auth_token", "app_secret", "token_app_secret"} {
		data := []byte(`{"` + field + `":"obf:v1:not-base64!"}`)
		if _, err := unmarshalAccount(data); err == nil {
			t.Fatalf("expected malformed %s to fail", field)
		}
	}
}

func TestAccountEmptyCredentialsRemainEmpty(t *testing.T) {
	data, err := marshalAccount(&Account{})
	if err != nil {
		t.Fatalf("marshal empty account: %v", err)
	}
	if strings.Contains(string(data), obfuscationPrefix) {
		t.Fatal("empty credentials should not be obfuscated")
	}
	loaded, err := unmarshalAccount(data)
	if err != nil {
		t.Fatalf("unmarshal empty account: %v", err)
	}
	if loaded.Password != "" || loaded.UserToken != "" || loaded.AppSecret != "" || loaded.TokenAppSecret != "" {
		t.Fatalf("empty credentials changed: %+v", loaded)
	}
}
