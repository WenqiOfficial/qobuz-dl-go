package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/imroc/req/v3"
)

func TestExtractAppID(t *testing.T) {
	tests := []struct {
		name   string
		bundle string
		want   string
	}{
		{
			name:   "production bundle",
			bundle: `production:{api:{appId:"123456789",appSecret:"0123456789abcdef0123456789abcdef"`,
			want:   "123456789",
		},
		{
			name:   "whitespace and single quotes",
			bundle: `production: { api: { appId: '123456789', appSecret: '0123456789abcdef0123456789abcdef'`,
			want:   "123456789",
		},
		{
			name:   "fallback without decoy constraint",
			bundle: `production:{api:{appId:"12345",appSecret:"changed"`,
			want:   "12345",
		},
		{
			name:   "missing app ID",
			bundle: `production:{api:{appSecret:"secret"`,
			want:   "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := extractAppID(test.bundle); got != test.want {
				t.Fatalf("extractAppID() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFetchSecretCandidatesOrdersAndDeduplicates(t *testing.T) {
	bundle := `production:{api:{appId:"123456789",appSecret:"decoy"` +
		secretFragment("london", "London", "london-secret") +
		secretFragment("berlin", "Berlin", "berlin-secret") +
		secretFragment("abidjan", "Abidjan", "abidjan-secret")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/login":
			_, _ = writer.Write([]byte(`<script src="/assets/bundle.js"></script>`))
		case "/assets/bundle.js":
			_, _ = writer.Write([]byte(bundle))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	appID, candidates, privateKey, err := fetchSecretCandidatesFromHost(req.NewClient(), server.URL)
	if err != nil {
		t.Fatalf("fetchSecretCandidatesFromHost() error = %v", err)
	}
	if appID != "123456789" {
		t.Fatalf("appID = %q", appID)
	}
	if privateKey != "" {
		t.Fatalf("privateKey = %q", privateKey)
	}
	wantTimezones := []string{"berlin", "abidjan", "london"}
	wantSecrets := []string{"berlin-secret", "abidjan-secret", "london-secret"}
	if len(candidates) != len(wantSecrets) {
		t.Fatalf("len(candidates) = %d", len(candidates))
	}
	for index, candidate := range candidates {
		if candidate.Timezone != wantTimezones[index] || candidate.Value != wantSecrets[index] {
			t.Fatalf("candidate[%d] = %+v", index, candidate)
		}
	}

	_, legacySecrets, _, err := fetchSecretsFromHost(req.NewClient(), server.URL)
	if err != nil {
		t.Fatalf("fetchSecretsFromHost() error = %v", err)
	}
	for index, secret := range legacySecrets {
		if secret != wantSecrets[index] {
			t.Fatalf("legacySecrets[%d] = %q", index, secret)
		}
	}
}

func TestFetchSecretCandidatesSkipsInvalidAndDuplicateSecrets(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("same-secret"))
	validCombined := encoded + fmt.Sprintf("%044s", "")
	bundle := `appId:"12345",appSecret:""` +
		`x.initialSeed("bad",window.utimezone.london)` +
		`name:"test/London",info:"",extras:""` +
		`x.initialSeed("bad",window.utimezone.berlin)` +
		`name:"test/Berlin",info:"",extras:""` +
		`x.initialSeed("` + validCombined + `",window.utimezone.abidjan)` +
		`name:"test/Abidjan",info:"",extras:""` +
		`x.initialSeed("` + validCombined + `",window.utimezone.paris)` +
		`name:"test/Paris",info:"",extras:""`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/login" {
			_, _ = writer.Write([]byte(`<script src="bundle.js"></script>`))
			return
		}
		_, _ = writer.Write([]byte(bundle))
	}))
	defer server.Close()

	_, candidates, _, err := fetchSecretCandidatesFromHost(req.NewClient(), server.URL)
	if err != nil {
		t.Fatalf("fetchSecretCandidatesFromHost() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].Timezone != "abidjan" || candidates[0].Value != "same-secret" {
		t.Fatalf("candidates = %+v", candidates)
	}
}

func secretFragment(timezone, capitalizedTimezone, secret string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(secret))
	combined := encoded + fmt.Sprintf("%044s", "")
	return `x.initialSeed("` + combined + `",window.utimezone.` + timezone + `)` +
		`name:"test/` + capitalizedTimezone + `",info:"",extras:""`
}
