Qobuz-DL Go 重构计划
🏗️ Phase 1: 项目基建
[ ] 初始化工程：go mod init qobuz-dl-go。

[ ] 目录结构设计：

/cmd: 存放 CLI 入口 (cobra)。

/internal/engine: 核心下载与解密逻辑。

/internal/api: Qobuz 官方接口封装。

/internal/server: Web 服务模块 (gin/echo)。

/web: 前端静态资源 (Vue3/Vite)。

[ ] 依赖引入：安装 Cobra, Gin, Resty/Httpx, Crypto 等核心库。

🧠 Phase 2: 核心引擎 (Engine) - 逻辑复现
[ ] 鉴权模块：实现 AppID/Secret 换取 UserToken 的逻辑。

[ ] 元数据处理：重写 Track/Album/Playlist 的信息获取函数。

[ ] 解密核心：

复现原项目的 AES-CBC/CTR 解密算法。

[最优解] 实现 io.ReadCloser 接口的流式解密器，确保数据边读边解。

[ ] 文件封装：实现 FLAC 标签写入与封面嵌入。

💻 Phase 3: CLI 模式开发
[ ] 配置管理：实现 $HOME/.qobuz.yaml 的读取与持久化（仅本地模式）。

[ ] 下载指令：实现单曲、专辑、歌单下载命令。

[ ] 进度反馈：引入 mpb 等库实现多线程下载进度条。

🌐 Phase 4: Web 服务模式 (无状态重构)
[ ] 流式转发 (Streaming Proxy)：

核心逻辑：Qobuz API -> Go Engine (解密) -> HTTP Response (io.Copy)。

[ ] 无状态鉴权：设计中间件，从 Request Header 动态提取 Token 注入 Engine。

[ ] 前端实现：

用户登录态保存至 LocalStorage。

实现调用后端接口的下载队列管理。

🚀 Phase 5: 优化与打包
[ ] 并发控制：使用 errgroup 限制最大并发下载数。

[ ] 静态资源嵌入：使用 go:embed 将前端页面打包进单个二进制文件。

[ ] 跨平台编译：编写 Makefile 支持 Windows/Linux/MacOS 一键编译。