package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"qobuz-dl-go/internal/api"
	"qobuz-dl-go/internal/engine"
	"qobuz-dl-go/internal/server"
)

var (
	appID     string
	appSecret string
	email     string
	password  string
	userToken string
	quality   int
	outputDir string
	port      string
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
			
			// Initialize Client
			if appID == "" || appSecret == "" {
				fmt.Println("Warning: No app-id/secret provided. You might need them for downloading.")
			}
			
			client := api.NewClient(appID, appSecret)
			if userToken != "" {
				client.SetUserToken(userToken)
			}
			
			eng := engine.New(client)
			
			fmt.Printf("Starting Server on port %s...\n", port)
			server.Start(eng, port)
		},
	}
	serveCmd.Flags().StringVarP(&port, "port", "P", "8080", "Server port")

	var dlCmd = &cobra.Command{
		Use:   "dl [track_id]",
		Short: "Download a track by ID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			trackID := args[0]
			fmt.Printf("Starting download for Track ID: %s\n", trackID)

			// Initialize Client
			if appID == "" || appSecret == "" {
				fmt.Println("App ID/Secret not provided. Attempting to fetch from Qobuz...")
				var err error
				appID, appSecret, err = api.FetchSecrets()
				if err != nil {
					fmt.Printf("Failed to fetch secrets: %v. Please provide --app-id and --app-secret\n", err)
					os.Exit(1)
				}
				fmt.Printf("Found AppID: %s\n", appID)
			}

			client := api.NewClient(appID, appSecret)

			// Auth
			if userToken != "" {
				client.SetUserToken(userToken)
			} else if email != "" && password != "" {
				fmt.Println("Logging in...")
				if err := client.Login(email, password); err != nil {
					fmt.Printf("Login failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("Login successful.")
			} else {
				fmt.Println("Error: Provide --token or --email and --password")
				os.Exit(1)
			}

			// Initialize Engine
			eng := engine.New(client)

			// Progress Bar Setup
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

			// Download
			var totalSet bool
			err := eng.DownloadTrack(context.Background(), trackID, quality, outputDir, func(current, total int64) {
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
			fmt.Println("Download complete!")
		},
	}

	// dlCmd Flags
	dlCmd.Flags().IntVarP(&quality, "quality", "q", 6, "Quality ID (5=MP3, 6=FLAC 16bit, 7=FLAC 24bit, 27=FLAC 24bit>96)")
	dlCmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Output directory")

	rootCmd.AddCommand(dlCmd)
	rootCmd.AddCommand(serveCmd)

	// PERSISTENT FLAGS
	rootCmd.PersistentFlags().StringVar(&appID, "app-id", "", "Qobuz App ID")
	rootCmd.PersistentFlags().StringVar(&appSecret, "app-secret", "", "Qobuz App Secret")
	rootCmd.PersistentFlags().StringVarP(&email, "email", "e", "", "User Email")
	rootCmd.PersistentFlags().StringVarP(&password, "password", "p", "", "User Password")
	rootCmd.PersistentFlags().StringVarP(&userToken, "token", "t", "", "User Auth Token")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
