// Local-dev opt-in proxy: forwards /api/v1/* to the local monolith (API_BASE_URL) and
// explicitly passes all response headers — including Set-Cookie — back to the
// browser. next.config rewrites silently drop Set-Cookie from external origins.
//
// OPT-IN: only reached when the browser calls a RELATIVE /api/v1 (set
// NEXT_PUBLIC_API_BASE_URL=/api/v1 in .env.local). The default is the absolute
// gateway URL → browser hits test-api directly (parity mode) and never this
// route. Use proxy mode to sidestep cross-origin friction (CORS / cookie domain
// / VPN / firewall / content-blocker / self-signed cert) on a flaky localdev
// network. See .env.example.
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

// The full Playwright suite shares one rate-limited test gateway. Repeated
// anonymous reference-data reads across fresh browser contexts can otherwise
// exhaust that gateway even with a single Playwright worker. This cache exists
// only in the explicitly opted-in, loopback-only E2E proxy; deployed Workers
// never set PLAYWRIGHT_E2E_API_PROXY and therefore never use it.
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
	// Browser API calls send application/json, while Playwright's
	// APIRequestContext fixture discovery defaults to */*. Both are eligible;
	// only an actual JSON upstream response is persisted below.
	if (accept.includes("text/event-stream")) return null;

	const pathname = new URL(req.url).pathname;
	if (
		pathname !== "/api/v1/public" &&
		!pathname.startsWith("/api/v1/public/")
	) {
		return null;
	}

	// These public routes are user-specific, mutation-adjacent, or streaming.
	// They must retain fresh per-request semantics even inside the E2E proxy.
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
		// Do not attach one browser request's signal to a fetch shared by several
		// E2E callers. These are bounded JSON GETs; a navigation abort from the
		// first caller must not cancel every coalesced waiter.
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

	// Refresh insertion order so the bounded map behaves as a small LRU.
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
	// Local dev is enabled automatically. Production-mode Playwright builds opt
	// in explicitly because their browser bundle is intentionally baked with a
	// same-origin API base so the same-origin session path can be tested. Real
	// deployments never set PLAYWRIGHT_E2E_API_PROXY and continue to 404 here.
	// As defence in depth, the production-mode opt-in is accepted only when the
	// request itself targets a loopback host; leaking the flag into a deployed
	// environment therefore cannot expose this credential-forwarding proxy.
	if (!isLocalApiProxyEnabled(req.url)) {
		return new NextResponse(null, { status: 404 });
	}

	const url = apiProxyTarget(req.url);

	const headers = new Headers();
	for (const [k, v] of req.headers.entries()) {
		const lk = k.toLowerCase();
		// Skip Next.js internal headers and hop-by-hop host.
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
		// req.signal: when the browser drops the request (navigation, logout,
		// SSE watchdog abort), abort the upstream fetch too. Without this the
		// gateway connection lingers until upstream times out — for SSE that
		// leaks into the per-account 3-stream cap and surfaces as 429s.
		upstream = await fetch(url, {
			method: req.method,
			headers,
			body,
			signal: req.signal,
		});
	} catch {
		return new NextResponse("upstream unreachable", { status: 502 });
	}

	const respHeaders = new Headers(upstream.headers);
	// Node fetch decompresses the body; strip stale encoding/length headers so
	// the browser doesn't try to decompress already-decompressed data.
	respHeaders.delete("content-encoding");
	respHeaders.delete("transfer-encoding");
	respHeaders.delete("content-length");

	// Preserve multiple Set-Cookie headers verbatim. `new Headers(upstream.headers)`
	// folds repeated Set-Cookie into one comma-joined value (undici), which
	// corrupts cookies. getSetCookie() splits them correctly; re-append each so
	// the browser sees distinct headers. No Domain rewrite needed: the gateway
	// (test-api and prod) is host-only post-B-202 and never sends a Domain
	// attribute.
	const setCookies = upstream.headers.getSetCookie();
	if (setCookies.length > 0) {
		respHeaders.delete("set-cookie");
		for (const cookie of setCookies) {
			respHeaders.append("set-cookie", cookie);
		}
	}

	// SSE pass-through. The buffered `arrayBuffer()` path below hangs on
	// `text/event-stream` since the upstream body never closes while the
	// stream is live — the realtime comment / notification feeds would
	// never reach the browser in dev. For SSE, hand the upstream body
	// to the browser as a ReadableStream so events flow per chunk.
	const upstreamCT = upstream.headers.get("content-type") ?? "";
	if (upstreamCT.includes("text/event-stream")) {
		// Don't pass `upstream.body` straight through. When the gateway later
		// drops the long-lived connection (idle/keepalive timeout, k3s recycle —
		// commonly after several minutes), undici terminates the body mid-pipe
		// and Next.js logs a noisy `failed to pipe response`
		// (UND_ERR_SOCKET: other side closed), billing the request as a 10-min
		// 500. Re-wrap the body so an upstream termination ends OUR stream
		// gracefully instead. The browser's SSE client treats a clean close like
		// an error — it reconnects either way (see lib/sse/client.ts) — so
		// swallowing the terminate changes nothing user-visible, only the dev
		// log. Browser-initiated disconnects still tear down the upstream via the
		// `req.signal` passed to fetch() above, with cancel() below as a backstop.
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
						// Upstream dropped the connection (gateway idle/keepalive
						// timeout, k3s recycle, or a browser-abort propagated via
						// req.signal). Close gracefully — the SSE client reconnects on
						// a clean close exactly as it would on an error.
						controller.close();
					}
				},
				cancel(reason) {
					// Downstream (browser) navigated away / aborted — release upstream.
					void reader.cancel(reason).catch(() => {});
				},
			});
		}

		return new NextResponse(safeBody, {
			status: upstream.status,
			headers: respHeaders,
		});
	}

	// 204 No Content / 304 Not Modified must carry a null body — handing the
	// Response constructor even an empty buffer throws "Invalid response status
	// code". The gateway returns 204 for OPTIONS preflights and for some
	// mutations (e.g. DELETE), so forward those bodyless.
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
