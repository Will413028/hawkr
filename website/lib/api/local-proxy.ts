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

export function apiProxyTarget(
	requestUrl: string,
	apiBaseUrl = process.env.API_BASE_URL || DEV_GATEWAY_BASE,
): string {
	const incoming = new URL(requestUrl);
	const base = new URL(apiBaseUrl);
	const suffix = incoming.pathname.replace(/^\/api\/v1(?=\/|$)/, "");
	const basePath = base.pathname.replace(/\/+$/, "");
	const upstream = new URL(`${basePath}${suffix}`, base.origin);
	upstream.search = incoming.search;
	return upstream.toString();
}

/**
 * Connect 第一方路徑的上游網址：`/api/connect/{service}/{method}` → 入口根目錄的 `/{service}/{method}`
 * （Connect 不在 /api/v1 底下；lib/connect/transport.ts 的 FIRST_PARTY_CONNECT_PATH）。
 */
export function connectProxyTarget(
	requestUrl: string,
	apiBaseUrl = process.env.API_BASE_URL || DEV_GATEWAY_BASE,
): string {
	const incoming = new URL(requestUrl);
	const base = new URL(apiBaseUrl);
	const suffix = incoming.pathname.replace(/^\/api\/connect(?=\/|$)/, "");
	const basePath = base.pathname.replace(/\/+$/, "").replace(/\/api\/v1$/, "");
	const upstream = new URL(`${basePath}${suffix}`, base.origin);
	upstream.search = incoming.search;
	return upstream.toString();
}

/**
 * 單體發起登入的上游網址：`/api/auth/{provider}/start`、`/callback` 原路徑（登入流程在單體的根目錄
 * `/api/auth/*`，不在 /api/v1 底下；lib/auth/oauth-start.ts）。
 */
export function authProxyTarget(
	requestUrl: string,
	apiBaseUrl = process.env.API_BASE_URL || DEV_GATEWAY_BASE,
): string {
	const incoming = new URL(requestUrl);
	const base = new URL(apiBaseUrl);
	const upstream = new URL(incoming.pathname, base.origin);
	upstream.search = incoming.search;
	return upstream.toString();
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
