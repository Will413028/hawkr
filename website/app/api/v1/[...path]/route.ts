import { type NextRequest, NextResponse } from "next/server";
import { apiProxyTarget, isLocalApiProxyEnabled } from "@/lib/api/local-proxy";

type ProxyResponseSnapshot = {
	status: number;
	headers: Array<[string, string]>;
	setCookies: string[];
	body: ArrayBuffer | string | null;
};

type E2ePublicGetCacheEntry = {
	expiresAt: number;
	snapshot: ProxyResponseSnapshot;
};

const E2E_PUBLIC_GET_CACHE_TTL_MS = 15 * 60 * 1000;
const E2E_PUBLIC_GET_CACHE_MAX_ENTRIES = 256;
const E2E_PUBLIC_GET_TIMEOUT_MS = 30_000;
const e2ePublicGetCache = new Map<string, E2ePublicGetCacheEntry>();
const e2ePublicGetInflight = new Map<
	string,
	Promise<ProxyResponseSnapshot | null>
>();

function e2ePublicGetCacheKey(
	req: NextRequest,
	upstreamUrl: string,
): string | null {
	if (process.env.PLAYWRIGHT_E2E_API_PROXY !== "1") return null;
	if (req.method !== "GET") return null;
	if (req.headers.has("authorization") || req.headers.has("cookie"))
		return null;
	if (req.headers.has("x-active-role")) return null;
	if (
		req.headers.has("range") ||
		req.headers.has("if-match") ||
		req.headers.has("if-none-match") ||
		req.headers.has("if-modified-since") ||
		req.headers.has("if-unmodified-since") ||
		req.headers.has("if-range")
	) {
		return null;
	}

	const accept = req.headers.get("accept")?.toLowerCase() ?? "";
	if (accept.includes("text/event-stream")) return null;

	const pathname = new URL(req.url).pathname;
	if (
		pathname !== "/api/v1/public" &&
		!pathname.startsWith("/api/v1/public/")
	) {
		return null;
	}

	if (
		pathname.startsWith("/api/v1/public/auth/") ||
		pathname.startsWith("/api/v1/public/carts") ||
		pathname.includes("/comments") ||
		pathname.includes("/reviews") ||
		pathname.endsWith("/stream")
	) {
		return null;
	}

	const language =
		req.headers.get("accept-language")?.trim().toLowerCase() ?? "";
	return `${upstreamUrl}\naccept:${accept}\naccept-language:${language}`;
}

function snapshotResponse(snapshot: ProxyResponseSnapshot): NextResponse {
	const headers = new Headers(snapshot.headers);
	for (const cookie of snapshot.setCookies) {
		headers.append("set-cookie", cookie);
	}
	const body =
		snapshot.body instanceof ArrayBuffer
			? snapshot.body.slice(0)
			: snapshot.body;
	return new NextResponse(body, {
		status: snapshot.status,
		headers,
	});
}

async function fetchPublicGetSnapshot(
	url: string,
	headers: Headers,
): Promise<ProxyResponseSnapshot | null> {
	let upstream: Response;
	try {
		upstream = await fetch(url, {
			method: "GET",
			headers,
			signal: AbortSignal.timeout(E2E_PUBLIC_GET_TIMEOUT_MS),
		});
	} catch {
		return {
			status: 502,
			headers: [["content-type", "text/plain;charset=UTF-8"]],
			setCookies: [],
			body: "upstream unreachable",
		};
	}

	const contentType = upstream.headers.get("content-type")?.toLowerCase() ?? "";
	if (contentType.includes("text/event-stream")) {
		await upstream.body?.cancel().catch(() => {});
		return null;
	}

	const responseHeaders = new Headers(upstream.headers);
	responseHeaders.delete("content-encoding");
	responseHeaders.delete("transfer-encoding");
	responseHeaders.delete("content-length");
	const setCookies = upstream.headers.getSetCookie();
	responseHeaders.delete("set-cookie");
	if (setCookies.length > 0) {
		await upstream.body?.cancel().catch(() => {});
		return null;
	}

	const nullBody = upstream.status === 204 || upstream.status === 304;
	let body: ArrayBuffer | null;
	try {
		body = nullBody ? null : await upstream.arrayBuffer();
	} catch {
		return {
			status: 502,
			headers: [["content-type", "text/plain;charset=UTF-8"]],
			setCookies: [],
			body: "upstream unreachable",
		};
	}

	return {
		status: upstream.status,
		headers: Array.from(responseHeaders.entries()),
		setCookies,
		body,
	};
}

