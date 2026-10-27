/**
 * REST client for the not-yet-migrated `/api/v1` surface, served through the
 * monolith's entry point (which authenticates and forwards to the gateway).
 *
 *   apiClient.public(path, opts)  → ${base}/public/* (anonymous)
 *   apiClient.authed(path, opts)  → ${base}/app/*    (signed in)
 *
 * Authed requests in the browser carry the Clerk session token as
 * `Authorization: Bearer` (lib/auth/clerk-token.ts, ADR
 * 2026-09-27-identity-clerk). Credentials default by mode: authed → "include",
 * public → "omit" (an anonymous read must not carry cookies — it would fragment
 * shared/CDN caches). A caller may override via opts.credentials.
 * CSRF: the entry point checks Sec-Fetch-Site / Origin (the monolith's
 * http.CrossOriginProtection), so no custom header is needed.
 * Authed requests: X-Active-Role from the stored active role (workspace view
 * hint; gateway validates ∈ role set, injects downstream as X-Login-Role).
 *
 * Base: env.apiBaseUrl in the browser (same-origin first-party path), else
 * env.serverApiBase. A 401 on an authed call emits AUTH_EXPIRED_EVENT and the
 * auth context signs out.
 */

import { getStoredActiveRole, roleToHeader } from "../auth/active-role";
import {
	AuthUnavailableError,
	withClerkAuthorization,
} from "../auth/clerk-token";
import { getEnv } from "../config/env";
import {
	DelegatedRuntimeTransportError,
	isDelegatedRuntime,
} from "./delegated-runtime";
import { parseJsonSafe } from "./json";

export const AUTH_EXPIRED_EVENT = "hk-auth:expired";
/** 任何 authed 呼叫收到業務碼 9240（SELLER_SUSPENDED_OPERATION_BLOCKED）時廣播；SellerCenterShell
 * 收到後重讀店家，頁面能否使用由更新後的能力判斷決定。 */
export const SELLER_SUSPENDED_EVENT = "hk-seller:suspended";
const SELLER_SUSPENDED_CODE = 9240;

export interface ApiEnvelope<T> {
	code: number;
	data: T;
	msg?: string;
	message?: string;
	traceId?: string; // 後端有送（gateway 注入），對應 Tempo trace id
	/** procurement 18 支 RPC 的 Gateway 錯誤 JSON 是 {code,reason,message}
	 * 獨立三欄位（非標準 envelope），其餘端點的回應沒有這個欄位。 */
	reason?: string;
}

export class ApiClientError extends Error {
	public status: number;
	public code?: number;
	public payload?: unknown;
	public apiMethod?: string;
	public apiEndpoint?: string;
	public apiMode?: Mode;
	public apiRole?: string;
	public cloudflareRayId?: string;
	/** 機器可讀業務錯誤碼（procurement 的 {code,reason,message} 契約）；
	 * 沒有帶這個欄位的回應一律是 ""，與 Gateway 端 `reason := ""` 的
	 * 無-detail fallback 對齊，不是猜測值。 */
	public reason: string;
	constructor(
		message: string,
		status: number,
		code?: number,
		payload?: unknown,
		reason?: string,
	) {
		super(message);
		this.status = status;
		this.code = code;
		this.payload = payload;
		this.reason = reason ?? "";
		this.name = "ApiClientError";
	}
}

export class UnauthorizedError extends ApiClientError {
	constructor(message = "Unauthorized") {
		super(message, 401);
		this.name = "UnauthorizedError";
	}
}

function attachResponseCorrelation<T extends ApiClientError>(
	error: T,
	response: Response,
): T {
	// `CF-Ray` is returned as `<id>-<colo>` while the Workers trace attribute
	// `cloudflare.ray_id` contains the ID only. Store the normalized ID so a
	// Sentry issue can be cross-queried against the exported Worker log/trace
	// even where Cloudflare cannot propagate its native trace ID externally.
	const rayId = response.headers.get("cf-ray")?.split("-", 1)[0]?.trim();
	if (rayId && /^[0-9a-f]{16,32}$/i.test(rayId)) {
		error.cloudflareRayId = rayId;
	}
	return error;
}

