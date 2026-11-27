/**
 * REST client for the not-yet-migrated `/api/v1` surface, served through the
 * monolith's entry point (which authenticates and forwards to the gateway).
 *
 *   apiClient.public(path, opts)  → ${base}/public/* (anonymous)
 *   apiClient.authed(path, opts)  → ${base}/app/*    (signed in)
 *
 * Authed requests in the browser carry the same-origin session cookie
 * (`__Host-hk_session`, HttpOnly; ADR 2026-10-30-identity-self-hosted-federated-login)
 * and no `Authorization` header. Credentials default by mode: authed → "include",
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
/** 任何呼叫收到 `ACCOUNT_SUSPENDED`（帳號停權，受限登入）時廣播；AuthProvider 收到後重讀登入狀態，
 * 讓頁面載入後才被停權的使用者看到橫幅。不登出、不導頁。 */
export const ACCOUNT_SUSPENDED_EVENT = "hk-account:suspended";
/** 舊 REST 的 403 body（`{code, message, reason}`）帶的 `reason`，值是 `ErrorReason.ACCOUNT_SUSPENDED` 的名稱。 */
const ACCOUNT_SUSPENDED_REASON = "ACCOUNT_SUSPENDED";

/**
 * 使用者沒有主動觸發的舊 REST 寫入（開啟通知時自動送出）：停權帳號收到
 * `ACCOUNT_SUSPENDED` 時靜默略過，不廣播（A7-D17、A7-D18）。與 Connect 的 `BACKGROUND_WRITE_METHODS` 同一個意思，
 * 形狀對齊 buyer-app 的 `_backgroundRestWrites`；新增自動送出的寫入時加在這裡，其餘寫入都視為使用者主動
 * （`read-all` 是使用者點的，不在這裡）。
 */
const BACKGROUND_REST_WRITES: readonly { method: string; path: RegExp }[] = [
	{ method: "PATCH", path: /\/app\/notifications\/\d+\/read\/?$/ },
];

export function isBackgroundRestWrite(method: string, path: string): boolean {
	const m = method.toUpperCase();
	return BACKGROUND_REST_WRITES.some(
		(rule) => rule.method === m && rule.path.test(path),
	);
}
const SELLER_SUSPENDED_CODE = 9240;

export interface ApiEnvelope<T> {
	code: number;
	data: T;
	msg?: string;
	message?: string;
	traceId?: string;
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

export interface RequestOptions extends RequestInit {
	json?: unknown;
	query?: Record<string, string | number | boolean | undefined | null>;
	throwOnHttpError?: boolean;
	throwOnEnvelopeError?: boolean;
	onUnauthorized?: () => void | Promise<void>;
	/** Suppress the AUTH_EXPIRED_EVENT emit on 401 (still throws UnauthorizedError). Used by background reads that must not sign the user out. */
	suppressAuthExpired?: boolean;
	/**
	 * Server side only: the visitor's session cookie to forward on an authed GET/HEAD
	 * (`sessionCookieHeader()` in lib/auth/session-cookie.server.ts). Ignored in the
	 * browser (which sends the HttpOnly cookie itself) and on writes (D6).
	 */
	cookie?: string;
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
	 * Narrower sibling of `softFail` for reads that back a `use cache` entry:
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
	 * `use cache` key — a per-visitor cache key would split the cache per
	 * visitor, which is the fan-out this issue exists to remove.
	 */
	visitorIp?: string | null;
}

/** @deprecated `role` is ignored — active role comes from the stored UI state. */
export interface AuthedRequestOptions extends RequestOptions {
	role?: "buyer" | "seller";
}

type Mode = "public" | "authed";

const MAX_RETRIES = 1;
const API_FAILURE_DEDUP_MS = 30_000;
const recentApiFailures = new Map<string, number>();
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
		});
}


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
	const isApp = p === "/app" || p.startsWith("/app/");
	const isCartPublic = p === "/public/carts" || p.startsWith("/public/carts/");
	if (isCartPublic) return `${base}${p}`;
	return `${base}${isApp ? p : `/app${p}`}`;
}

