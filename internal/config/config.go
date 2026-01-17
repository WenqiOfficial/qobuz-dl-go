package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Quality int    `json:"quality"`
	Output  string `json:"output"`
	Proxy   string `json:"proxy"`
	NoSave  bool   `json:"nosave"`
	OgCover bool   `json:"og_cover"`
}

type Account struct {
	Email     string `json:"email"`
	Password  string `json:"password"` // In real app, consider encrypting
	UserToken string `json:"user_auth_token"`
	UserID    int    `json:"user_id"`
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

func GetConfigPath() string {
	// Simple current directory for now, or user home
	return "config.json"
}

func GetAccountPath() string {
	return "account.json"
}

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

func SaveAccount(acc *Account) error {
	data, err := json.MarshalIndent(acc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(GetAccountPath(), data, 0600)
}
