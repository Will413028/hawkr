const DEV_GATEWAY_BASE = "http://localhost:8080/api/v1";

interface ApiProxyEnvironment {
	NODE_ENV?: string;
	API_BASE_URL?: string;
	PLAYWRIGHT_E2E_API_PROXY?: string;
}

function isLoopbackHostname(hostname: string): boolean {
	return (
		hostname === "localhost" ||
		hostname === "127.0.0.1" ||
		hostname === "::1" ||
		hostname === "[::1]"
	);
}

export function isLocalApiProxyEnabled(
	requestUrl: string,
	env: ApiProxyEnvironment = process.env,
): boolean {
	if (env.NODE_ENV === "development") return true;
	if (env.NODE_ENV !== "production") return false;
	if (env.PLAYWRIGHT_E2E_API_PROXY !== "1") return false;

	try {
		return isLoopbackHostname(new URL(requestUrl).hostname.toLowerCase());
	} catch {
		return false;
	}
}

interface UpstreamParts {
	origin: string;
	apiPath: string;
	rootPath: string;
}

/** 單體入口的組成：origin，加上 `API_BASE_URL` 去掉結尾斜線的路徑（`/api/v1`）與去掉 `/api/v1` 的入口根路徑。 */
function upstreamParts(apiBaseUrl: string): UpstreamParts {
	const base = new URL(apiBaseUrl);
	const apiPath = base.pathname.replace(/\/+$/, "");
	return {
		origin: base.origin,
		apiPath,
		rootPath: apiPath.replace(/\/api\/v1$/, ""),
	};
}

/**
 * 同源反代的路徑對照，只在這裡定義：瀏覽器打的前綴（`source`）對到單體上的路徑前綴（`upstream`）。
 * route handler 的上游網址（`*ProxyTarget`）與 `next.config.ts` 的 rewrites（`apiProxyRewrites`）都由此推出。
 * Connect 不在 /api/v1 底下（lib/connect/transport.ts 的 FIRST_PARTY_CONNECT_PATH），登入流程在單體根目錄的
 * `/api/auth/*`（lib/auth/oauth-start.ts）。
 */
const PROXY_ROUTES = {
	connect: {
		source: "/api/connect",
		upstream: (p: UpstreamParts) => p.rootPath,
	},
	auth: { source: "/api/auth", upstream: () => "/api/auth" },
	api: { source: "/api/v1", upstream: (p: UpstreamParts) => p.apiPath },
} as const;

function proxyTarget(
	route: (typeof PROXY_ROUTES)[keyof typeof PROXY_ROUTES],
	requestUrl: string,
	apiBaseUrl: string,
): string {
	const incoming = new URL(requestUrl);
	const parts = upstreamParts(apiBaseUrl);
	const suffix = incoming.pathname.slice(route.source.length);
	const upstream = new URL(`${route.upstream(parts)}${suffix}`, parts.origin);
	upstream.search = incoming.search;
	return upstream.toString();
}

export function apiProxyTarget(
	requestUrl: string,
	apiBaseUrl = process.env.API_BASE_URL || DEV_GATEWAY_BASE,
): string {
	return proxyTarget(PROXY_ROUTES.api, requestUrl, apiBaseUrl);
}

/** `/api/connect/{service}/{method}` → 入口根目錄的 `/{service}/{method}`。 */
export function connectProxyTarget(
	requestUrl: string,
	apiBaseUrl = process.env.API_BASE_URL || DEV_GATEWAY_BASE,
): string {
	return proxyTarget(PROXY_ROUTES.connect, requestUrl, apiBaseUrl);
}

/** `/api/auth/{provider}/start`、`/callback` 原路徑。 */
export function authProxyTarget(
	requestUrl: string,
	apiBaseUrl = process.env.API_BASE_URL || DEV_GATEWAY_BASE,
): string {
	return proxyTarget(PROXY_ROUTES.auth, requestUrl, apiBaseUrl);
}

