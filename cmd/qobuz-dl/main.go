package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/config"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/engine"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/server"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/updater"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/version"
	"golang.org/x/term"
)

var (
	// Flags
	flagAppID      string
	flagAppSecret  string
	flagEmail      string
	flagPassword   string
	flagToken      string
	flagQuality    int
	flagOutputDir  string
	flagProxy      string
	flagNoSave     bool
	flagPort       string
	flagThreads    int
	flagNoCDN      bool   // Disable CDN proxy site
	flagSearchType string // Search type: album, track, artist
)

func main() {
	// Clean up leftover backup from previous update
	cleanupOldBinary()

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

	// Update Command
	var updateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update to the latest version",
		Run: func(cmd *cobra.Command, args []string) {
			// Configure proxy for updater if specified
			if flagProxy != "" {
				if err := updater.SetProxy(flagProxy); err != nil {
					fmt.Printf("Warning: Failed to set proxy for update: %v\n", err)
				}
			}

			fmt.Println("Checking for updates...")

			// Use CDN unless --nocdn is specified
			useCDN := !flagNoCDN
			result, err := updater.CheckForUpdate(useCDN)
			if err != nil {
				fmt.Printf("Failed to check for updates: %v\n", err)
				os.Exit(1)
			}

			if !result.HasUpdate {
				fmt.Printf("Already up to date (v%s)\n", result.CurrentVersion)
				return
			}

			fmt.Printf("Update available: v%s -> v%s\n", result.CurrentVersion, result.LatestVersion)

			// Get platform-specific asset
			asset, err := result.ReleaseInfo.GetPlatformAsset()
			if err != nil {
				fmt.Printf("No release found for your platform: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Downloading %s (%.2f MB)...\n", asset.Name, float64(asset.Size)/1024/1024)

			// Download and apply update atomically
			err = updater.DownloadAndApply(asset, result.ReleaseInfo.TagName, func(current, total int64) {
				percent := int(float64(current) / float64(total) * 100)
				fmt.Printf("\r  Progress: %d%%", percent)
			})
			if err != nil {
				fmt.Printf("\nUpdate failed: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("\n\nUpdate complete! v%s -> v%s\n", result.CurrentVersion, result.LatestVersion)
			fmt.Println("Please restart the application to use the new version.")
			os.Exit(0)
		},
	}

	// Completion Command - generates completion scripts to files
	var completionCmd = &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script files",
		Long: `Generate shell completion scripts and save them to the current directory.

Generated files:
  bash:       qobuz-dl-go.bash
  zsh:        _qobuz-dl-go
  fish:       qobuz-dl-go.fish
  powershell: qobuz-dl-go.ps1`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Run: func(cmd *cobra.Command, args []string) {
			var filename string
			var content strings.Builder

			switch args[0] {
			case "bash":
				filename = "qobuz-dl-go.bash"
				rootCmd.GenBashCompletion(&content)
			case "zsh":
				filename = "_qobuz-dl-go"
				rootCmd.GenZshCompletion(&content)
			case "fish":
				filename = "qobuz-dl-go.fish"
				rootCmd.GenFishCompletion(&content, true)
			case "powershell":
				filename = "qobuz-dl-go.ps1"
				rootCmd.GenPowerShellCompletionWithDesc(&content)
			default:
				fmt.Printf("Unknown shell: %s\n", args[0])
				fmt.Println("Valid options: bash, zsh, fish, powershell")
				os.Exit(1)
			}

			if err := os.WriteFile(filename, []byte(content.String()), 0644); err != nil {
				fmt.Printf("Failed to write completion file: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Completion script generated: %s\n", filename)
			os.Exit(0)
		},
	}

	rootCmd.AddCommand(dlCmd)
	rootCmd.AddCommand(searchCmd(dlCmd))
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(completionCmd)

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

	// Show version info after command execution
	showVersionInfo()
}

// searchCmd creates the search command with download integration.
func searchCmd(dlCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search and download music from Qobuz",
		Long: `Search for albums, tracks, or artists on Qobuz and interactively select items to download.

Results are displayed in a numbered list. Enter the number to download.

Search types:
  album   Search for albums (default)
  track   Search for tracks
  artist  Search for artists (lists their albums for selection)`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			query := strings.Join(args, " ")
			if len(strings.TrimSpace(query)) < 2 {
				fmt.Println("Error: search query too short (minimum 2 characters)")
				os.Exit(1)
			}

			// Setup client
			client, err := setupClient(false)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			eng := engine.New(client)
			if flagThreads > 0 {
				eng.SetConcurrency(flagThreads)
			}

			searchType := strings.ToLower(flagSearchType)

			switch searchType {
			case "album":
				runAlbumSearch(client, eng, query)
			case "track":
				runTrackSearch(client, eng, query)
			case "artist":
				runArtistSearch(client, eng, query)
			default:
				fmt.Printf("Error: unknown search type %q (use album, track, or artist)\n", searchType)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVarP(&flagSearchType, "type", "T", "album", "Search type: album, track, artist")
	cmd.Flags().IntVarP(&flagQuality, "quality", "q", 6, "Quality ID (5=MP3, 6=FLAC 16bit, 7=FLAC 24bit, 27=FLAC 24bit>96)")
	cmd.Flags().StringVarP(&flagOutputDir, "output", "o", ".", "Output directory")
	cmd.Flags().IntVarP(&flagThreads, "threads", "n", 3, "Number of concurrent download threads (1-10)")

	return cmd
}

// formatDuration formats seconds into a human-readable mm:ss or h:mm:ss string.
func formatDuration(seconds int) string {
	if seconds <= 0 {
		return "0:00"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// qualityTag returns a concise quality label for display.
func qualityTag(hiRes bool, bitDepth int, sampleRate float64) string {
	if hiRes {
		return fmt.Sprintf("Hi-Res %d/%gkHz", bitDepth, sampleRate)
	}
	return "Lossless"
}

// runeWidth returns the display width of a rune (CJK = 2, others = 1).
func runeWidth(r rune) int {
	if r >= 0x1100 && r <= 0x115F ||
		r >= 0x2E80 && r <= 0x9FFF ||
		r >= 0xA960 && r <= 0xA97F ||
		r >= 0xAC00 && r <= 0xD7FF ||
		r >= 0xF900 && r <= 0xFAFF ||
		r >= 0xFE10 && r <= 0xFE1F ||
		r >= 0xFE30 && r <= 0xFE6F ||
		r >= 0xFF00 && r <= 0xFF60 ||
		r >= 0xFFE0 && r <= 0xFFE6 ||
		r >= 0x20000 && r <= 0x2FFFF ||
		r >= 0x30000 && r <= 0x3FFFF {
		return 2
	}
	if r < 0x20 || r == 0x7F ||
		r >= 0x200B && r <= 0x200F ||
		r >= 0x2028 && r <= 0x202E ||
		r >= 0xFE00 && r <= 0xFE0F ||
		r == 0xFEFF {
		return 0
	}
	return 1
}

// stringDisplayWidth calculates the total display width of a string.
func stringDisplayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// truncateToWidth truncates a string to fit within a given display width, adding "..." if needed.
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}
	if stringDisplayWidth(s) <= maxWidth {
		return s
	}
	target := maxWidth - 3
	w := 0
	var result []rune
	for _, r := range s {
		rw := runeWidth(r)
		if w+rw > target {
			break
		}
		result = append(result, r)
		w += rw
	}
	return string(result) + "..."
}

// padRight pads a string to a fixed display width using spaces.
func padRight(s string, targetWidth int) string {
	cur := stringDisplayWidth(s)
	if cur >= targetWidth {
		return truncateToWidth(s, targetWidth)
	}
	return s + strings.Repeat(" ", targetWidth-cur)
}

// padLeft pads a string to a fixed display width with leading spaces.
func padLeft(s string, targetWidth int) string {
	cur := stringDisplayWidth(s)
	if cur >= targetWidth {
		return truncateToWidth(s, targetWidth)
	}
	return strings.Repeat(" ", targetWidth-cur) + s
}

// readSelection reads user input and returns the selected 1-based index.
// Returns -1 for quit, 0 for invalid input.
func readSelection(max int) int {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\nEnter number to download (1-%d, q to quit): ", max)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "" || strings.EqualFold(line, "q") || strings.EqualFold(line, "quit") {
		return -1
	}

	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > max {
		return 0
	}
	return n
}

// runAlbumSearch performs an album search and handles interactive download.
func runAlbumSearch(client *api.Client, eng *engine.Engine, query string) {
	fmt.Printf("Searching albums for %q...\n\n", query)

	result, err := client.SearchAlbums(query, 10)
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		os.Exit(1)
	}

	items := result.Albums.Items
	if len(items) == 0 {
		fmt.Println("No albums found.")
		return
	}

	// Display results
	fmt.Println("  #  Artist - Album                                         Duration  Quality")
	fmt.Println(strings.Repeat("─", 88))

	for i, album := range items {
		title := album.Title
		if album.Version != "" {
			title += " (" + album.Version + ")"
		}

		display := fmt.Sprintf("%s - %s", album.Artist.Name, title)
		dur := formatDuration(album.Duration)
		tag := qualityTag(album.HiresStreamable, album.MaximumBitDepth, album.MaximumSamplingRate)

		// Truncate display to fit
		maxDisplayWidth := 54
		displayPadded := padRight(display, maxDisplayWidth)
		durPadded := padLeft(dur, 8)

		fmt.Printf(" %2d  %s %s  [%s]\n", i+1, displayPadded, durPadded, tag)
	}

	// Interactive selection
	sel := readSelection(len(items))
	if sel == -1 {
		return
	}
	if sel == 0 {
		fmt.Println("Invalid selection.")
		return
	}

	album := items[sel-1]
	fmt.Println()
	err = eng.DownloadAlbum(context.Background(), album.ID, flagQuality, flagOutputDir)
	if err != nil {
		fmt.Printf("Download failed: %v\n", err)
		os.Exit(1)
	}
}

// runTrackSearch performs a track search and handles interactive download.
func runTrackSearch(client *api.Client, eng *engine.Engine, query string) {
	fmt.Printf("Searching tracks for %q...\n\n", query)

	result, err := client.SearchTracks(query, 10)
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		os.Exit(1)
	}

	items := result.Tracks.Items
	if len(items) == 0 {
		fmt.Println("No tracks found.")
		return
	}

	// Display results
	fmt.Println("  #  Artist - Title                                       Duration  Quality")
	fmt.Println(strings.Repeat("─", 78))

	for i, track := range items {
		title := track.Title
		if track.Version != "" {
			title += " (" + track.Version + ")"
		}

		display := fmt.Sprintf("%s - %s", track.Performer.Name, title)
		dur := formatDuration(track.Duration)
		tag := qualityTag(track.HiresStreamable, track.MaximumBitDepth, track.MaximumSamplingRate)

		maxDisplayWidth := 54
		displayPadded := padRight(display, maxDisplayWidth)
		durPadded := padLeft(dur, 8)

		fmt.Printf(" %2d  %s %s  [%s]\n", i+1, displayPadded, durPadded, tag)
	}

	// Interactive selection
	sel := readSelection(len(items))
	if sel == -1 {
		return
	}
	if sel == 0 {
		fmt.Println("Invalid selection.")
		return
	}

	track := items[sel-1]
	trackID := strconv.Itoa(track.ID)
	fmt.Printf("\nDownloading %s - %s...\n", track.Performer.Name, track.Title)

	err = eng.DownloadTrack(context.Background(), trackID, flagQuality, flagOutputDir, func(current, total int64) {
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

// runArtistSearch performs an artist search, then lists albums for download.
func runArtistSearch(client *api.Client, eng *engine.Engine, query string) {
	fmt.Printf("Searching artists for %q...\n\n", query)

	result, err := client.SearchArtists(query, 10)
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		os.Exit(1)
	}

	items := result.Artists.Items
	if len(items) == 0 {
		fmt.Println("No artists found.")
		return
	}

	// Display artist results
	fmt.Println("  #  Artist                                                        Albums")
	fmt.Println(strings.Repeat("─", 78))

	for i, artist := range items {
		display := artist.Name
		maxDisplayWidth := 62
		displayPadded := padRight(display, maxDisplayWidth)

		fmt.Printf(" %2d  %s %5d\n", i+1, displayPadded, artist.AlbumsCount)
	}

	// Select artist
	sel := readSelection(len(items))
	if sel == -1 {
		return
	}
	if sel == 0 {
		fmt.Println("Invalid selection.")
		return
	}

	artist := items[sel-1]
	fmt.Printf("\nLoading albums for %s...\n\n", artist.Name)

	// Fetch artist's albums
	artistResp, err := client.GetArtistAlbums(strconv.Itoa(artist.ID), 10, 0)
	if err != nil {
		fmt.Printf("Failed to get artist albums: %v\n", err)
		os.Exit(1)
	}

	albums := artistResp.Albums.Items
	if len(albums) == 0 {
		fmt.Println("No albums found for this artist.")
		return
	}

	// Display album list
	fmt.Println("  #  Album                                                Duration  Quality")
	fmt.Println(strings.Repeat("─", 78))

	for i, album := range albums {
		title := album.Title
		if album.Version != "" {
			title += " (" + album.Version + ")"
		}

		dur := formatDuration(album.Duration)
		tag := qualityTag(album.HiresStreamable, album.MaximumBitDepth, album.MaximumSamplingRate)

		maxDisplayWidth := 54
		displayPadded := padRight(title, maxDisplayWidth)
		durPadded := padLeft(dur, 8)

		fmt.Printf(" %2d  %s %s  [%s]\n", i+1, displayPadded, durPadded, tag)
	}

	// Select album
	sel = readSelection(len(albums))
	if sel == -1 {
		return
	}
	if sel == 0 {
		fmt.Println("Invalid selection.")
		return
	}

	album := albums[sel-1]
	fmt.Println()
	err = eng.DownloadAlbum(context.Background(), album.ID, flagQuality, flagOutputDir)
	if err != nil {
		fmt.Printf("Download failed: %v\n", err)
		os.Exit(1)
	}
}

func shouldRefreshLogin(providedEmail string, acc *config.Account) bool {
	if strings.TrimSpace(providedEmail) == "" {
		return false
	}
	if acc == nil {
		return true
	}
	if acc.Email == "" {
		return true
	}
	return !strings.EqualFold(strings.TrimSpace(acc.Email), strings.TrimSpace(providedEmail))
}

func readSecret(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		secret, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(secret)), nil
	}
	value, err := reader.ReadString('\n')
	return strings.TrimSpace(value), err
}

