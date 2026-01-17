// Package config handles application configuration and account persistence.
// It provides functions to load and save configuration files.
package config

import (
	"encoding/json"
	"os"
)

// Config holds application-level settings.
type Config struct {
	Output  string `json:"output"`   // Default output directory
	Proxy   string `json:"proxy"`    // Proxy URL (http/https/socks5)
	Quality int    `json:"quality"`  // Audio quality: 5=MP3, 6=FLAC 16bit, 7=FLAC 24bit, 27=Hi-Res
	NoSave  bool   `json:"nosave"`   // If true, don't save credentials
	OgCover bool   `json:"og_cover"` // If true, download original quality cover
}

// Account holds user authentication credentials.
type Account struct {
	Email     string `json:"email"`
	Password  string `json:"password"` // Note: stored in plaintext, consider encrypting
	UserToken string `json:"user_auth_token"`
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
	UserID    int    `json:"user_id"`
}

// GetConfigPath returns the path to the configuration file.
func GetConfigPath() string {
	return "config.json"
}

// GetAccountPath returns the path to the account credentials file.
func GetAccountPath() string {
	return "account.json"
}

// LoadConfig loads the configuration from disk.
// Returns default values if the config file doesn't exist.
func LoadConfig() (*Config, error) {
	path := GetConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Config{Quality: 6, Output: "."}, nil // Defaults
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadAccount loads saved account credentials from disk.
// Returns an empty Account if the file doesn't exist.
func LoadAccount() (*Account, error) {
	path := GetAccountPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Account{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var acc Account
	if err := json.Unmarshal(data, &acc); err != nil {
		return nil, err
	}
	return &acc, nil
}

// SaveAccount persists account credentials to disk with restricted permissions (0600).
func SaveAccount(acc *Account) error {
	data, err := json.MarshalIndent(acc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(GetAccountPath(), data, 0600)
}
