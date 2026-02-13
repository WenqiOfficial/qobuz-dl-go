# 📖 Usage Guide

**English** | [中文](usage.md)

## 1. Basic Download

Download a track or album by providing the URL or ID:

```bash
# Download an album
./qobuz-dl-go dl https://play.qobuz.com/album/j3wq4jjuhznjb

# Download a single track
./qobuz-dl-go dl https://play.qobuz.com/track/123456
```

## 2. Authentication

The program prioritizes cached credentials from `account.json`. If no cache exists, you can log in via:

**Interactive Login (Recommended)**:
Simply run the download command - the program will prompt for email and password.

**Command-line Login**:
```bash
./qobuz-dl-go dl <url> --email your@email.com --password yourpassword
```

**Token Login**:
```bash
./qobuz-dl-go dl <url> --token <user-auth-token>
```

## 3. Download Quality

Use `-q` or `--quality` to specify audio quality:

*   `5`: MP3 320kbps
*   `6`: FLAC Lossless (16-bit / 44.1kHz) **(Default)**
*   `7`: FLAC 24-bit / 96kHz
*   `27`: FLAC 24-bit / 192kHz (Highest quality)

```bash
# Download in highest quality
./qobuz-dl-go dl <url> -q 27
```

## 4. Proxy Settings

The program automatically uses proxy settings from system environment variables (`HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`).

You can also manually specify a proxy with `--proxy`:

```bash
# HTTP Proxy
./qobuz-dl-go dl <url> --proxy http://127.0.0.1:7890

# SOCKS5 Proxy
./qobuz-dl-go dl <url> --proxy socks5://127.0.0.1:1080
```

## 5. CDN Acceleration

CDN acceleration is enabled by default for Chinese mainland access. To disable:

```bash
./qobuz-dl-go dl <url> --nocdn
```

## 6. Search

Use the `search` command to search and download:

```bash
# Search albums (default)
./qobuz-dl-go search "Mili"

# Search tracks
./qobuz-dl-go search "world.execute(me)" -T track

# Search artists
./qobuz-dl-go search "Mili" -T artist
```

Search results are displayed as an interactive list. Select an item to download. You can set download preferences such as quality and proxy in advance.

## 7. Concurrent Downloads

Use `--threads` or `-n` to set the number of concurrent download threads (default 3, max 10):

```bash
./qobuz-dl-go dl <url> -n 5
```

## 8. Other Options

*   `--output`, `-o`: Specify output directory (defaults to current directory).
*   `--threads`, `-n`: Number of concurrent download threads (1-10, default 3).
*   `--nosave`: Don't save credentials to local `account.json`.
*   `--nocdn`: Disable CDN acceleration, connect directly to Qobuz servers.
*   `--app-id`, `--app-secret`: Manually specify App ID and Secret (usually not needed - auto-fetched).

## 9. Auto Update

Check and update to the latest version:

```bash
./qobuz-dl-go update
```

## 10. Shell Completion

Generate shell auto-completion scripts:

```bash
# Bash
./qobuz-dl-go completion bash > /etc/bash_completion.d/qobuz-dl-go

# Zsh
./qobuz-dl-go completion zsh > "${fpath[1]}/_qobuz-dl-go"

# Fish
./qobuz-dl-go completion fish > ~/.config/fish/completions/qobuz-dl-go.fish

# PowerShell
./qobuz-dl-go completion powershell | Out-String | Invoke-Expression
```

## 📂 Configuration Files

The program generates the following files in the same directory:

*   `account.json`: Stores encrypted user credentials (Token, UserID, etc.).
*   `config.json`: (Planned) For storing default download path, quality preferences, and other global settings.
