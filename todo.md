# Qobuz-DL Go 重构计划

## 🏗️ Phase 1: 基础设施 (已完成)

- [x] 项目初始化 (`go mod init`)
- [x] 目录结构 (`cmd`, `internal/engine`, `internal/api`, `internal/server`)
- [x] 依赖管理 (`cobra`, `echo`, `req/v3`, `mpb`, `go-flac`)

## 🧠 Phase 2: 核心引擎 (已完成)

- [x] 认证: 登录及 `AppID/Secret` 自动获取
- [x] API 客户端: 基于 `req/v3` 的客户端，支持 MD5 签名生成
- [x] 下载器: 高性能并发下载 (流式 & 文件)
- [x] 元数据: FLAC Vorbis Comments 标签写入
- [x] 封面: 高质量封面下载与嵌入
- [x] 代理支持: HTTP/SOCKS5 代理集成
- [x] 配置管理: JSON 配置与账户持久化

## 💻 Phase 3: CLI (已完成)

- [x] 基于 `cobra` 的命令行界面
- [x] 基于 `mpb` 的进度条
- [x] 交互式认证回退
- [x] 音质/输出/代理参数

## 🌐 Phase 4: Web 界面 (待开发)

- [ ] Web 服务器集成 (`echo`)
- [ ] 前端实现 (Vue3/React)
- [ ] 搜索和下载队列 API

## 🚀 优化与清理

- [x] 重构 `main.go`
- [x] 修复 FLAC 标签语法错误
- [x] 强制最大质量封面
- [x] 文件名安全字符清理 (sanitize)
- [x] 代码注释整理
- [ ] 完善单元测试

---

## 📊 与 Python 版本对比 - 未实现功能

### 🔴 高优先级 (核心功能缺失)

| 功能 | Python 实现 | Go 状态 | 说明 |
|------|------------|---------|------|
| 搜索功能 | `search_by_type()` | ❌ 未实现 | 支持搜索专辑/曲目/艺术家/播放列表 |
| 播放列表下载 | `get_plist_meta()` | ❌ 未实现 | 下载整个播放列表 |
| 艺术家下载 | `get_artist_meta()` | ❌ 未实现 | 下载艺术家所有专辑 |
| 厂牌下载 | `get_label_meta()` | ❌ 未实现 | 下载厂牌下所有专辑 |
| MP3 标签 | `tag_mp3()` | ✅ 已实现 (ID3v2) | MP3 格式的 ID3 标签写入 |

### 🟡 中优先级 (体验优化)

| 功能 | Python 实现 | Go 状态 | 说明 |
|------|------------|---------|------|
| 交互式选择 | `interactive()` | ❌ 未实现 | 使用 pick 库的交互式搜索选择 |
| Lucky 模式 | `lucky_mode()` | ❌ 未实现 | 搜索并自动下载第一个结果 |
| 下载数据库 | `db.py` | ❌ 未实现 | SQLite 数据库避免重复下载 |
| 批量文本下载 | `download_from_txt_file()` | ❌ 未实现 | 从文本文件读取 URL 批量下载 |
| 自定义文件名 | `folder_format`/`track_format` | ❌ 未实现 | 可配置的文件夹/文件命名模板 |
| M3U 生成 | `make_m3u()` | ❌ 未实现 | 为播放列表生成 M3U 文件 |

### 🟢 低优先级 (高级功能)

| 功能 | Python 实现 | Go 状态 | 说明 |
|------|------------|---------|------|
| 智能专辑过滤 | `smart_discography_filter()` | ❌ 未实现 | 过滤重复/精选集/现场专辑 |
| 质量降级回退 | `quality_fallback` | ✅ 已实现（客户端顺序回退 + MIME 确定扩展名） | 当请求质量不可用时自动降级并使用实际格式 |
| Last.fm 歌单 | `download_lastfm_pl()` | ❌ 未实现 | 支持 Last.fm 歌单导入 |
| 仅专辑过滤 | `albums_only` | ❌ 未实现 | 忽略单曲和 EP |
| 配置文件重置 | `-r` / `--reset` | ❌ 未实现 | 重新配置账户信息 |
| 多碟专辑 | `Disc N` 子目录 | ⚠️ 部分 | 多碟专辑分目录 (已有 MediaNumber 字段) |
| 配置显示 | `--show-config` | ❌ 未实现 | 显示当前配置 |

### 📝 模型字段补充 (API)

以下字段在 Python 版中使用但 Go 版 `models.go` 尚未定义:

```go
// TrackMetadata 需要补充:
Composer struct { Name string } `json:"composer"`
Copyright string                 `json:"copyright"`
Work      string                 `json:"work"`  // 古典音乐作品名
ISRC      string                 `json:"isrc"`

// AlbumMetadata 需要补充:
Label       struct { Name string } `json:"label"`
TracksCount int                    `json:"tracks_count"`
GenresList  []string               `json:"genres_list"`
Copyright   string                 `json:"copyright"`
Streamable  bool                   `json:"streamable"`
ReleaseType string                 `json:"release_type"` // album/single/ep
Goodies     []Goodie               `json:"goodies"`      // PDF 书签等
```

---

## 🐛 已知问题 & 鲁棒性改进

### ✅ 已修复

- [x] 文件名包含非法字符 → 添加 `sanitizeFilename()` 函数
- [x] MP3 标签写入 → 新增 ID3v2 支持
- [x] 文件扩展名使用 MIME 类型判断 → 质量为 MP3 不再写成 .flac

### ⚠️ 待修复

- [ ] **错误重试机制**: 网络请求失败时应支持重试
- [ ] **下载断点续传**: 大文件下载中断后应能继续
- [ ] **输入验证**: 对用户输入的 URL/ID 进行更严格验证
- [ ] **日志系统**: 使用结构化日志替代 fmt.Printf
- [ ] **Context 取消**: 确保所有长时间操作都响应 context 取消
- [ ] **并发限制**: 专辑下载时限制并发数量避免 API 限流
- [ ] **临时文件清理**: 下载失败时清理不完整的临时文件
- [ ] **权限检查**: 写入目录前检查写权限

---

## 🎯 下一步开发计划

1. **v0.2.0**: 实现搜索功能和交互式模式
2. **v0.3.0**: 支持播放列表/艺术家/厂牌下载
3. **v0.4.0**: 添加下载数据库和断点续传
4. **v0.5.0**: 完善 Web 界面
5. **v1.0.0**: 功能完整版，全面测试
