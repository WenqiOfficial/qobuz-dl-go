// Package auth owns Qobuz app credentials, user tokens, and login state.
package auth

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/config"
)

const secretValidationRetryDelay = 500 * time.Millisecond

// State describes the furthest authentication stage reached by a Manager.
type State string

const (
	StateNew             State = "new"
	StateAccountLoaded   State = "account_loaded"
	StateAppCredentials  State = "app_credentials_ready"
	StateSecretValid     State = "secret_valid"
	StateTokenValid      State = "token_valid"
	StateAuthenticated   State = "authenticated"
	StateUnauthenticated State = "unauthenticated"
)

// Options contains command-line values and runtime behavior for authentication.
type Options struct {
	AppID            string
	AppSecret        string
	Email            string
	Password         string
	Token            string
	ProxyURL         string
	DisableProxySite bool
	Server           bool
	AllowPrompt      bool
	Quiet            bool
	Save             bool
	Input            io.Reader
	Output           io.Writer
}

// Dependencies isolates disk, network, and timing side effects for tests.
type Dependencies struct {
	LoadAccount           func() (*config.Account, error)
	SaveAccount           func(*config.Account) error
	FetchSecretCandidates func(string, bool) (string, []api.SecretCandidate, error)
	NewClient             func(appID, appSecret string) (*api.Client, error)
	Sleep                 func(time.Duration)
}

// Manager performs credential preparation and user authentication.
type Manager struct {
	options Options
	deps    Dependencies
	account *config.Account
	client  *api.Client
	state   State
}

// NewManager creates a Manager using production dependencies.
func NewManager(options Options) *Manager {
	return NewManagerWithDependencies(options, Dependencies{})
}

// NewManagerWithDependencies creates a Manager with injectable side effects.
func NewManagerWithDependencies(options Options, deps Dependencies) *Manager {
	if options.Input == nil {
		options.Input = os.Stdin
	}
	if options.Output == nil {
		options.Output = os.Stdout
	}
	defaults := defaultDependencies(options)
	if deps.LoadAccount == nil {
		deps.LoadAccount = defaults.LoadAccount
	}
	if deps.SaveAccount == nil {
		deps.SaveAccount = defaults.SaveAccount
	}
	if deps.FetchSecretCandidates == nil {
		deps.FetchSecretCandidates = defaults.FetchSecretCandidates
	}
	if deps.NewClient == nil {
		deps.NewClient = defaults.NewClient
	}
	if deps.Sleep == nil {
		deps.Sleep = defaults.Sleep
	}
	return &Manager{
		options: options,
		deps:    deps,
		state:   StateNew,
	}
}

func defaultDependencies(options Options) Dependencies {
	return Dependencies{
		LoadAccount: config.LoadAccount,
		SaveAccount: config.SaveAccount,
		FetchSecretCandidates: func(proxyURL string, useProxySite bool) (string, []api.SecretCandidate, error) {
			return api.FetchSecretCandidates(proxyURL, useProxySite)
		},
		NewClient: func(appID, appSecret string) (*api.Client, error) {
			client := api.NewClient(appID, appSecret)
			client.SetUseProxy(!options.DisableProxySite)
			if options.ProxyURL != "" {
				if err := client.SetProxy(options.ProxyURL); err != nil {
					return nil, fmt.Errorf("configure proxy: %w", err)
				}
			}
			return client, nil
		},
		Sleep: time.Sleep,
	}
}

// State returns the farthest successful stage reached by the manager.
func (m *Manager) State() State {
	return m.state
}

// Account returns the current account after a preparation step.
func (m *Manager) Account() *config.Account {
	return m.account
}

// Client returns the current API client after a preparation step.
func (m *Manager) Client() *api.Client {
	return m.client
}

// LoadAccount loads the persisted account once, if it has not been loaded yet.
func (m *Manager) LoadAccount() error {
	return m.loadAccount()
}

// Prepare runs the full credential, token, and login sequence.
func (m *Manager) Prepare() (*api.Client, *config.Account, error) {
	if err := m.validateOptions(); err != nil {
		return nil, nil, err
	}
	if err := m.loadAccount(); err != nil {
		return nil, nil, fmt.Errorf("load account %s: %w", config.GetAccountPath(), err)
	}
	if _, err := m.prepareAppCredentials(); err != nil {
		return nil, nil, err
	}
	if err := m.ensureUserAuthentication(); err != nil {
		return nil, nil, err
	}
	m.syncAccountCredentials()
	if m.options.Save {
		if err := m.deps.SaveAccount(m.account); err != nil {
			return nil, nil, fmt.Errorf("save account credentials: %w", err)
		}
	}
	return m.client, m.account, nil
}

// PrepareDiagnostics prepares app credentials without logging in or saving.
func (m *Manager) PrepareDiagnostics() (*api.Client, *config.Account, error) {
	if err := m.validateOptions(); err != nil {
		return nil, nil, err
	}
	if err := m.loadAccount(); err != nil {
		return nil, nil, fmt.Errorf("load account %s: %w", config.GetAccountPath(), err)
	}
	if _, err := m.prepareAppCredentials(); err != nil {
		return nil, nil, err
	}
	return m.client, m.account, nil
}