export interface RequestOptions extends RequestInit {
	json?: unknown;
	query?: Record<string, string | number | boolean | undefined | null>;
	throwOnHttpError?: boolean;
	throwOnEnvelopeError?: boolean;
	onUnauthorized?: () => void | Promise<void>;
	/** Suppress the AUTH_EXPIRED_EVENT emit on 401 (still throws UnauthorizedError). Used by background reads that must not sign the user out. */
	suppressAuthExpired?: boolean;
	/**
	 * Next.js Data Cache directives, applied server-side only (the browser path is
	 * always no-store). Lets a cacheable public read opt into ISR / tag revalidation.
	 */
	next?: { revalidate?: number | false; tags?: string[] };
	/** Per-request timeout (ms). Defaults to the per-flavor `env.apiTimeoutMs`. Always applied — a caller `signal` is composed with it, not replaced. */
	timeoutMs?: number;
	/**
	 * Prerender-safe GET: network error / timeout / 5xx (after the idempotent retry)
	 * and 404 resolve to a `{ code, data: null }` envelope instead of throwing, so a
	 * transient upstream can never crash an SSG prerender. GET only — a mutation
	 * is never softened. Other 4xx still throw (a real client error the caller owns).
	 */
	softFail?: boolean;
	/**
	 * Narrower sibling of `softFail` for reads that back an `unstable_cache` entry:
	 * ONLY a genuine 404 (the resource is absent) resolves to a `{ code, data: null }`
	 * envelope — a legal, cacheable "not found". A 5xx / network error / unreadable
	 * or unparseable body still THROWS so the cache layer keeps its prior stale entry
	 * instead of committing an outage-time null as fresh. GET only; `softFail`
	 * (soften-all) wins if both are set.
	 */
	softFailNotFound?: boolean;
	/**
	 * The end user this server-side call is made on behalf of (backend#2264).
	 *
	 * Server-side only, and only where the visitor is actually visible: middleware
	 * and `/api/public/*` route handlers hold the request, a cached RSC render does
	 * not. Passing it makes the gateway charge this call to that person's rate-limit
	 * subject instead of to the shared SSR one; omitting it is not an error, it just
	 * meters the call as SSR.
	 *
	 * NEVER derive it from a header the browser controls, and never let it reach an
	 * `unstable_cache` key — a per-visitor cache key would split the cache per
	 * visitor, which is the fan-out this issue exists to remove.
	 */
	visitorIp?: string | null;
}

/** @deprecated `role` is ignored — active role comes from the stored UI state. */
export interface AuthedRequestOptions extends RequestOptions {
	role?: "buyer" | "seller";
}

type Mode = "public" | "authed";

// B-199: a transient backend blip (gateway answers 5xx — e.g. 503 when Redis is
// momentarily down/failing over) should be absorbed, not surfaced as a failure.
// Retry once, only for idempotent GETs — a mutation must never be replayed.
// No backoff delay: a real setTimeout() scheduled inside a streaming
// Suspense hole can fire after Cloudflare Workers has torn down this request's
// IoContext, throwing "Cannot perform I/O on behalf of a different request."
// Retrying immediately avoids the timer entirely.
const MAX_RETRIES = 1;
const API_FAILURE_DEDUP_MS = 30_000;
const recentApiFailures = new Map<string, number>();
// `url` may intentionally be same-origin (for example `/api/v1/...` in the
// browser). We only need its pathname for diagnostics, so give URL a harmless
// base instead of requiring every browser-facing API base to be absolute.
const ENDPOINT_PARSE_BASE = "https://hawkr.invalid";

function endpointPath(url: string): string {
	return new URL(url, ENDPOINT_PARSE_BASE).pathname;
}

