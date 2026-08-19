package main

import (
	"testing"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/config"
)

func TestShouldRefreshLogin(t *testing.T) {
	acc := &config.Account{Email: "old@example.com", UserToken: "stale-token"}

	if !shouldRefreshLogin("new@example.com", acc) {
		t.Fatal("explicit email should force a fresh login flow")
	}

	if shouldRefreshLogin("", acc) {
		t.Fatal("no email override should not force re-login when token is still valid")
	}
}