func promptCustomAppCredentials(client *api.Client, userToken string) (*api.Client, string, string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("App ID: ")
	customID, _ := reader.ReadString('\n')
	customID = strings.TrimSpace(customID)
	customSecret, err := readSecret(reader, "App Secret: ")
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to read custom App Secret: %w", err)
	}
	if customID == "" || customSecret == "" {
		return nil, "", "", fmt.Errorf("custom App ID and App Secret are required")
	}

	customClient := api.NewClient(customID, customSecret)
	customClient.SetUseProxy(client.UseProxy)
	if flagProxy != "" {
		if err := customClient.SetProxy(flagProxy); err != nil {
			return nil, "", "", fmt.Errorf("failed to configure proxy: %w", err)
		}
	}
	if userToken != "" {
		customClient.SetUserToken(userToken)
	}
	return customClient, customID, customSecret, nil
}

func saveAccountSnapshot(acc *config.Account) error {
	if flagNoSave {
		return nil
	}
	return config.SaveAccount(acc)
}

// setupClient handles all configuration, authentication, and client initialization logic
func setupClient(isServer bool) (*api.Client, error) {
	_, _ = config.LoadConfig() // Currently unused but prepared
	acc, _ := config.LoadAccount()


	appID := flagAppID
	appSecret := flagAppSecret

	if appID == "" && acc.AppID != "" {
		appID = acc.AppID
	}
	if appSecret == "" && acc.AppSecret != "" {
		appSecret = acc.AppSecret
	}

	needSecretValidation := false
	var secretsFetchErr error
	manualCredentials := false
	if appID == "" {
		fmt.Println("App ID missing. Fetching from Qobuz...")
		fetchedID, secrets, err := api.FetchSecrets(flagProxy, !flagNoCDN)
		appID = fetchedID
		// Store secrets for later validation after login
		acc.PendingSecrets = secrets
		needSecretValidation = true
		secretsFetchErr = err
		if err != nil && isServer {
			return nil, fmt.Errorf("failed to fetch secrets: %w", err)
		}
	} else if appSecret == "" {
		// Have appID but no secret
		needSecretValidation = true
	}

	client := api.NewClient(appID, appSecret)

	if flagNoCDN {
		client.SetUseProxy(false)
		fmt.Println("CDN proxy disabled, using direct connection")
	}

	if flagProxy != "" {
		if err := client.SetProxy(flagProxy); err != nil {
			fmt.Printf("Warning: Failed to set proxy: %v\n", err)
		}
	}

	if appID == "" && secretsFetchErr != nil {
		if isServer {
			return nil, fmt.Errorf("failed to fetch secrets: %w", secretsFetchErr)
		}
		fmt.Printf("Automatic credential discovery failed: %v\n", secretsFetchErr)
		fmt.Println("Please enter custom Qobuz application credentials before login.")
		customClient, customID, customSecret, customErr := promptCustomAppCredentials(client, "")
		if customErr != nil {
			return nil, customErr
		}
		client = customClient
		appID = customID
		appSecret = customSecret
		manualCredentials = true
		needSecretValidation = true
		secretsFetchErr = nil
	}

	email := flagEmail
	if email == "" {
		email = acc.Email
	}
	if shouldRefreshLogin(email, acc) {
		acc.UserToken = ""
		flagToken = ""
	}

	userToken := flagToken
	if userToken == "" && acc.UserToken != "" {
		userToken = acc.UserToken
	}

	if userToken != "" {
		client.SetUserToken(userToken)
	} else {
		// Need to login first
		pass := flagPassword

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
					readPassword, readErr := readSecret(reader, "Password: ")
					if readErr != nil {
						return nil, fmt.Errorf("failed to read password: %w", readErr)
					}
					pass = readPassword
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

	if needSecretValidation || (appSecret != "" && !client.ValidateSecret()) {
		if appSecret != "" {
			fmt.Println("Saved secret is invalid. Refreshing...")
		}

		// Get fresh secrets if we don't have pending ones
		secrets := acc.PendingSecrets
		if len(secrets) == 0 && !manualCredentials && secretsFetchErr == nil {
			fmt.Println("Fetching secrets from Qobuz...")
			fetchedID, fetchedSecrets, err := api.FetchSecrets(flagProxy, !flagNoCDN)
			if err != nil {
				secretsFetchErr = err
			} else {
				appID = fetchedID
				secrets = fetchedSecrets
				client = api.NewClient(appID, "")
				client.SetUseProxy(!flagNoCDN)
				if flagProxy != "" {
					if err := client.SetProxy(flagProxy); err != nil {
						return nil, fmt.Errorf("failed to configure proxy: %w", err)
					}
				}
				if userToken != "" {
					client.SetUserToken(userToken)
				}
			}
		}

		if manualCredentials {
			if !client.ValidateSecret() {
				return nil, fmt.Errorf("custom App ID/App Secret failed validation")
			}
			fmt.Println("Custom App ID/App Secret validated successfully!")
		} else if secretsFetchErr != nil {
			if isServer {
				return nil, fmt.Errorf("failed to fetch secrets: %w", secretsFetchErr)
			}
			fmt.Printf("Automatic credential discovery failed: %v\n", secretsFetchErr)
			fmt.Println("Automatically fetched App ID/App Secret are not usable.")
			fmt.Println("Please enter custom Qobuz application credentials.")
			customClient, customID, customSecret, customErr := promptCustomAppCredentials(client, userToken)
			if customErr != nil {
				return nil, customErr
			}
			if !customClient.ValidateSecret() {
				return nil, fmt.Errorf("custom App ID/App Secret failed validation")
			}
			client = customClient
			appID = customID
			appSecret = customSecret
		} else {
			fmt.Printf("Testing %d secrets for AppID: %s...\n", len(secrets), appID)
			validSecret, err := client.FindValidSecret(secrets)
			if err != nil {
				if isServer {
					return nil, fmt.Errorf("no valid secret found: %w", err)
				}
				fmt.Println("Automatically fetched App ID/App Secret are not usable.")
				fmt.Println("Please enter custom Qobuz application credentials.")
				customClient, customID, customSecret, customErr := promptCustomAppCredentials(client, userToken)
				if customErr != nil {
					return nil, customErr
				}
				if !customClient.ValidateSecret() {
					return nil, fmt.Errorf("custom App ID/App Secret failed validation")
				}
				client = customClient
				appID = customID
				appSecret = customSecret
			} else {
				fmt.Println("Valid secret found!")
				appSecret = validSecret
				client.AppSecret = appSecret
			}
		}

		// Clear pending secrets
		acc.PendingSecrets = nil
	}

	if !flagNoSave {
		acc.AppID = appID
		acc.AppSecret = appSecret
		if err := saveAccountSnapshot(acc); err != nil {
			fmt.Printf("Warning: Failed to save credentials: %v\n", err)
		} else if needSecretValidation {
			fmt.Println("Credentials saved.")
		}
	}

	return client, nil
}

// showVersionInfo displays version information and checks for updates
func showVersionInfo() {
	// Always show current version
	fmt.Printf("\nQobuz DL Go v%s\n", version.Version)

	// Skip update check for dev builds
	if version.Version == "dev" || strings.HasPrefix(version.Version, "dev-") {
		fmt.Println("Skip check for dev version.")
		return
	}

	// Check for updates (use CDN by default for faster check)
	result, err := updater.CheckForUpdate(true)
	if err != nil {
		// Silently ignore update check failures
		return
	}

	if result.HasUpdate {
		fmt.Printf("\nUpdate v%s available! Update with:\n", result.LatestVersion)
		fmt.Println("    qobuz-dl-go update")
	}
}

// cleanupOldBinary removes the backup file left by selfupdate after a successful update
func cleanupOldBinary() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	// selfupdate renames old binary to: .{filename}.old
	// e.g., qobuz-dl-go.exe -> .qobuz-dl-go.exe.old
	dir := filepath.Dir(exePath)
	name := filepath.Base(exePath)
	oldPath := filepath.Join(dir, "."+name+".old")
	os.Remove(oldPath) // Silently ignore errors
}