function reportApiFailure(
	error: ApiClientError,
	context: {
		method: string;
		endpoint: string;
		mode: Mode;
		role?: string;
	},
): void {
	if (error.status < 500) return;

	error.apiMethod = context.method;
	error.apiEndpoint = context.endpoint;
	error.apiMode = context.mode;
	error.apiRole =
		context.role ?? (context.mode === "public" ? "anonymous" : "unknown");

	// The transport already retries an idempotent GET before this point. This
	// short process-local window additionally collapses polling/query fan-out for
	// the same failed endpoint so one backend outage cannot flood Sentry.
	const traceId = (error.payload as ApiEnvelope<unknown> | undefined)?.traceId;
	const key = [
		context.mode,
		context.method,
		context.endpoint,
		error.status,
		traceId || "no-trace",
	].join(":");
	const now = Date.now();
	const lastSeen = recentApiFailures.get(key);
	if (lastSeen !== undefined && now - lastSeen < API_FAILURE_DEDUP_MS) return;
	recentApiFailures.set(key, now);
	if (recentApiFailures.size > 200) {
		for (const [candidate, timestamp] of recentApiFailures) {
			if (now - timestamp >= API_FAILURE_DEDUP_MS)
				recentApiFailures.delete(candidate);
		}
	}

	// Load lazily so the transport remains usable in build/test contexts where
	// Sentry is intentionally absent or only partially mocked.
	void import("@sentry/nextjs")
		.then((Sentry) => {
			Sentry.captureException?.(error, {
				fingerprint: [
					"api-5xx",
					context.mode,
					context.method,
					context.endpoint,
					String(error.status),
				],
				tags: {
					api_endpoint: context.endpoint,
					api_method: context.method,
					api_mode: context.mode,
					api_role: error.apiRole,
					http_status: String(error.status),
				},
			});
		})
		.catch(() => {
			// Observability must never replace the original API result/error.
		});
}

export function __resetApiFailureDedupForTest(): void {
	recentApiFailures.clear();
}

/**
 * Breadcrumb for ANOMALOUS settled API calls — the request trail attached to
 * whatever error event fires later (breadcrumbs only ship inside events, so
 * this costs no quota). The SDK's default fetch breadcrumb already records
 * every request (url incl. query, method, status) — this one adds only what
 * that lacks: the envelope business code, on the two shapes the default trail
 * can't distinguish（HTTP 4xx/5xx、HTTP 200 但 code≠0 的業務失敗）。Only
 * path/status/code, never the body（scrub 底線）。Lazy import for the same
 * build/test reason as reportApiFailure.
 */
function recordApiBreadcrumb(data: {
	method: string;
	endpoint: string;
	status: number;
	code?: number;
	mode: Mode;
}): void {
	const envelopeFailure =
		data.code != null && data.code !== 0 && data.code !== 200;
	if (data.status < 400 && !envelopeFailure) return;
	void import("@sentry/nextjs")
		.then((Sentry) => {
			Sentry.addBreadcrumb?.({
				category: "api",
				level: "warning",
				data,
			});
		})
		.catch(() => {
			// Observability must never replace the original API result/error.
		});
}

// Bound every request so a hung backend connection can't block an SSG prerender
// worker (or a runtime request) indefinitely. The bound is the per-flavor
// `env.apiTimeoutMs` (dev 30s — tolerates the slower cross-origin localdev path;
// prod 15s), the env SSOT for timeouts. Callers may override by passing their
// own `signal`, or per-request via `opts.timeoutMs`.

function apiBase(): string {
	const e = getEnv();
	return typeof window === "undefined" ? e.serverApiBase : e.apiBaseUrl;
}

function appendQuery(url: string, query?: RequestOptions["query"]): string {
	if (!query) return url;
	const usp = new URLSearchParams();
	for (const [k, v] of Object.entries(query)) {
		if (v == null) continue;
		usp.set(k, String(v));
	}
	const qs = usp.toString();
	if (!qs) return url;
	return url.includes("?") ? `${url}&${qs}` : `${url}?${qs}`;
}