// ValidateUserToken validates a token against the prepared app credentials.
func (m *Manager) ValidateUserToken(token string) (*api.LoginResponse, error) {
	if m.client == nil || m.account == nil {
		return nil, fmt.Errorf("app credentials are not prepared")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("user token is empty")
	}
	response, err := m.validateTokenWithAppAffinity(token)
	if err == nil {
		m.state = StateTokenValid
	}
	return response, err
}

func (m *Manager) syncAccountCredentials() {
	if m.client == nil || m.account == nil {
		return
	}
	m.account.AppID = m.client.AppID
	m.account.AppSecret = m.client.AppSecret
	m.account.PendingSecrets = nil
}

func (m *Manager) validateOptions() error {
	if strings.TrimSpace(m.options.Email) != "" && strings.TrimSpace(m.options.Token) != "" {
		return fmt.Errorf("--email and --token are mutually exclusive; use --email to force a fresh password login")
	}
	return nil
}

func (m *Manager) loadAccount() error {
	if m.account != nil {
		return nil
	}
	account, err := m.deps.LoadAccount()
	if err != nil {
		return err
	}
	if account == nil {
		account = &config.Account{}
	}
	m.account = account
	m.state = StateAccountLoaded
	return nil
}

func (m *Manager) prepareAppCredentials() (*api.Client, error) {
	appID := strings.TrimSpace(m.options.AppID)
	appSecret := strings.TrimSpace(m.options.AppSecret)
	pinnedAppID := appID
	explicitSecret := appSecret != ""
	if appID == "" {
		appID = m.account.AppID
	}
	if appSecret == "" && m.account.AppID == appID {
		appSecret = m.account.AppSecret
	}
	if explicitSecret && appID == "" {
		return nil, fmt.Errorf("--app-secret requires --app-id or a saved App ID")
	}
	if appID == "" {
		return m.refreshAppCredentials("")
	}

	client, err := m.newClient(appID, appSecret)
	if err != nil {
		return nil, err
	}
	m.client = client
	m.state = StateAppCredentials

	status, validationErr := m.validateSecretWithRetry(client)
	if status == api.SecretStatusValid {
		m.state = StateSecretValid
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
	return m.refreshAppCredentials(pinnedAppID)
}

func (m *Manager) refreshAppCredentials(pinnedAppID string) (*api.Client, error) {
	m.print("App credentials are missing or invalid. Fetching fresh credentials...")
	fetchedID, candidates, fetchErr := m.deps.FetchSecretCandidates(m.options.ProxyURL, !m.options.DisableProxySite)
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
	if targetAppID != m.account.AppID {
		m.clearToken()
	}

	client, err := m.newClient(targetAppID, "")
	if err != nil {
		return nil, err
	}
	validSecret, err := client.FindValidSecret(secretCandidateValues(candidates))
	if err != nil {
		return nil, fmt.Errorf("validate refreshed app credentials: %w", err)
	}
	client.AppSecret = validSecret
	m.account.AppID = targetAppID
	m.account.AppSecret = validSecret
	m.client = client
	m.state = StateSecretValid
	return client, nil
}

func (m *Manager) validateSecretWithRetry(client *api.Client) (api.SecretStatus, error) {
	status, err := client.ValidateSecretDetailed()
	if status != api.SecretStatusUnknown {
		return status, err
	}
	m.deps.Sleep(secretValidationRetryDelay)
	return client.ValidateSecretDetailed()
}

func (m *Manager) ensureUserAuthentication() error {
	forcePasswordLogin := strings.TrimSpace(m.options.Email) != ""
	if forcePasswordLogin {
		m.clearToken()
	}

	userToken := strings.TrimSpace(m.options.Token)
	if userToken == "" && !forcePasswordLogin {
		userToken = m.account.UserToken
	}

	if userToken != "" && !forcePasswordLogin {
		m.print("Validating saved user token...")
		response, tokenErr := m.validateTokenWithAppAffinity(userToken)
		if tokenErr == nil {
			if m.options.Password != "" {
				m.account.Password = m.options.Password
			}
			m.updateAccountFromLogin(response)
			m.state = StateTokenValid
			return nil
		}
		if errors.Is(tokenErr, api.ErrTokenInvalid) {
			m.print("Saved user token is invalid or expired. Signing in again...")
			m.clearToken()
		} else {
			return fmt.Errorf("validate user token: %w", tokenErr)
		}
	}

	if m.client.UserToken == "" {
		email, password := m.resolveLoginCredentials()
		hasCredentials := email != "" && password != ""
		if m.options.Server && !hasCredentials && !m.options.AllowPrompt &&
			strings.TrimSpace(m.options.Email) == "" && strings.TrimSpace(m.options.Password) == "" {
			m.print("Warning: Starting server without user authentication. Some features may fail.")
			m.state = StateUnauthenticated
			return nil
		}
		response, loginErr := m.loginWithPassword(email, password)
		if loginErr != nil {
			return loginErr
		}
		m.updateAccountFromLogin(response)
		m.state = StateAuthenticated
	}

	if m.client.UserToken == "" {
		m.clearToken()
		m.state = StateUnauthenticated
	}
	return nil
}

func (m *Manager) validateTokenWithAppAffinity(token string) (*api.LoginResponse, error) {
	response, err := m.client.LoginWithToken(token)
	explicitAppID := strings.TrimSpace(m.options.AppID) != ""
	if !errors.Is(err, api.ErrAppIDRejected) || explicitAppID ||
		m.account.TokenAppID == "" || m.account.TokenAppID == m.client.AppID {
		return response, err
	}
	if m.account.TokenAppSecret == "" {
		return nil, fmt.Errorf("token was rejected by App ID %s and no matching saved secret exists for its issuing App ID %s: %w", m.client.AppID, m.account.TokenAppID, err)
	}

	affinityClient, clientErr := m.newClient(m.account.TokenAppID, m.account.TokenAppSecret)
	if clientErr != nil {
		return nil, clientErr
	}
	response, affinityErr := affinityClient.LoginWithToken(token)
	if affinityErr != nil {
		return nil, fmt.Errorf("token was rejected by App ID %s and its issuing App ID %s: %w", m.client.AppID, m.account.TokenAppID, affinityErr)
	}
	*m.client = *affinityClient
	return response, nil
}

func (m *Manager) resolveLoginCredentials() (string, string) {
	email := strings.TrimSpace(m.options.Email)
	if email == "" {
		email = m.account.Email
	}
	password := m.options.Password
	if password == "" && (strings.TrimSpace(m.options.Email) == "" ||
		strings.EqualFold(strings.TrimSpace(m.options.Email), strings.TrimSpace(m.account.Email))) {
		password = m.account.Password
	}
	return email, password
}

func (m *Manager) loginWithPassword(email, password string) (*api.LoginResponse, error) {
	if email == "" || password == "" {
		if !m.options.AllowPrompt {
			return nil, fmt.Errorf("authentication required: provide --token or --email/--password")
		}
		var promptErr error
		email, password, promptErr = m.promptCredentials(email, password)
		if promptErr != nil {
			return nil, promptErr
		}
	}
	if email == "" || password == "" {
		return nil, fmt.Errorf("authentication required: provide --token or --email/--password")
	}

	m.account.Password = password
	if strings.TrimSpace(m.options.Email) == "" {
		m.account.Email = email
	}
	m.print("Logging in...")
	response, err := m.client.Login(email, password)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}
	return response, nil
}

