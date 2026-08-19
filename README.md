# Qobuz-DL Go

[English](README_EN.md) | **中文**

[![Go Version](https://img.shields.io/github/go-mod/go-version/WenqiOfficial/qobuz-dl-go?style=flat-square)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/WenqiOfficial/qobuz-dl-go?style=flat-square&color=blue)](https://github.com/WenqiOfficial/qobuz-dl-go/releases/latest)
[![License](https://img.shields.io/github/license/WenqiOfficial/qobuz-dl-go?style=flat-square)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/WenqiOfficial/qobuz-dl-go?style=flat-square)](https://goreportcard.com/report/github.com/WenqiOfficial/qobuz-dl-go)
[![Downloads](https://img.shields.io/github/downloads/WenqiOfficial/qobuz-dl-go/total?style=flat-square&color=green)](https://github.com/WenqiOfficial/qobuz-dl-go/releases)

🚀 **Qobuz-DL Go** 是一个用 Go 语言重写的高性能 Qobuz 音乐下载器。它旨在提供比原版 Python 项目更快的下载速度、更便捷的部署方式以及更强大的功能支持。

本项目支持 CLI（命令行）模式，并预留了 Web 服务模式（WIP）。

## ✨ 功能特性

*   **⚡ 高性能并发下载**：利用 Go 强大的并发模型，极大提升下载速度。
*   **🔓 自动密钥获取**：内置自动从 Qobuz Web Player 抓取最新 `App ID` 和 `Secret` 的功能，无需手动配置。
*   **🎧 全音质支持**：
    *   MP3 (320kbps)
    *   FLAC (16-bit / 44.1kHz)
    *   FLAC (24-bit / Hi-Res 最高 192kHz)
*   **🏷️ 完善的元数据**：
    *   FLAC: Vorbis Comments 标签
    *   MP3: ID3v2 标签
*   **🎨 封面获取**：下载并嵌入高质量专辑封面。
*   **🔐 智能凭证管理**：
    *   支持交互式登录。
    *   支持通过命令行参数传递账号密码。
    *   支持本地保存凭证 (`account.json`)。
    *   提供 `--nosave` 选项以保护隐私。
*   **🌐 网络支持**：全面支持 HTTP / HTTPS / SOCKS5 代理。
*   **🚀 CDN 加速**：内置国内 CDN 加速，优化访问体验（可通过 `--nocdn` 禁用）。

## 🛠️ 安装与构建

### 前置要求
*   [Go 1.23+](https://go.dev/dl/)

### 源码构建

1.  克隆仓库：
    ```bash
    git clone https://github.com/your-repo/qobuz-dl-go.git
    cd qobuz-dl-go
    ```

2.  整理依赖并构建：
    ```bash
    go mod tidy
    go build -o qobuz-dl-go ./cmd/qobuz-dl
    ```

## 📖 使用指南

详细使用说明请参考 **[使用文档](docs/usage.md)**。

快速开始：

```bash
# 下载专辑
./qobuz-dl-go dl https://play.qobuz.com/album/xxx

# 搜索并下载
./qobuz-dl-go search "Mili"

# 查看帮助
./qobuz-dl-go --help
```

## 📂 配置文件

程序运行后会在同级目录下生成以下文件：

*   `account.json`: 存储加密后的用户凭证（Token、UserID 等）。
*   `config.json`: (计划中) 用于存储默认下载路径、质量偏好等全局配置。

## ⚠️ 免责声明

本项目仅供技术研究和教育用途。请勿用于侵犯版权或商业用途。使用本项目所产生的任何法律后果由使用者自行承担。请支持正版音乐。

## 👍 感谢项目

本项目使用或参考了以下开源项目和库，感谢：

*   [qobuz-dl (Python)](https://github.com/vitiko98/qobuz-dl) - 使用 Python 语言的 Qobuz 下载器
*   [Cobra](https://github.com/spf13/cobra) - 用于构建命令行应用的库
*   [Echo](https://github.com/labstack/echo) - 高性能、极简的 Go Web 框架
*   [Req](https://github.com/imroc/req) - 简洁易用的 Go HTTP 客户端
*   [MPB](https://github.com/vbauerster/mpb) - 终端多进度条库
*   [Go-Flac](https://github.com/mewkiz/flac) - FLAC 音频解码库

## 🛠️ 贡献指南

欢迎任何形式的贡献！无论是报告问题、提出功能请求，还是提交代码改进，都非常感谢。请遵循以下步骤：
1.  Fork 本仓库。

2.  创建新分支：`git checkout -b feature/your-feature-name`

3.  提交更改：`git commit -m 'Add some feature'`

4.  推送到分支：`git push origin feature/your-feature-name`

5.  提交 Pull Request。

请确保在提交代码前运行 `go fmt` 以保持代码风格一致。

## ⭐ Star

<a href="https://star-history.com/#WenqiOfficial/qobuz-dl-go&Date">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=WenqiOfficial/qobuz-dl-go&type=Date&theme=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=WenqiOfficial/qobuz-dl-go&type=Date" />
   <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=WenqiOfficial/qobuz-dl-go&type=Date" />
 </picture>
</a>

## 📜 许可证

本项目采用 GPL v3 许可证。详情请参阅 [LICENSE](LICENSE) 文件。

## ☕ 赞助我

项目中使用了个人自建CDN，如果觉得这个项目对你有帮助，可以考虑用一杯咖啡支持一下CDN的维护：

- [爱发电](https://afdian.com/a/wenqiofficial)

- [投喂窗](https://blog.wenqi.icu/donate)