package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"qobuz-dl-go/internal/api"
	"qobuz-dl-go/internal/config"
	"qobuz-dl-go/internal/engine"
	"qobuz-dl-go/internal/server"
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
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "qobuz-dl",
		Short: "A high performance Qobuz music downloader",
		Long:  `A Go implementation of the Qobuz downloader with dual-mode support (CLI & Web).`,
	}

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
				// Track Download with Progress Bar
				p := mpb.New(mpb.WithWidth(60))
				bar := p.AddBar(0,
					mpb.PrependDecorators(
						decor.Name("downloading"),
						decor.Percentage(decor.WCSyncSpace),
					),
					mpb.AppendDecorators(
						decor.OnComplete(
							decor.EwmaETA(decor.ET_STYLE_GO, 60), "done",
						),
					),
				)

				var totalSet bool
				err := eng.DownloadTrack(context.Background(), id, flagQuality, flagOutputDir, func(current, total int64) {
					if !totalSet && total > 0 {
						bar.SetTotal(total, false)
						totalSet = true
					}
					bar.SetCurrent(current)
				})

				if err != nil {
					fmt.Printf("Download failed: %v\n", err)
					os.Exit(1)
				}
				p.Wait()
			}

			fmt.Println("Work complete!")
		},
	}

	// dlCmd Flags
	dlCmd.Flags().IntVarP(&flagQuality, "quality", "q", 6, "Quality ID (5=MP3, 6=FLAC 16bit, 7=FLAC 24bit, 27=FLAC 24bit>96)")
	dlCmd.Flags().StringVarP(&flagOutputDir, "output", "o", ".", "Output directory")

	rootCmd.AddCommand(dlCmd)
	rootCmd.AddCommand(serveCmd)

	// Global Flags
	rootCmd.PersistentFlags().StringVar(&flagAppID, "app-id", "", "Qobuz App ID")
	rootCmd.PersistentFlags().StringVar(&flagAppSecret, "app-secret", "", "Qobuz App Secret")
	rootCmd.PersistentFlags().StringVarP(&flagEmail, "email", "e", "", "User Email")
	rootCmd.PersistentFlags().StringVarP(&flagPassword, "password", "p", "", "User Password")
	rootCmd.PersistentFlags().StringVarP(&flagToken, "token", "t", "", "User Auth Token")
	rootCmd.PersistentFlags().StringVar(&flagProxy, "proxy", "", "Proxy URL (http/https/socks5)")
	rootCmd.PersistentFlags().BoolVar(&flagNoSave, "nosave", false, "Do not save credentials to account.json")

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
	// We only have Flag for now
	// If needed we can check config.Proxy here

	// 3. Resolve App Secrets
	appID := flagAppID
	appSecret := flagAppSecret

	// If not provided in flags, check Account
	if appID == "" && acc.AppID != "" {
		appID = acc.AppID
	}
	if appSecret == "" && acc.AppSecret != "" {
		appSecret = acc.AppSecret
	}

	// If still empty, fetch dynamically
	if appID == "" || appSecret == "" {
		fmt.Println("App ID/Secret missing. Fetching from Qobuz...")
		var secrets []string
		var err error
		fetchedID, secrets, err := api.FetchSecrets()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch secrets: %w", err)
		}

		// Create temp client to verify secrets
		// Note: We need to use the fetched ID
		tempClient := api.NewClient(fetchedID, "")
		if flagProxy != "" {
			tempClient.SetProxy(flagProxy)
		}

		fmt.Printf("Testing %d secrets for AppID: %s...\n", len(secrets), fetchedID)
		validSecret, err := tempClient.FindValidSecret(secrets)
		if err != nil {
			return nil, fmt.Errorf("no valid secret found: %w", err)
		}

		fmt.Println("Valid secret found!")
		appID = fetchedID
		appSecret = validSecret

		// Save if allowed
		if !flagNoSave {
			acc.AppID = appID
			acc.AppSecret = appSecret
			_ = config.SaveAccount(acc)
		}
	}

	// 4. Create Client
	client := api.NewClient(appID, appSecret)
	if flagProxy != "" {
		if err := client.SetProxy(flagProxy); err != nil {
			fmt.Printf("Warning: Failed to set proxy: %v\n", err)
		}
	}

	// 5. Resolve User Auth
	userToken := flagToken
	if userToken == "" && acc.UserToken != "" {
		userToken = acc.UserToken
	}

	if userToken != "" {
		client.SetUserToken(userToken)
		return client, nil
	}

	// If no token, try login with Email/Pass
	email := flagEmail
	pass := flagPassword

	// If flags missing, check account
	if email == "" {
		email = acc.Email
	}
	if pass == "" {
		pass = acc.Password
	}

	// If still missing and we are interactive (CLI), ask user
	// Check if simple CLI mode (not server or maybe server needs it too?)
	// For server, we usually don't block on stdin unless it's initial setup.
	// We'll ask if stdin is available.
	if email == "" || pass == "" {
		if !isServer { // Only prompt in DL mode for now to avoid blocking background services
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

		// Save if allowed
		if !flagNoSave {
			acc.Email = email
			// acc.Password = pass // Saving plaintext password is risky, maybe skip or ask user?
			// User asked for "using account.json" for next time.
			// Ideally we assume yes.
			acc.Password = pass
			acc.UserToken = resp.UserAuthToken
			acc.UserID = resp.User.ID
			if err := config.SaveAccount(acc); err != nil {
				fmt.Printf("Warning: Failed to save account: %v\n", err)
			} else {
				fmt.Println("Account credentials saved.")
			}
		}
	} else {
		// No credentials found
		if isServer {
			fmt.Println("Warning: Starting server without user authentication. Some features may fail.")
		} else {
			return nil, fmt.Errorf("authentication required. Provide --token or --email/--password")
		}
	}

	return client, nil
}
