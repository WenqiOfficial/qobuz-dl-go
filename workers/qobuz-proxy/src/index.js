/**
 * Qobuz API Proxy for Cloudflare Workers
 * 
 * This worker proxies requests to Qobuz API, useful for:
 * - Bypassing regional restrictions
 * - Providing a stable endpoint for the Go client
 * 
 * Deploy: wrangler deploy
 * 
 * Domains:
 *   qobuz.wenqi.icu      -> www.qobuz.com (API)
 *   play-qobuz.wenqi.icu -> play.qobuz.com (Web Player for secrets)
 * 
 * The worker auto-detects which domain is being accessed.
 */

// CORS headers for browser access (if needed)
const CORS_HEADERS = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type, X-App-Id, X-User-Auth-Token',
};

// Domain mappings
const DOMAIN_MAP = {
  'qobuz.wenqi.icu': {
    host: 'www.qobuz.com',
    origin: 'https://www.qobuz.com',
  },
  'play-qobuz.wenqi.icu': {
    host: 'play.qobuz.com',
    origin: 'https://play.qobuz.com',
  },
};

export default {
  async fetch(request, env, ctx) {
    // Handle CORS preflight
    if (request.method === 'OPTIONS') {
      return new Response(null, {
        status: 204,
        headers: CORS_HEADERS,
      });
    }

    const url = new URL(request.url);
    const hostname = url.hostname;

    // Health check endpoint
    if (url.pathname === '/' || url.pathname === '/health') {
      return new Response(JSON.stringify({
        status: 'ok',
        service: 'qobuz-api-proxy',
        hostname: hostname,
        timestamp: new Date().toISOString(),
      }), {
        headers: {
          'Content-Type': 'application/json',
          ...CORS_HEADERS,
        },
      });
    }

    // Get target configuration based on hostname
    const config = DOMAIN_MAP[hostname];
    if (!config) {
      // Default to www.qobuz.com for unknown hostnames
      return proxyRequest(request, url, 'www.qobuz.com', 'https://www.qobuz.com');
    }

    return proxyRequest(request, url, config.host, config.origin);
  },
};

async function proxyRequest(request, url, targetHost, targetOrigin) {
  try {
    // Build target URL
    const targetUrl = new URL(url.pathname + url.search, targetOrigin);

    // Clone headers, modify Host
    const headers = new Headers(request.headers);
    headers.set('Host', targetHost);
    headers.delete('CF-Connecting-IP');
    headers.delete('CF-IPCountry');
    headers.delete('CF-Ray');
    headers.delete('CF-Visitor');

    // Forward the request
    const response = await fetch(targetUrl.toString(), {
      method: request.method,
      headers: headers,
      body: request.method !== 'GET' && request.method !== 'HEAD' 
        ? await request.arrayBuffer() 
        : undefined,
    });

    // Clone response and add CORS headers
    const responseHeaders = new Headers(response.headers);
    Object.entries(CORS_HEADERS).forEach(([key, value]) => {
      responseHeaders.set(key, value);
    });

    return new Response(response.body, {
      status: response.status,
      statusText: response.statusText,
      headers: responseHeaders,
    });

  } catch (error) {
    return new Response(JSON.stringify({
      error: 'Proxy error',
      message: error.message,
    }), {
      status: 502,
      headers: {
        'Content-Type': 'application/json',
        ...CORS_HEADERS,
      },
    });
  }
}
