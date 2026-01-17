# Qobuz-DL Go

[![Go Version](https://img.shields.io/github/go-mod/go-version/WenqiOfficial/qobuz-dl-go?style=flat-square)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/WenqiOfficial/qobuz-dl-go?style=flat-square&color=blue)](https://github.com/WenqiOfficial/qobuz-dl-go/releases/latest)
[![License](https://img.shields.io/github/license/WenqiOfficial/qobuz-dl-go?style=flat-square)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/WenqiOfficial/qobuz-dl-go?style=flat-square)](https://goreportcard.com/report/github.com/WenqiOfficial/qobuz-dl-go)
[![Downloads](https://img.shields.io/github/downloads/WenqiOfficial/qobuz-dl-go/total?style=flat-square&color=green)](https://github.com/WenqiOfficial/qobuz-dl-go/releases)

🚀 **Qobuz-DL Go** 是一个用 Go 语言重写的高性能 Qobuz 音乐下载器。它旨在提供比原�?Python 项目更快的下载速度、更便捷的部署方式以及更强大的功能支持�?

本项目支�?CLI（命令行）模式，并预留了 Web 服务模式（WIP）�?

## �?功能特�?

*   **�?高性能并发下载**：利�?Go 强大的并发模型，极大提升下载速度�?
*   **🔓 自动密钥获取**：内置自动从 Qobuz Web Player 抓取最�?`App ID` �?`Secret` 的功能，无需手动配置�?
*   **🎧 全音质支�?*�?
    *   MP3 (320kbps)
    *   FLAC (16-bit / 44.1kHz)
    *   FLAC (24-bit / Hi-Res 最�?192kHz)
*   **🏷�?完善的元数据**：自动通过 Vorbis Comments 写入完整的歌曲标签（标题、艺术家、专辑、曲号等）�?
*   **🎨 封面获取**：尝试下载并嵌入专辑封面�?
*   **🔐 智能凭证管理**�?
    *   支持交互式登录�?
    *   支持通过命令行参数传递账号密码�?
    *   支持本地保存凭证 (`account.json`)�?
    *   提供 `--nosave` 选项以保护隐私�?
*   **🌐 网络支持**：全面支�?HTTP / HTTPS / SOCKS5 代理�?

## 🛠�?安装与构�?

### 前置要求
*   [Go 1.23+](https://go.dev/dl/)

### 源码构建

1.  克隆仓库�?
    ```bash
    git clone https://github.com/your-repo/qobuz-dl-go.git
    cd qobuz-dl-go
    ```

2.  整理依赖并构建：
    ```bash
    go mod tidy
    go build -o qobuz-dl.exe ./cmd/qobuz-dl
    ```

## 📖 使用指南

### 1. 基础下载

下载单曲或专辑，只需提供 URL �?ID�?

```bash
# 下载专辑
./qobuz-dl dl https://play.qobuz.com/album/j3wq4jjuhznjb

# 下载单曲
./qobuz-dl dl https://play.qobuz.com/track/123456
```

### 2. 身份验证

程序会优先读取本地缓存的 `account.json`。如果没有缓存，可以通过以下方式登录�?

**交互式登录（推荐�?*�?
直接运行下载命令，程序会提示输入邮箱和密码�?

**命令行参数登�?*�?
```bash
./qobuz-dl dl <url> --email your@email.com --password yourpassword
```

**使用 Token 登录**�?
```bash
./qobuz-dl dl <url> --token <user-auth-token>
```

### 3. 下载质量

使用 `-q` �?`--quality` 参数指定音质�?

*   `5`: MP3 320kbps
*   `6`: FLAC Lossless (16-bit / 44.1kHz) **(默认)**
*   `7`: FLAC 24-bit / 96kHz
*   `27`: FLAC 24-bit / 192kHz (最高音�?

```bash
# 下载最高音�?
./qobuz-dl dl <url> -q 27
```

### 4. 代理设置

程序会自动使用系统环境变量中的代理设置（`HTTP_PROXY`、`HTTPS_PROXY`、`ALL_PROXY`）�?

也可以通过 `--proxy` 参数手动指定代理�?

```bash
# HTTP 代理
./qobuz-dl dl <url> --proxy http://127.0.0.1:7890

# SOCKS5 代理
./qobuz-dl dl <url> --proxy socks5://127.0.0.1:1080
```

### 5. 其他选项

*   `--output`, `-o`: 指定输出目录（默认为当前目录）�?
*   `--nosave`: 不将本次登录的凭证保存到本地 `account.json`�?
*   `--app-id`, `--app-secret`: 手动指定 App 已知�?ID 和密钥（通常不需要，程序会自动获取）�?

## 📂 配置文件

程序运行后会在同级目录下生成以下文件�?

*   `account.json`: 存储加密后的用户凭证（Token、UserID 等）�?
*   `config.json`: (计划�? 用于存储默认下载路径、质量偏好等全局配置�?

## ⚠️ 免责声明

本项目仅供技术研究和教育用途。请勿用于侵犯版权或商业用途。使用本项目所产生的任何法律后果由使用者自行承担。请支持正版音乐�?

## 👍 感谢项目

本项目使用或参考了以下开源项目和库，感谢�?

*   [qobuz-dl (Python)](https://github.com/vitiko98/qobuz-dl) - 使用 Python 语言�?Qobuz 下载�?
*   [Cobra](https://github.com/spf13/cobra) - 用于构建命令行应用的�?
*   [Echo](https://github.com/labstack/echo) - 高性能、极简�?Go Web 框架
*   [Req](https://github.com/imroc/req) - 简洁易用的 Go HTTP 客户�?
*   [MPB](https://github.com/vbauerster/mpb) - 终端多进度条�?
*   [Go-Flac](https://github.com/mewkiz/flac) - FLAC 音频解码�?

## 🛠�?贡献指南

欢迎任何形式的贡献！无论是报告问题、提出功能请求，还是提交代码改进，都非常感谢。请遵循以下步骤�?
1.  Fork 本仓库�?

2.  创建新分支：`git checkout -b feature/your-feature-name`

3.  提交更改：`git commit -m 'Add some feature'`

4.  推送到分支：`git push origin feature/your-feature-name`

5.  提交 Pull Request�?

请确保在提交代码前运�?`go fmt` 以保持代码风格一致�?

## �?Star History

<a href="https://star-history.com/#WenqiOfficial/qobuz-dl-go&Date">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=WenqiOfficial/qobuz-dl-go&type=Date&theme=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=WenqiOfficial/qobuz-dl-go&type=Date" />
   <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=WenqiOfficial/qobuz-dl-go&type=Date" />
 </picture>
</a>

## 📜 许可�?

本项目采�?GPL v3 许可证。详情请参阅 [LICENSE](LICENSE) 文件�?
