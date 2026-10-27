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
