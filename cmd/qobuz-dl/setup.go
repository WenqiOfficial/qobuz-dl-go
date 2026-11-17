package main

import (
	"os"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/auth"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/config"
)

var newAuthManager = func(options auth.Options) *auth.Manager {
	return auth.NewManager(options)
}

func authOptions(isServer, quiet bool) auth.Options {
	return auth.Options{
		AppID:            flagAppID,
		AppSecret:        flagAppSecret,
		Email:            flagEmail,
		Password:         flagPassword,
		Token:            flagToken,
		ProxyURL:         flagProxy,
		DisableProxySite: flagNoCDN,
		Server:           isServer,
		AllowPrompt:      !isServer,
		Quiet:            quiet,
		Save:             !flagNoSave,
		Input:            os.Stdin,
		Output:           os.Stdout,
	}
}

func setupClient(isServer bool) (*api.Client, *config.Account, error) {
	return newAuthManager(authOptions(isServer, false)).Prepare()
}