async function parseEnvelope<T>(res: Response): Promise<ApiEnvelope<T> | null> {
	const text = await res.text();
	if (!text.trim()) return null;
	try {
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

export function emitAccountSuspended(detail: {
	path?: string;
	message?: string;
}) {
	if (typeof window === "undefined") return;
	window.dispatchEvent(new CustomEvent(ACCOUNT_SUSPENDED_EVENT, { detail }));
}

async function dispatch<T>(
	mode: Mode,
	path: string,
	opts: AuthedRequestOptions,
): Promise<ApiEnvelope<T>> {
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
	if (getEnv().enableApiLogging)
		console.debug(`[api:${mode}]`, method, endpoint);

	const isBrowser = typeof window !== "undefined";
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
		credentials: opts.credentials ?? (mode === "authed" ? "include" : "omit"),
		cache: opts.cache ?? (isBrowser ? "no-store" : undefined),
		signal: composeSignals(
			opts.signal,
			AbortSignal.timeout(opts.timeoutMs ?? getEnv().apiTimeoutMs),
		),
	};
	if (isBrowser) delete init.next;

	const canRetry = method === "GET";
	const canSoftFail = opts.softFail === true && method === "GET";
	const canSoftFailNotFound =
		!canSoftFail && opts.softFailNotFound === true && method === "GET";
	const soft = (status: number, message?: string): ApiEnvelope<T> => ({
		code: status,
		data: null as unknown as T,
		message,
	});
	if (
		mode === "authed" &&
		!isBrowser &&
		opts.cookie &&
		(method === "GET" || method === "HEAD")
	) {
		headers.set("Cookie", opts.cookie);
	}
	let res: Response;
	for (let attempt = 0; ; attempt++) {
		try {
			res = await fetch(url, init);
		} catch (networkErr) {
			if (canRetry && !init.signal?.aborted && attempt < MAX_RETRIES) {
				continue;
			}
			if (canSoftFail) return soft(503, (networkErr as Error)?.message);
			throw networkErr;
		}
		if (res.status >= 500 && canRetry && attempt < MAX_RETRIES) {
			continue;
		}
		break;
	}

	let parsed: ApiEnvelope<T> | null;
	try {
		parsed = await abortable(parseEnvelope<T>(res), init.signal);
	} catch (readErr) {
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
			const error = new ApiClientError(
				parsed?.message || parsed?.msg || `HTTP ${res.status}`,
				res.status,
				parsed?.code,
				parsed,
				parsed?.reason,
			);
			reportApiFailure(error, { method, endpoint, mode, role: activeRole });
		}
		return soft(res.status, parsed?.message || parsed?.msg);
	}

	if (canSoftFailNotFound && res.status === 404) {
		return soft(404, parsed?.message || parsed?.msg);
	}

	if (res.status === 401) {
		if (opts.onUnauthorized) {
			try {
				await opts.onUnauthorized();
			} catch {
			}
		}
		if (mode === "authed" && !opts.suppressAuthExpired) {
			emitAuthExpired({
				path,
				message: parsed?.message || parsed?.msg || "Unauthorized",
			});
		}
		if (opts.throwOnHttpError !== false) {
			throw new UnauthorizedError(
				parsed?.message || parsed?.msg || "Unauthorized",
			);
		}
	}

	if (
		mode === "authed" &&
		res.status === 403 &&
		parsed?.reason === ACCOUNT_SUSPENDED_REASON &&
		!isBackgroundRestWrite(method, endpoint)
	) {
		emitAccountSuspended({ path, message: parsed?.message || parsed?.msg });
	}

	if (opts.throwOnHttpError !== false && !res.ok) {
		if (mode === "authed" && parsed?.code === SELLER_SUSPENDED_CODE) {
			emitSellerSuspended({ path, message: parsed?.message || parsed?.msg });
		}
		const error = new ApiClientError(
			parsed?.message || parsed?.msg || `HTTP ${res.status}`,
			res.status,
			parsed?.code,
			parsed,
			parsed?.reason,
		);
		reportApiFailure(error, { method, endpoint, mode, role: activeRole });
		throw error;
	}

	if (
		opts.throwOnEnvelopeError !== false &&
		parsed &&
		parsed.code != null &&
		parsed.code !== 0 &&
		parsed.code !== 200
	) {
		if (mode === "authed" && parsed.code === SELLER_SUSPENDED_CODE) {
			emitSellerSuspended({ path, message: parsed.message || parsed.msg });
		}
		throw new ApiClientError(
			parsed.message || parsed.msg || `code ${parsed.code}`,
			res.status,
			parsed.code,
			parsed,
			parsed.reason,
		);
	}

	if (canSoftFailNotFound && parsed === null) {
		throw new ApiClientError(
			`empty or unparseable body (HTTP ${res.status})`,
			res.status,
			undefined,
			null,
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
