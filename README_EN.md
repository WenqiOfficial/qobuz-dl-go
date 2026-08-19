# Qobuz-DL Go

**English** | [中文](README.md)

[![Go Version](https://img.shields.io/github/go-mod/go-version/WenqiOfficial/qobuz-dl-go?style=flat-square)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/WenqiOfficial/qobuz-dl-go?style=flat-square&color=blue)](https://github.com/WenqiOfficial/qobuz-dl-go/releases/latest)
[![License](https://img.shields.io/github/license/WenqiOfficial/qobuz-dl-go?style=flat-square)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/WenqiOfficial/qobuz-dl-go?style=flat-square)](https://goreportcard.com/report/github.com/WenqiOfficial/qobuz-dl-go)
[![Downloads](https://img.shields.io/github/downloads/WenqiOfficial/qobuz-dl-go/total?style=flat-square&color=green)](https://github.com/WenqiOfficial/qobuz-dl-go/releases)

🚀 **Qobuz-DL Go** is a high-performance Qobuz music downloader rewritten in Go. It aims to provide faster download speeds, easier deployment, and more powerful features than the original Python version.

This project supports CLI (Command Line Interface) mode and has Web service mode planned (WIP).

## ✨ Features

*   **⚡ High-Performance Concurrent Downloads**: Leverages Go's powerful concurrency model for dramatically faster downloads.
*   **🔓 Automatic Secret Fetching**: Built-in functionality to automatically scrape the latest `App ID` and `Secret` from the Qobuz Web Player - no manual configuration needed.
*   **🎧 Full Audio Quality Support**:
    *   MP3 (320kbps)
    *   FLAC (16-bit / 44.1kHz)
    *   FLAC (24-bit / Hi-Res up to 192kHz)
*   **🏷️ Complete Metadata**:
    *   FLAC: Vorbis Comments tags
    *   MP3: ID3v2 tags
*   **🎨 Cover Art**: Downloads and embeds high-quality album artwork.
*   **🔐 Smart Credential Management**:
    *   Interactive login support.
    *   Command-line credential passing.
    *   Local credential caching (`account.json`).
    *   `--nosave` option for privacy.
*   **🌐 Network Support**: Full HTTP / HTTPS / SOCKS5 proxy support.
*   **🚀 CDN Acceleration**: Built-in CDN proxy for optimized access in China (can be disabled with `--nocdn`).

## 🛠️ Installation & Build

### Prerequisites
*   [Go 1.23+](https://go.dev/dl/)

### Build from Source

1.  Clone the repository:
    ```bash
    git clone https://github.com/WenqiOfficial/qobuz-dl-go.git
    cd qobuz-dl-go
    ```

2.  Install dependencies and build:
    ```bash
    go mod tidy
    go build -o qobuz-dl ./cmd/qobuz-dl
    ```

## 📖 Usage Guide

For detailed usage instructions, please refer to the **[Usage Documentation](docs/usage_en.md)**.

Quick Start:

```bash
# Download an album
./qobuz-dl-go dl https://play.qobuz.com/album/xxx

# Search and download
./qobuz-dl-go search "Mili"

# View help
./qobuz-dl-go --help
```

## 📂 Configuration Files

The program generates the following files in the same directory:

*   `account.json`: Stores encrypted user credentials (Token, UserID, etc.).
*   `config.json`: (Planned) For storing default download path, quality preferences, and other global settings.

## ⚠️ Disclaimer

This project is for technical research and educational purposes only. Do not use for copyright infringement or commercial purposes. Users are solely responsible for any legal consequences arising from the use of this project. Please support legitimate music.

## 👍 Acknowledgments

This project uses or references the following open-source projects and libraries:

*   [qobuz-dl (Python)](https://github.com/vitiko98/qobuz-dl) - Python-based Qobuz downloader
*   [Cobra](https://github.com/spf13/cobra) - Library for building CLI applications
*   [Echo](https://github.com/labstack/echo) - High-performance, minimalist Go web framework
*   [Req](https://github.com/imroc/req) - Simple and elegant Go HTTP client
*   [MPB](https://github.com/vbauerster/mpb) - Multi-progress bar library for terminals
*   [Go-Flac](https://github.com/mewkiz/flac) - FLAC audio decoding library

## 🛠️ Contributing

Contributions of any kind are welcome! Whether it's reporting issues, suggesting features, or submitting code improvements, we appreciate it. Please follow these steps:

1.  Fork this repository.
2.  Create a new branch: `git checkout -b feature/your-feature-name`
3.  Commit your changes: `git commit -m 'Add some feature'`
4.  Push to the branch: `git push origin feature/your-feature-name`
5.  Submit a Pull Request.

Please run `go fmt` before submitting code to maintain consistent code style.

## ⭐ Star History

<a href="https://star-history.com/#WenqiOfficial/qobuz-dl-go&Date">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=WenqiOfficial/qobuz-dl-go&type=Date&theme=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=WenqiOfficial/qobuz-dl-go&type=Date" />
   <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=WenqiOfficial/qobuz-dl-go&type=Date" />
 </picture>
</a>

## 📜 License

This project is licensed under the GPL v3 License. See the [LICENSE](LICENSE) file for details.

## ☕ Support Me

This project uses a self-hosted CDN. If you find it helpful, consider buying me a coffee to support CDN maintenance:

- [Afdian](https://afdian.com/a/wenqiofficial)
- [Donate](https://blog.wenqi.icu/donate)


## ❗ Caution

This document is translated from Chinese to English for broader accessibility. While efforts have been made to ensure accuracy, some nuances may be lost in translation. Please refer to the original Chinese version for the most precise information.Welcome to report any translation issues or suggest improvements.