function normalizePath(path: string) {
	return path.startsWith("/") ? path : `/${path}`;
}

function buildUrl(mode: Mode, path: string): string {
	const p = normalizePath(path);
	const base = apiBase();
	if (mode === "public") {
		const rest = p.startsWith("/public") ? p.slice("/public".length) : p;
		return `${base}/public${rest.startsWith("/") ? rest : `/${rest}`}`;
	}
	// authed: most paths target /app/*; allow callers to omit "/app".
	const isApp = p === "/app" || p.startsWith("/app/");
	const isCartPublic = p === "/public/carts" || p.startsWith("/public/carts/");
	if (isCartPublic) return `${base}${p}`;
	return `${base}${isApp ? p : `/app${p}`}`;
}

async function parseEnvelope<T>(res: Response): Promise<ApiEnvelope<T> | null> {
	const text = await res.text();
	if (!text.trim()) return null;
	try {
		// parseJsonSafe preserves 64-bit TSIDs (post/comment/order ids sent as
		// JSON numbers) that a plain JSON.parse would round — the whole API surface
		// is safe under it (lib/api/json.ts), and it kills the TSID-as-number bug class.
		return parseJsonSafe<ApiEnvelope<T>>(text);
	} catch {
		return null;
	}
}

function abortError(signal: AbortSignal): Error {
	const reason = signal.reason;
	return reason instanceof Error ? reason : new Error("Request aborted");
}

function abortable<T>(
	promise: Promise<T>,
	signal?: AbortSignal | null,
): Promise<T> {
	if (!signal) return promise;
	if (signal.aborted) return Promise.reject(abortError(signal));
	return new Promise((resolve, reject) => {
		const onAbort = () => reject(abortError(signal));
		signal.addEventListener("abort", onAbort, { once: true });
		promise.then(resolve, reject).finally(() => {
			signal.removeEventListener("abort", onAbort);
		});
	});
}

// Merge an optional caller signal with the transport's timeout signal so the
// request aborts on whichever fires first. A caller signal (React Query aborts
// on unmount / query-key change) carries no deadline; without composing it the
// timeout would be dropped and a hung fetch could stay pending forever.
// AbortSignal.any is native on workerd and every browser target; with no caller
// signal the timeout signal is used directly.
function composeSignals(
	caller: AbortSignal | null | undefined,
	timeout: AbortSignal,
): AbortSignal {
	return caller ? AbortSignal.any([caller, timeout]) : timeout;
}

function emitAuthExpired(detail: { path?: string; message?: string }) {
	if (typeof window === "undefined") return;
	window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT, { detail }));
}

export function emitSellerSuspended(detail: {
	path?: string;
	message?: string;
}) {
	if (typeof window === "undefined") return;
	window.dispatchEvent(new CustomEvent(SELLER_SUSPENDED_EVENT, { detail }));
}

