# Qobuz API Proxy (Cloudflare Workers)

Cloudflare Workers 代理，转发 Qobuz API 和 Web Player 请求。

## 支持的域名

| 代理域名 | 目标 | 用途 |
|----------|------|------|
| `qobuz.wenqi.icu` | `www.qobuz.com` | API 请求 |
| `play-qobuz.wenqi.icu` | `play.qobuz.com` | Web Player (获取 App ID/Secret) |

## 部署步骤

### 1. 安装 Wrangler CLI

```bash
npm install -g wrangler
```

### 2. 登录 Cloudflare

```bash
wrangler login
```

### 3. 部署

```bash
cd workers/qobuz-proxy
wrangler deploy
```

### 4. 配置自定义域名

在 Cloudflare Dashboard 中为每个域名配置路由：

**方法 A: Workers Routes**
1. 进入 Cloudflare Dashboard → 选择域名 (wenqi.icu)
2. Workers Routes → 添加两条路由：
   - Pattern: `qobuz.wenqi.icu/*` → Worker: `qobuz-proxy`
   - Pattern: `play-qobuz.wenqi.icu/*` → Worker: `qobuz-proxy`

**方法 B: DNS + Custom Domains**
1. Workers & Pages → qobuz-proxy → Settings → Triggers
2. Add Custom Domain:
   - `qobuz.wenqi.icu`
   - `play-qobuz.wenqi.icu`

## API 对应关系

| 原始 | 代理后 |
|------|--------|
| `https://www.qobuz.com/api.json/0.2/...` | `https://qobuz.wenqi.icu/api.json/0.2/...` |
| `https://play.qobuz.com/login` | `https://play-qobuz.wenqi.icu/login` |
| `https://play.qobuz.com/resources/.../bundle.js` | `https://play-qobuz.wenqi.icu/resources/.../bundle.js` |

## 健康检查

```bash
curl https://qobuz.wenqi.icu/health
curl https://play-qobuz.wenqi.icu/health
```

## 本地开发

```bash
wrangler dev
```

会在 `http://localhost:8787` 启动本地服务器。