func (m *Manager) promptCredentials(email, password string) (string, string, error) {
	m.print("Authentication required.")
	reader := bufio.NewReader(m.options.Input)
	if email == "" {
		fmt.Fprint(m.options.Output, "Email: ")
		value, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(value) == "" {
			return email, password, fmt.Errorf("read email: %w", err)
		}
		email = strings.TrimSpace(value)
	}
	if password == "" {
		value, err := m.readSecret(reader, "Password: ")
		if err != nil {
			return email, password, fmt.Errorf("read password: %w", err)
		}
		password = value
	}
	return email, password, nil
}

func (m *Manager) readSecret(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Fprint(m.options.Output, prompt)
	if file, ok := m.options.Input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		secret, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(m.options.Output)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(secret)), nil
	}
	value, err := reader.ReadString('\n')
	if err != nil && strings.TrimSpace(value) == "" {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (m *Manager) updateAccountFromLogin(response *api.LoginResponse) {
	if response == nil || m.account == nil {
		return
	}
	if strings.TrimSpace(m.options.Email) != "" {
		m.account.Email = strings.TrimSpace(m.options.Email)
	}
	if response.User.Email != "" {
		m.account.Email = response.User.Email
	}
	if m.options.Password != "" {
		m.account.Password = m.options.Password
	}
	m.account.UserToken = response.UserAuthToken
	if m.client == nil {
		m.account.TokenAppID = ""
		m.account.TokenAppSecret = ""
	} else {
		m.account.TokenAppID = m.client.AppID
		m.account.TokenAppSecret = m.client.AppSecret
	}
	m.account.UserID = response.User.ID
}

func (m *Manager) clearToken() {
	if m.account != nil {
		m.account.UserToken = ""
		m.account.TokenAppID = ""
		m.account.TokenAppSecret = ""
	}
	if m.client != nil {
		m.client.SetUserToken("")
	}
}

func (m *Manager) newClient(appID, appSecret string) (*api.Client, error) {
	client, err := m.deps.NewClient(appID, appSecret)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("client factory returned nil")
	}
	return client, nil
}

func (m *Manager) print(message string) {
	if !m.options.Quiet {
		fmt.Fprintln(m.options.Output, message)
	}
}

func secretCandidateValues(candidates []api.SecretCandidate) []string {
	values := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		values = append(values, candidate.Value)
	}
	return values
}