/**
 * 本機的 route handler 是不是這個 build 的反代：與 `isLocalApiProxyEnabled` 同一條規則的 build 期版本
 * （`next dev`，或帶 `PLAYWRIGHT_E2E_API_PROXY=1` 的 e2e build；後者執行期再限 loopback）。
 * 是的話 rewrites 不啟用，請求進 `app/api/{connect,auth,v1}`。
 */
export function isLocalApiProxyBuild(
	env: Pick<
		ApiProxyEnvironment,
		"NODE_ENV" | "PLAYWRIGHT_E2E_API_PROXY"
	> = process.env,
): boolean {
	return env.NODE_ENV === "development" || env.PLAYWRIGHT_E2E_API_PROXY === "1";
}

/**
 * 部署環境的同源反代：`rewrites().beforeFiles` 把 `/api/{connect,auth,v1}/*` 轉給單體，單體位址由
 * `API_BASE_URL` 推得。本機反代的 build（`isLocalApiProxyBuild`）回空陣列。啟用時 `API_BASE_URL`
 * 必須是絕對的 http(s) 網址，否則丟錯讓 build 失敗。
 */
export function apiProxyRewrites(
	env: ApiProxyEnvironment = process.env,
): { source: string; destination: string }[] {
	if (isLocalApiProxyBuild(env)) return [];
	const raw = env.API_BASE_URL?.trim();
	let parts: UpstreamParts;
	try {
		if (!raw) throw new Error("unset");
		const base = new URL(raw);
		if (base.protocol !== "https:" && base.protocol !== "http:") {
			throw new Error("not http(s)");
		}
		parts = upstreamParts(raw);
	} catch {
		throw new Error(
			`API_BASE_URL must be an absolute http(s) URL (e.g. https://api.<your-domain>/api/v1) for the /api rewrites (got "${raw ?? ""}")`,
		);
	}
	return Object.values(PROXY_ROUTES).map((route) => ({
		source: `${route.source}/:path*`,
		destination: `${parts.origin}${route.upstream(parts)}/:path*`,
	}));
}

/**
 * 本機代理的轉送：請求的 header（Cookie、Authorization 等）原樣送給單體；`redirect: "manual"` 讓上游的
 * 3xx 連同 `Location` 原樣交給瀏覽器（登入流程的 302 要由瀏覽器去 Google 或 LINE、再回來；fetch 預設會自己
 * 跟隨，OAuth 的 302 就到不了瀏覽器）；回應的多個 Set-Cookie 逐筆傳回（`new Headers(upstream.headers)` 會
 * 把它們併成一個以逗號串起的值，cookie 就壞了）。回應以串流傳回，Connect 的 server streaming 也逐筆送達。
 */
export async function passThroughToUpstream(
	req: Request,
	target: string,
): Promise<Response> {
	const headers = new Headers();
	for (const [k, v] of req.headers.entries()) {
		const lk = k.toLowerCase();
		if (lk === "host" || lk.startsWith("x-nextjs-")) continue;
		headers.set(k, v);
	}
	let upstream: Response;
	try {
		upstream = await fetch(target, {
			method: req.method,
			headers,
			body:
				req.method === "GET" || req.method === "HEAD"
					? undefined
					: await req.arrayBuffer(),
			redirect: "manual",
			signal: req.signal,
		});
	} catch {
		return new Response("upstream unreachable", { status: 502 });
	}
	const respHeaders = new Headers(upstream.headers);
	// Node 的 fetch 已經解壓縮，去掉過時的編碼與長度，瀏覽器才不會再解一次。
	respHeaders.delete("content-encoding");
	respHeaders.delete("transfer-encoding");
	respHeaders.delete("content-length");
	const setCookies = upstream.headers.getSetCookie();
	respHeaders.delete("set-cookie");
	for (const cookie of setCookies) respHeaders.append("set-cookie", cookie);
	return new Response(upstream.body, {
		status: upstream.status,
		headers: respHeaders,
	});
}