async function dispatch<T>(
	mode: Mode,
	path: string,
	opts: AuthedRequestOptions,
): Promise<ApiEnvelope<T>> {
	// 代行執行期唯一的傳輸層護欄（56 號 L1051）。理由與「為什麼是 throw 不是
	// fallback」寫在 lib/api/delegated-runtime.ts —— 一句話：漏切的元件必須壞掉，
	// 不能靜默用管理員自己的賣家身分成功。
	if (mode === "authed" && isDelegatedRuntime()) {
		throw new DelegatedRuntimeTransportError(path);
	}
	const method = (opts.method ?? "GET").toUpperCase();
	const headers = new Headers(opts.headers);
	if (!headers.has("Accept")) headers.set("Accept", "application/json");

	let body = opts.body;
	if (opts.json !== undefined) {
		if (!headers.has("Content-Type"))
			headers.set("Content-Type", "application/json");
		body = JSON.stringify(opts.json);
	}

	if (mode === "authed") {
		const role = roleToHeader(getStoredActiveRole());
		if (role) headers.set("X-Active-Role", role);
	}

	const url = appendQuery(buildUrl(mode, path), opts.query);
	const endpoint = endpointPath(url);
	const activeRole =
		mode === "authed" ? roleToHeader(getStoredActiveRole()) : undefined;
	// Logs are exported from Workers Observability to Sentry. Keep query strings
	// out of the record: search/customer filters can contain user-provided text,
	// while the normalized endpoint still provides the operational signal.
	if (getEnv().enableApiLogging)
		console.debug(`[api:${mode}]`, method, endpoint);

	const isBrowser = typeof window !== "undefined";
	// Rate-limit subject attestation (backend#2264). SERVER ONLY, and the guard is
	// the whole point: this Worker speaks for one visitor at a time, but from the
	// gateway's side every one of its calls arrives from one address. Signing in
	// the browser too would do the exact opposite of what this is for — it would
	// funnel every visitor's direct calls (OTP sends included) into whichever
	// single subject the browser could claim.
	//
	// Loaded lazily so neither the secret read nor the signing code enters the
	// client bundle, and awaited rather than raced: an unsigned request is not a
	// failure (the gateway falls back to the peer address) but a request signed
	// after it was sent is not a thing.
	if (!isBrowser) {
		const { signOriginAuth, ORIGIN_AUTH_HEADER } = await import(
			"./origin-auth.server"
		);
		const attestation = await signOriginAuth({
			method,
			url,
			visitorIp: opts.visitorIp,
		});
		if (attestation) headers.set(ORIGIN_AUTH_HEADER, attestation);
	}
	const init: RequestInit & {
		next?: { revalidate?: number | false; tags?: string[] };
	} = {
		...opts,
		method,
		headers,
		body,
		// public = anonymous read → "omit" (no cookies: keeps shared/CDN caches
		// from fragmenting on cookie). authed → "include". Override via
		// opts.credentials.
		credentials: opts.credentials ?? (mode === "authed" ? "include" : "omit"),
		// Browser-side requests must never be served from the HTTP cache. A mutation
		// on one URI (e.g. PUT /app/products/{id}/status) does NOT invalidate the
		// sibling GET /app/products/{id} (RFC 9111 §4.4 only invalidates the same
		// URI), so a soft navigation would read a stale product until a hard reload.
		// Authed/dynamic gateway data wants to be fresh; the cacheable SEO content
		// is fetched server-side instead. Server stays untouched (no browser cache
		// there, and forcing no-store would opt routes out of the Data Cache).
		// A caller may still opt back in explicitly via opts.cache.
		cache: opts.cache ?? (isBrowser ? "no-store" : undefined),
		// A caller's own signal (React Query passes one that aborts on unmount /
		// query-key change) carries NO time budget, so on its own it would let a
		// hung upstream stay pending forever. Compose it with the timeout so the
		// request aborts when EITHER the caller aborts OR the deadline elapses.
		signal: composeSignals(
			opts.signal,
			AbortSignal.timeout(opts.timeoutMs ?? getEnv().apiTimeoutMs),
		),
	};
	// Next Data Cache directives are server-only. `...opts` already spread `next`
	// into init; in the browser strip it so it can't conflict with no-store.
	if (isBrowser) delete init.next;

	// Retry once on a 5xx or a network error, but only for idempotent GETs
	// (B-199). 401/4xx are real outcomes and are never retried.
	const canRetry = method === "GET";
	// softFail only ever softens an idempotent GET — a mutation is never turned
	// into a silent null (it must surface so the caller can react / not double-send).
	const canSoftFail = opts.softFail === true && method === "GET";
	// Cache-layer variant: a genuine 404 is a cacheable null, but a 5xx / network
	// error / unreadable body MUST throw so an unstable_cache wrapper keeps its
	// prior stale entry rather than caching an outage-time null. GET only; the
	// broader softFail wins if both are set.
	const canSoftFailNotFound =
		!canSoftFail && opts.softFailNotFound === true && method === "GET";
	const soft = (status: number, message?: string): ApiEnvelope<T> => ({
		code: status,
		data: null as unknown as T,
		message,
	});
	try {
		if (mode === "authed")
			await withClerkAuthorization(headers, {}, init.signal);
	} catch (error) {
		if (canSoftFail) return soft(503, (error as Error)?.message);
		throw error;
	}
	let refreshed = false;
	let res: Response;
	for (let attempt = 0; ; attempt++) {
		try {
			res = await fetch(url, init);
			if (
				mode === "authed" &&
				isBrowser &&
				res.status === 401 &&
				!refreshed &&
				headers.has("Authorization")
			) {
				refreshed = true;
				await withClerkAuthorization(headers, { skipCache: true }, init.signal);
				res = await fetch(url, init);
			}
		} catch (networkErr) {
			if (
				!(networkErr instanceof AuthUnavailableError) &&
				canRetry &&
				!init.signal?.aborted &&
				attempt < MAX_RETRIES
			) {
				continue;
			}
			// Timeout / connection refused / DNS — a hung or unreachable upstream.
			if (canSoftFail) return soft(503, (networkErr as Error)?.message);
			throw networkErr;
		}
		if (res.status >= 500 && canRetry && attempt < MAX_RETRIES) {
			continue;
		}
		break;
	}

	// A settled 5xx / 404 soft-fails to a null-data envelope (prerender-safe).
	// Other 4xx fall through to the throw sites below — softFail absorbs "upstream
	// unavailable", not a real client error (400/401/403/409), which the caller owns.
	let parsed: ApiEnvelope<T> | null;
	try {
		parsed = await abortable(parseEnvelope<T>(res), init.signal);
	} catch (readErr) {
		// fetch() resolves on headers, but the body read can still abort — an
		// upstream that sends a status then stalls mid-body (the React #419
		// read-hang) trips the AbortSignal.timeout here, not at the fetch() above.
		// Under softFail that degrades like an unreachable upstream.
		if (canSoftFail)
			return soft(res.status || 503, (readErr as Error)?.message);
		throw readErr;
	}

	recordApiBreadcrumb({
		method,
		endpoint,
		status: res.status,
		code: parsed?.code,
		mode,
	});

	if (canSoftFail && (res.status === 404 || res.status >= 500)) {
		if (res.status >= 500) {
			const error = attachResponseCorrelation(
				new ApiClientError(
					parsed?.message || parsed?.msg || `HTTP ${res.status}`,
					res.status,
					parsed?.code,
					parsed,
					parsed?.reason,
				),
				res,
			);
			reportApiFailure(error, { method, endpoint, mode, role: activeRole });
		}
		return soft(res.status, parsed?.message || parsed?.msg);
	}

	// Cache-layer 404-only soften: a real not-found is a cacheable null. A 5xx is
	// intentionally NOT caught here — it falls through to the throw sites below so
	// the caller's unstable_cache keeps its prior stale entry.
	if (canSoftFailNotFound && res.status === 404) {
		return soft(404, parsed?.message || parsed?.msg);
	}

	if (res.status === 401) {
		if (opts.onUnauthorized) {
			try {
				await opts.onUnauthorized();
			} catch {
				/* ignore */
			}
		}
		if (mode === "authed" && !opts.suppressAuthExpired) {
			emitAuthExpired({
				path,
				message: parsed?.message || parsed?.msg || "Unauthorized",
			});
		}
		// Honour throwOnHttpError like every other 4xx: callers that pass
		// throwOnHttpError:false must NOT throw when the session is already gone —
		// they treat "no session" as a normal outcome. Falls through to normal
		// envelope handling.
		if (opts.throwOnHttpError !== false) {
			throw attachResponseCorrelation(
				new UnauthorizedError(parsed?.message || parsed?.msg || "Unauthorized"),
				res,
			);
		}
	}

	if (opts.throwOnHttpError !== false && !res.ok) {
		// 46 §7.2：9240 透傳 HTTP 409（BizDeny/GrpcErr 塌縮），走這支而非下面的
		// envelope-code 分支——收斂點統一放這裡才不會漏判 9240。
		if (mode === "authed" && parsed?.code === SELLER_SUSPENDED_CODE) {
			emitSellerSuspended({ path, message: parsed?.message || parsed?.msg });
		}
		const error = attachResponseCorrelation(
			new ApiClientError(
				parsed?.message || parsed?.msg || `HTTP ${res.status}`,
				res.status,
				parsed?.code,
				parsed,
				parsed?.reason,
			),
			res,
		);
		reportApiFailure(error, { method, endpoint, mode, role: activeRole });
		throw error;
	}

	if (
		opts.throwOnEnvelopeError !== false &&
		parsed &&
		// A bare (un-enveloped) response — e.g. a top-level array — has no `code`;
		// treat absent code as success rather than throwing "code undefined".
		parsed.code != null &&
		parsed.code !== 0 &&
		parsed.code !== 200
	) {
		if (mode === "authed" && parsed.code === SELLER_SUSPENDED_CODE) {
			emitSellerSuspended({ path, message: parsed.message || parsed.msg });
		}
		throw attachResponseCorrelation(
			new ApiClientError(
				parsed.message || parsed.msg || `code ${parsed.code}`,
				res.status,
				parsed.code,
				parsed,
				parsed.reason,
			),
			res,
		);
	}

	// Under softFailNotFound (a cache-backing read), an empty / unparseable 2xx
	// body is a backend contract breach, not a cacheable value — throw so the
	// cache keeps its prior stale entry instead of committing an empty payload.
	// (softFail and normal reads keep the lenient `{ code: 0, data: undefined }`
	// fallback below so a legitimately empty body stays non-fatal for them.)
	if (canSoftFailNotFound && parsed === null) {
		throw attachResponseCorrelation(
			new ApiClientError(
				`empty or unparseable body (HTTP ${res.status})`,
				res.status,
				undefined,
				null,
			),
			res,
		);
	}

	return (
		parsed ??
		({ code: 0, data: undefined as unknown as T } satisfies ApiEnvelope<T>)
	);
}

