# 📖 使用指南

[English](usage_en.md) | **中文**

## 1. 基础下载

下载单曲或专辑，只需提供 URL 或 ID：

```bash
# 下载专辑
./qobuz-dl-go dl https://play.qobuz.com/album/j3wq4jjuhznjb

# 下载单曲
./qobuz-dl-go dl https://play.qobuz.com/track/123456
```

## 2. 身份验证

程序会优先读取本地缓存的 `account.json`。如果没有缓存，可以通过以下方式登录：

**交互式登录（推荐）**：
直接运行下载命令，程序会提示输入邮箱和密码。

**命令行参数登录**：
```bash
./qobuz-dl-go dl <url> --email your@email.com --password yourpassword
```

**使用 Token 登录**：
```bash
./qobuz-dl-go dl <url> --token <user-auth-token>
```

## 3. 下载质量

使用 `-q` 或 `--quality` 参数指定音质：

*   `5`: MP3 320kbps
*   `6`: FLAC Lossless (16-bit / 44.1kHz) **(默认)**
*   `7`: FLAC 24-bit / 96kHz
*   `27`: FLAC 24-bit / 192kHz (最高音质)

```bash
# 下载最高音质
./qobuz-dl-go dl <url> -q 27
```

## 4. 代理设置

程序会自动使用系统环境变量中的代理设置（`HTTP_PROXY`、`HTTPS_PROXY`、`ALL_PROXY`）。

也可以通过 `--proxy` 参数手动指定代理：

```bash
# HTTP 代理
./qobuz-dl-go dl <url> --proxy http://127.0.0.1:7890

# SOCKS5 代理
./qobuz-dl-go dl <url> --proxy socks5://127.0.0.1:1080
```

## 5. CDN 加速

默认启用 CDN 加速，优化国内访问速度。如需禁用：

```bash
./qobuz-dl-go dl <url> --nocdn
```

## 6. 搜索功能

使用 `search` 命令进行搜索并下载：

```bash
# 搜索专辑（默认）
./qobuz-dl-go search "Mili"

# 搜索单曲
./qobuz-dl-go search "world.execute(me)" -T track

# 搜索艺术家
./qobuz-dl-go search "Mili" -T artist
```

搜索结果会以交互式列表呈现，选择后自动下载。可提前通过下载质量和代理参数设置下载偏好。

## 7. 并发下载

使用 `--threads` 或 `-n` 参数设置并发下载线程数（默认 3，最大 10）：

```bash
./qobuz-dl-go dl <url> -n 5
```

## 8. 其他选项

*   `--output`, `-o`: 指定输出目录（默认为当前目录）。
*   `--threads`, `-n`: 并发下载线程数（1-10，默认 3）。
*   `--nosave`: 不将本次登录的凭证保存到本地 `account.json`。
*   `--nocdn`: 禁用 CDN 加速，直连 Qobuz 服务器。
*   `--app-id`, `--app-secret`: 手动指定 App 已知的 ID 和密钥（通常不需要，程序会自动获取）。

## 9. 自动更新

检查并更新到最新版本：

```bash
./qobuz-dl-go update
```

## 10. Shell 补全

生成 Shell 自动补全脚本：

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

## 📂 配置文件

程序运行后会在同级目录下生成以下文件：

*   `account.json`: 存储加密后的用户凭证（Token、UserID 等）。
*   `config.json`: (计划中) 用于存储默认下载路径、质量偏好等全局配置。
