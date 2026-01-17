# Qobuz-DL Go Refactoring Plan

## 🏗️ Phase 1: Infrastructure (Completed)
- [x] Project Initialization (`go mod init`).
- [x] Directory Structure (`cmd`, `internal/engine`, `internal/api`, `internal/server`).
- [x] Dependencies (`cobra`, `echo`, `req/v3`, `mpb`, `go-flac`).

## 🧠 Phase 2: Core Engine (Completed)
- [x] Authentication: Login and `AppID/Secret` harvesting.
- [x] API Client: `req/v3` based client with MD5 signature generation.
- [x] Downloader: High-performance concurrent downloading (Stream & File).
- [x] Metadata: FLAC Vorbis Comments tagging.
- [x] Cover Art: High-quality cover art downloading and embedding.
- [x] Proxy Support: HTTP/SOCKS5 proxy integration.
- [x] Configuration: JSON based config and account persistence.

## 💻 Phase 3: CLI (Completed)
- [x] Command Line Interface with `cobra`.
- [x] Progress Bars with `mpb`.
- [x] interactive Auth fallback.
- [x] Flags for quality, output, proxy.

## 🌐 Phase 4: Web Interface (Pending)
- [ ] Web Server Integration (`echo`).
- [ ] Frontend Implementation (Vue3/React).
- [ ] API Endpoints for Search and Download queue.

## 🚀 Optimization & Clean-up
- [x] Refactor `main.go`.
- [x] Fix FLAC tagging syntax errors.
- [x] Max quality cover art forced.
- [ ] Comprehensive Testing.