interface EnvelopeMethods {
	get<T = unknown>(
		path: string,
		opts?: AuthedRequestOptions,
	): Promise<ApiEnvelope<T>>;
	post<T = unknown>(
		path: string,
		opts?: AuthedRequestOptions,
	): Promise<ApiEnvelope<T>>;
	put<T = unknown>(
		path: string,
		opts?: AuthedRequestOptions,
	): Promise<ApiEnvelope<T>>;
	patch<T = unknown>(
		path: string,
		opts?: AuthedRequestOptions,
	): Promise<ApiEnvelope<T>>;
	delete<T = unknown>(
		path: string,
		opts?: AuthedRequestOptions,
	): Promise<ApiEnvelope<T>>;
}

function bind(mode: Mode): EnvelopeMethods {
	const verb =
		(m: string) =>
		<T>(path: string, opts: AuthedRequestOptions = {}) =>
			dispatch<T>(mode, path, { ...opts, method: m });
	return {
		get: verb("GET"),
		post: verb("POST"),
		put: verb("PUT"),
		patch: verb("PATCH"),
		delete: verb("DELETE"),
	};
}

export const apiClient = {
	public: bind("public"),
	authed: bind("authed"),
	buildUrl: (mode: Mode, path: string) => buildUrl(mode, path),
};

export type ApiClient = typeof apiClient;

export interface AuthedFetchOptions extends AuthedRequestOptions {}

/** @deprecated Prefer `apiClient.authed.<method>`. Kept so repo modules compile. */
export async function authedFetch<T = unknown>(
	path: string,
	opts: AuthedFetchOptions = {},
): Promise<ApiEnvelope<T>> {
	return dispatch<T>("authed", path, opts);
}
