package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"qobuz-dl-go/internal/api"
	"qobuz-dl-go/internal/config"
	"qobuz-dl-go/internal/engine"
	"qobuz-dl-go/internal/server"
	"qobuz-dl-go/internal/version"
)

var (
	// Flags
	flagAppID     string
	flagAppSecret string
	flagEmail     string
	flagPassword  string
	flagToken     string
	flagQuality   int
	flagOutputDir string
	flagProxy     string
	flagNoSave    bool
	flagPort      string
	flagThreads   int
	flagNoCDN     bool // Disable CDN proxy site
)

func main() {
	var rootCmd = &cobra.Command{
		Use:     "qobuz-dl-go",
		Short:   "A high performance Qobuz music downloader",
		Long:    `A Go implementation of the Qobuz downloader with dual-mode support (CLI & Web).`,
		Version: version.Short(),
	}

	// Custom version template
	rootCmd.SetVersionTemplate(fmt.Sprintf("%s\n", version.Full()))

	var serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the web server",
		Run: func(cmd *cobra.Command, args []string) {
			client, err := setupClient(true) // strict=true? Maybe false for server?
			if err != nil {
				fmt.Printf("Startup Error: %v\n", err)
				os.Exit(1)
			}

			eng := engine.New(client)
			fmt.Printf("Starting Server on port %s...\n", flagPort)
			server.Start(eng, flagPort)
		},
	}
	serveCmd.Flags().StringVarP(&flagPort, "port", "P", "8080", "Server port")

	var dlCmd = &cobra.Command{
		Use:   "dl [track_id/url]",
		Short: "Download a track or album by ID or URL",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			input := args[0]

			// Setup Client
			client, err := setupClient(false)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			// Parse Resource
			resType, id, err := api.ParseURL(input)
			if err != nil {
				// Fallback to track ID if pure digits or simple string
				resType = api.TypeTrack
				id = input
			}

			fmt.Printf("Processing %s ID: %s\n", resType, id)

			// Initialize Engine
			eng := engine.New(client)

			// Set concurrency if specified
			if flagThreads > 0 {
				eng.SetConcurrency(flagThreads)
			}

			// Default Output Dir from Config if not flagged
			if flagOutputDir == "." {
				// We could load config default here, but let's stick to current dir
			}

			if resType == api.TypeAlbum {
				// Album Download
				err := eng.DownloadAlbum(context.Background(), id, flagQuality, flagOutputDir)
				if err != nil {
					fmt.Printf("Album download failed: %v\n", err)
					os.Exit(1)
				}
			} else {
				// Track Download with simple progress
				fmt.Printf("Downloading track %s...\n", id)
				err := eng.DownloadTrack(context.Background(), id, flagQuality, flagOutputDir, func(current, total int64) {
					if total > 0 {
						percent := int(float64(current) / float64(total) * 100)
						fmt.Printf("\r  Progress: %d%%", percent)
					}
				})

				if err != nil {
					fmt.Printf("\nDownload failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("\n  Done!")
			}

			fmt.Println("Work complete!")
		},
	}

	// dlCmd Flags
	dlCmd.Flags().IntVarP(&flagQuality, "quality", "q", 6, "Quality ID (5=MP3, 6=FLAC 16bit, 7=FLAC 24bit, 27=FLAC 24bit>96)")
	dlCmd.Flags().StringVarP(&flagOutputDir, "output", "o", ".", "Output directory")
	dlCmd.Flags().IntVarP(&flagThreads, "threads", "n", 3, "Number of concurrent download threads (1-10)")

	rootCmd.AddCommand(dlCmd)
	rootCmd.AddCommand(serveCmd)

	// Global Flags
	rootCmd.PersistentFlags().StringVar(&flagAppID, "app-id", "", "Qobuz App ID")
	rootCmd.PersistentFlags().StringVar(&flagAppSecret, "app-secret", "", "Qobuz App Secret")
	rootCmd.PersistentFlags().StringVarP(&flagEmail, "email", "e", "", "User Email")
	rootCmd.PersistentFlags().StringVarP(&flagPassword, "password", "p", "", "User Password")
	rootCmd.PersistentFlags().StringVarP(&flagToken, "token", "t", "", "User Auth Token")
	rootCmd.PersistentFlags().StringVar(&flagProxy, "proxy", "", "Proxy URL (http/https/socks5), overrides HTTP_PROXY/HTTPS_PROXY env")
	rootCmd.PersistentFlags().BoolVar(&flagNoSave, "nosave", false, "Do not save credentials to account.json")
	rootCmd.PersistentFlags().BoolVar(&flagNoCDN, "nocdn", false, "Disable CDN proxy, connect to Qobuz directly")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// setupClient handles all configuration, authentication, and client initialization logic
func setupClient(isServer bool) (*api.Client, error) {
	// 1. Load Configs
	_, _ = config.LoadConfig() // Currently unused but prepared
	acc, _ := config.LoadAccount()

	// 2. Resolve Proxy
	// Priority: Flag > Config(future) > Env(handled by req)

	// 3. Get App ID (without validation yet - need user token first)
	appID := flagAppID
	appSecret := flagAppSecret

	// If not provided in flags, check Account
	if appID == "" && acc.AppID != "" {
		appID = acc.AppID
	}
	if appSecret == "" && acc.AppSecret != "" {
		appSecret = acc.AppSecret
	}

	// If appID is missing, fetch it (but don't validate secret yet)
	needSecretValidation := false
	if appID == "" {
		fmt.Println("App ID missing. Fetching from Qobuz...")
		fetchedID, secrets, err := api.FetchSecrets(flagProxy, !flagNoCDN)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch secrets: %w", err)
		}
		appID = fetchedID
		// Store secrets for later validation after login
		acc.PendingSecrets = secrets
		needSecretValidation = true
	} else if appSecret == "" {
		// Have appID but no secret
		needSecretValidation = true
	}

	// 4. Create Client with current appID/appSecret
	client := api.NewClient(appID, appSecret)

	// Set CDN proxy preference
	if flagNoCDN {
		client.SetUseProxy(false)
		fmt.Println("CDN proxy disabled, using direct connection")
	}

	if flagProxy != "" {
		if err := client.SetProxy(flagProxy); err != nil {
			fmt.Printf("Warning: Failed to set proxy: %v\n", err)
		}
	}

	// 5. Resolve User Auth FIRST (needed for secret validation)
	userToken := flagToken
	if userToken == "" && acc.UserToken != "" {
		userToken = acc.UserToken
	}

	if userToken != "" {
		client.SetUserToken(userToken)
	} else {
		// Need to login first
		email := flagEmail
		pass := flagPassword

		if email == "" {
			email = acc.Email
		}
		if pass == "" {
			pass = acc.Password
		}

		if email == "" || pass == "" {
			if !isServer {
				fmt.Println("Authentication required.")
				reader := bufio.NewReader(os.Stdin)

				if email == "" {
					fmt.Print("Email: ")
					email, _ = reader.ReadString('\n')
					email = strings.TrimSpace(email)
				}

				if pass == "" {
					fmt.Print("Password: ")
					pass, _ = reader.ReadString('\n')
					pass = strings.TrimSpace(pass)
				}
			}
		}

		if email != "" && pass != "" {
			fmt.Println("Logging in...")
			resp, err := client.Login(email, pass)
			if err != nil {
				return nil, fmt.Errorf("login failed: %w", err)
			}

			userToken = resp.UserAuthToken

			// Save credentials
			if !flagNoSave {
				acc.Email = email
				acc.Password = pass
				acc.UserToken = resp.UserAuthToken
				acc.UserID = resp.User.ID
			}
		} else if !isServer {
			return nil, fmt.Errorf("authentication required. Provide --token or --email/--password")
		} else {
			fmt.Println("Warning: Starting server without user authentication. Some features may fail.")
		}
	}

	// 6. NOW validate/find secret (after we have user token)
	if needSecretValidation || (appSecret != "" && !client.ValidateSecret()) {
		if appSecret != "" {
			fmt.Println("Saved secret is invalid. Refreshing...")
		}

		// Get fresh secrets if we don't have pending ones
		secrets := acc.PendingSecrets
		if len(secrets) == 0 {
			fmt.Println("Fetching secrets from Qobuz...")
			fetchedID, fetchedSecrets, err := api.FetchSecrets(flagProxy, !flagNoCDN)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch secrets: %w", err)
			}
			appID = fetchedID
			secrets = fetchedSecrets
			client = api.NewClient(appID, "")
			if flagProxy != "" {
				client.SetProxy(flagProxy)
			}
			if userToken != "" {
				client.SetUserToken(userToken)
			}
		}

		fmt.Printf("Testing %d secrets for AppID: %s...\n", len(secrets), appID)
		validSecret, err := client.FindValidSecret(secrets)
		if err != nil {
			return nil, fmt.Errorf("no valid secret found: %w", err)
		}

		fmt.Println("Valid secret found!")
		appSecret = validSecret
		client.AppSecret = appSecret

		// Clear pending secrets
		acc.PendingSecrets = nil
	}

	// 7. Save account
	if !flagNoSave {
		acc.AppID = appID
		acc.AppSecret = appSecret
		if err := config.SaveAccount(acc); err != nil {
			fmt.Printf("Warning: Failed to save account: %v\n", err)
		} else if needSecretValidation {
			fmt.Println("Credentials saved.")
		}
	}

	return client, nil
}