function readCachedPublicGet(
	key: string,
	now = Date.now(),
): ProxyResponseSnapshot | null {
	const entry = e2ePublicGetCache.get(key);
	if (!entry) return null;
	if (entry.expiresAt <= now) {
		e2ePublicGetCache.delete(key);
		return null;
	}

	e2ePublicGetCache.delete(key);
	e2ePublicGetCache.set(key, entry);
	return entry.snapshot;
}

function writeCachedPublicGet(
	key: string,
	snapshot: ProxyResponseSnapshot,
	now = Date.now(),
): void {
	e2ePublicGetCache.delete(key);
	while (e2ePublicGetCache.size >= E2E_PUBLIC_GET_CACHE_MAX_ENTRIES) {
		const oldestKey = e2ePublicGetCache.keys().next().value;
		if (oldestKey === undefined) break;
		e2ePublicGetCache.delete(oldestKey);
	}
	e2ePublicGetCache.set(key, {
		expiresAt: now + E2E_PUBLIC_GET_CACHE_TTL_MS,
		snapshot,
	});
}

async function getE2ePublicGetSnapshot(
	key: string,
	url: string,
	headers: Headers,
): Promise<ProxyResponseSnapshot | null> {
	const cached = readCachedPublicGet(key);
	if (cached) return cached;

	let pending = e2ePublicGetInflight.get(key);
	if (!pending) {
		pending = fetchPublicGetSnapshot(url, headers).then((snapshot) => {
			const contentType =
				snapshot?.headers
					.find(([name]) => name.toLowerCase() === "content-type")?.[1]
					.toLowerCase() ?? "";
			if (
				snapshot &&
				snapshot.status >= 200 &&
				snapshot.status < 300 &&
				snapshot.setCookies.length === 0 &&
				contentType.includes("application/json")
			) {
				writeCachedPublicGet(key, snapshot);
			}
			return snapshot;
		});
		e2ePublicGetInflight.set(key, pending);
	}

	try {
		return await pending;
	} finally {
		if (e2ePublicGetInflight.get(key) === pending) {
			e2ePublicGetInflight.delete(key);
		}
	}
}

async function proxy(req: NextRequest): Promise<NextResponse> {
	if (!isLocalApiProxyEnabled(req.url)) {
		return new NextResponse(null, { status: 404 });
	}

	const url = apiProxyTarget(req.url);

	const headers = new Headers();
	for (const [k, v] of req.headers.entries()) {
		const lk = k.toLowerCase();
		if (lk === "host" || lk.startsWith("x-nextjs-")) continue;
		headers.set(k, v);
	}

	const body =
		req.method !== "GET" && req.method !== "HEAD"
			? await req.arrayBuffer()
			: undefined;

	const publicGetCacheKey = e2ePublicGetCacheKey(req, url);
	if (publicGetCacheKey) {
		const snapshot = await getE2ePublicGetSnapshot(
			publicGetCacheKey,
			url,
			headers,
		);
		if (snapshot) return snapshotResponse(snapshot);
	}

	let upstream: Response;
	try {
		upstream = await fetch(url, {
			method: req.method,
			headers,
			body,
			redirect: "manual",
			signal: req.signal,
		});
	} catch {
		return new NextResponse("upstream unreachable", { status: 502 });
	}

	const respHeaders = new Headers(upstream.headers);
	respHeaders.delete("content-encoding");
	respHeaders.delete("transfer-encoding");
	respHeaders.delete("content-length");

	const setCookies = upstream.headers.getSetCookie();
	if (setCookies.length > 0) {
		respHeaders.delete("set-cookie");
		for (const cookie of setCookies) {
			respHeaders.append("set-cookie", cookie);
		}
	}

	const upstreamCT = upstream.headers.get("content-type") ?? "";
	if (upstreamCT.includes("text/event-stream")) {
		const upstreamBody = upstream.body;
		let safeBody: ReadableStream<Uint8Array> | null = null;
		if (upstreamBody) {
			const reader = upstreamBody.getReader();
			safeBody = new ReadableStream<Uint8Array>({
				async pull(controller) {
					try {
						const result = await reader.read();
						if (result.done) {
							controller.close();
							return;
						}
						controller.enqueue(result.value);
					} catch {
						controller.close();
					}
				},
				cancel(reason) {
					void reader.cancel(reason).catch(() => {});
				},
			});
		}

		return new NextResponse(safeBody, {
			status: upstream.status,
			headers: respHeaders,
		});
	}

	const nullBody = upstream.status === 204 || upstream.status === 304;
	const respBody = nullBody ? null : await upstream.arrayBuffer();
	return new NextResponse(respBody, {
		status: upstream.status,
		headers: respHeaders,
	});
}

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
export const OPTIONS = proxy;
