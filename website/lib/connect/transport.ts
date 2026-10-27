import type { DescMethod, DescService } from "@bufbuild/protobuf";
import {
	Code,
	ConnectError,
	createContextKey,
	type Interceptor,
	type Transport,
} from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { classifyError, factsFromError } from "@hawkr/api/connect/errors";
import { retryInterceptor } from "@hawkr/api/connect/retry";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import { AUTH_EXPIRED_EVENT, emitSellerSuspended } from "@/lib/api/client";
import {
	DelegatedRuntimeTransportError,
	isDelegatedRuntime,
} from "@/lib/api/delegated-runtime";
import {
	AuthUnavailableError,
	withClerkAuthorization,
} from "@/lib/auth/clerk-token";
import { getEnv } from "@/lib/config/env";
import { activeLocale } from "@/lib/intl/active-locale";
import type { AppLocale } from "@/lib/intl/locales";

// 對外 API（Protobuf＋Connect）的 transport，三種（前兩種與 lib/api/client.ts 的兩種模式相同）：
// - publicTransport：公開讀取。不帶 cookie、server 端簽 origin-auth（backend#2264）；瀏覽器的 HTTP 快取以
//   條件請求重新驗證（公開讀取是 `public, no-cache` 加 ETag）。
// - authedTransport：帶 Clerk 的 session token（ADR 2026-09-27-identity-clerk，lib/auth/clerk-token.ts）。
//   瀏覽器不快取、不帶 X-Active-Role（Connect 不讀工作區，只有舊 REST 帶它）、*AppService 回 unauthenticated 時發 AUTH_EXPIRED_EVENT、代行工作區內直接拋錯。
// - vendorPortalTransport：供應商入口，邀請範圍的 cookie 憑證，不經 Clerk（見該函式的說明）。
// 三者都有逾時、5xx 上報 Sentry 與錯誤模型 D7 的重試。入口在單體：CSRF 看 Sec-Fetch-Site／Origin，不需要
// X-Requested-With（〈API 設計規範（Connect）〉§二）。
// 三者也都送單一值的 `Accept-Language`（UI 語系，見 acceptLanguageInterceptor）。

/**
 * 瀏覽器打 Connect 的第一方路徑，每個環境都一樣：瀏覽器只跟自己的 origin 說話（lib/config/env.ts 的
 * readApiBase；〈系統架構與資料流向〉§2.4）。Connect 的路徑 `/{service}/{method}` 在單體的根目錄，不在
 * /api/v1 底下，所以同源時要有自己的前綴。
 *
 * 部署環境的第一方反向代理（web host 上）必須把兩個前綴都轉到單體：`/api/connect/*` 去掉前綴後轉到單體的
 * 根目錄，`/api/v1/*` 原樣轉送。本機開發與 Playwright 由 app/api/connect、app/api/v1 的 route 代行。
 */
export const FIRST_PARTY_CONNECT_PATH = "/api/connect";

/**
 * Connect 的 base URL，由 REST 的 base 推得，兩者永遠指向同一個入口：
 * - 瀏覽器、REST 的 base 是相對路徑或與頁面同源：第一方路徑 FIRST_PARTY_CONNECT_PATH。
 * - 瀏覽器、明確設定了跨來源的 base：那個 API host 的根目錄。
 * - server 端（SSR）：serverApiBase 那個 API host 的根目錄。
 */
export function connectBaseUrl(
	isBrowser = typeof window !== "undefined",
	pageOrigin: string | undefined = isBrowser && typeof location !== "undefined"
		? location.origin
		: undefined,
): string {
	const e = getEnv();
	if (!isBrowser) return apiHostRoot(e.serverApiBase);
	const base = e.apiBaseUrl;
	if (base.startsWith("/")) return FIRST_PARTY_CONNECT_PATH;
	const origin = originOf(base);
	if (origin !== undefined && origin === pageOrigin) {
		return `${origin}${FIRST_PARTY_CONNECT_PATH}`;
	}
	return apiHostRoot(base);
}

/** REST 的 base（`…/api/v1`）所在的 API host 根目錄。 */
function apiHostRoot(restBase: string): string {
	return restBase.replace(/\/api\/v1\/?$/, "");
}

function originOf(url: string): string | undefined {
	try {
		return new URL(url).origin;
	} catch {
		return undefined;
	}
}

export interface TransportOptions {
	/**
	 * 這些呼叫代表的 UI 語系，送成 `Accept-Language`。瀏覽器端不用傳：取自 `AppProviders` 依路由的語系設的目前語系
	 * （`lib/intl/active-locale.ts`；不讀 `<html lang>`，那個會被瀏覽器的翻譯改掉）。server 端（SSR、ISR）沒有
	 * 「目前頁面」，一定要由呼叫的人傳它渲染的路由語系；沒傳，呼叫在送出前就丟錯，不會悄悄送 zh-TW。
	 */
	locale?: AppLocale;
}

export interface PublicTransportOptions extends TransportOptions {
	/** server 端代表哪一位訪客呼叫（origin-auth 的限流主體）；瀏覽器端不用。 */
	visitorIp?: string;
}

/** `opts.locale`，沒傳時是瀏覽器端的目前語系；server 端沒傳是程式錯誤（`activeLocale()` 會丟錯）。 */
function transportLocale(opts: TransportOptions): AppLocale {
	return opts.locale ?? activeLocale();
}

/**
 * 每個 Connect 呼叫帶單一值的 `Accept-Language: <UI 語系>`（ADR 2026-10-17 D1）：伺服器用它協商回應的語系
 * （`language.Matcher`，信心值 No 時退回 zh-TW），回應帶 `Vary: Accept-Language`。要在 transport 設，
 * 不在 BFF 覆寫：部署時瀏覽器直接打單體，沒有 BFF 可以覆寫。單一值、不帶 q 值，不送瀏覽器自己的語言清單。
 * 放在 interceptors 的最外層，重試沿用同一份 header。
 */
export function acceptLanguageInterceptor(
	locale: () => AppLocale,
): Interceptor {
	return (next) => (req) => {
		req.header.set("Accept-Language", locale());
		return next(req);
	};
}

function languageInterceptor(opts: TransportOptions): Interceptor {
	return acceptLanguageInterceptor(() => transportLocale(opts));
}

/** 公開 service 的 transport。讀取（NO_SIDE_EFFECTS）以 GET 呼叫，回應可以被 CDN 重新驗證。 */
export function publicTransport(opts: PublicTransportOptions = {}): Transport {
	const isBrowser = typeof window !== "undefined";
	return createConnectTransport({
		baseUrl: connectBaseUrl(isBrowser),
		useHttpGet: true,
		defaultTimeoutMs: getEnv().apiTimeoutMs,
		fetch: publicFetch(isBrowser, opts.visitorIp),
		interceptors: [
			languageInterceptor(opts),
			reportInterceptor(),
			retryInterceptor({ server: !isBrowser }),
		],
	});
}

/**
 * 帶登入憑證的 transport：*AppService 的方法。瀏覽器端以 `Authorization: Bearer` 帶 Clerk 的 session token。
 *
 * 讀取（NO_SIDE_EFFECTS）照〈API 設計規範（Connect）〉§三、§九以 GET 呼叫：GET 不受 CSRF 檢查、可以安全
 * 重試。伺服器對 AppService 回 `private, no-store`，瀏覽器端再以 `cache: "no-store"` 保證不從 HTTP 快取
 * 讀：登出之後還拿到快取裡的帳號會讓人以為仍在登入。
 */
export function authedTransport(opts: TransportOptions = {}): Transport {
	const isBrowser = typeof window !== "undefined";
	return delegatedGuard(
		createConnectTransport({
			baseUrl: connectBaseUrl(isBrowser),
			useHttpGet: true,
			defaultTimeoutMs: getEnv().apiTimeoutMs,
			fetch: authedFetch(isBrowser),
			interceptors: [
				languageInterceptor(opts),
				reportInterceptor(sentryCapture("authed")),
				authExpiredInterceptor(),
				sellerSuspendedInterceptor(),
				retryInterceptor({ server: !isBrowser }),
			],
		}),
	);
}

/**
 * 供應商入口（VendorPortalPublicService、VendorPortalInviteeService）的 transport，兩個 service 共用。
 * 獨立存在，因為它們的憑證是邀請範圍的 session cookie（`__Host-hk_vp_<rfq_recipient_id>`，ExchangeInvite
 * 以 Set-Cookie 發、HttpOnly，JS 讀不到），與 Clerk 無關：
 * - publicTransport 以 `credentials: "omit"` 送出，Set-Cookie 落不了地、cookie 也送不出去；
 * - authedTransport 會帶 Clerk 的 Bearer，並在 unauthenticated 時發 AUTH_EXPIRED_EVENT，
 *   把供應商的 session 失效誤報成買家登入過期。
 * 所以這裡 `credentials: "include"`（同源走 /api/connect，cookie 隨請求送出）、不碰 Clerk，瀏覽器不快取。
 * 供應商憑證的規則只定義在這裡：沒有標 NO_SIDE_EFFECTS 的方法（PreviewInvite、ExchangeInvite、
 * VerifySession）一律 POST，憑證在訊息裡不進 URL；NO_SIDE_EFFECTS 的讀取（GetSession）以 GET 呼叫
 * （API 設計規範（Connect）§三），收件人 ID 在 query。
 *
 * 重試沿用 retryInterceptor：它只重送「方法標了 idempotency_level 或請求帶 request_id」的呼叫（API 設計規範
 * （Connect）的 client 規則）。所以 GetSession 可以重試；ExchangeInvite、VerifySession 沒有標，不會自動
 * 重送（ExchangeInvite 每次可能發新的 session；transport.unit.test.ts 鎖住）。
 */
export function vendorPortalTransport(opts: TransportOptions = {}): Transport {
	const isBrowser = typeof window !== "undefined";
	return createConnectTransport({
		baseUrl: connectBaseUrl(isBrowser),
		useHttpGet: true,
		defaultTimeoutMs: getEnv().apiTimeoutMs,
		fetch: (input, init) =>
			fetch(input, {
				...init,
				credentials: "include",
				...(isBrowser ? { cache: "no-store" as const } : {}),
			}),
		interceptors: [
			languageInterceptor(opts),
			reportInterceptor(sentryCapture("public")),
			retryInterceptor({ server: !isBrowser }),
		],
	});
}

/**
 * connect-query 的 TransportProvider 只有一個 transport：依 service 分派——*AppService 走 authedTransport，
 * 其他走 publicTransport。授權跟著 service 走（〈API 設計規範（Connect）〉§二），帶不帶憑證也跟著它。
 */
export function browserTransport(opts: TransportOptions = {}): Transport {
	const pub = publicTransport(opts);
	const authed = authedTransport(opts);
	const pick = (method: DescMethod) =>
		isAppService(method.parent) ? authed : pub;
	return {
		unary: (method, signal, timeoutMs, header, input, contextValues) =>
			pick(method).unary(
				method,
				signal,
				timeoutMs,
				header,
				input,
				contextValues,
			),
		stream: (method, signal, timeoutMs, header, input, contextValues) =>
			pick(method).stream(
				method,
				signal,
				timeoutMs,
				header,
				input,
				contextValues,
			),
	};
}

/** 登入後的 service（`<模組>AppService`）：沒有登入時回 unauthenticated 的那一類。 */
function isAppService(service: DescService): boolean {
	return service.name.endsWith("AppService");
}

/**
 * 代行執行期的護欄（56 號 L1051，同 lib/api/client.ts 的 dispatch）：/adm 工作區裡不得用管理員自己在 www
 * 的 session 呼叫。不發請求、直接以 DelegatedRuntimeTransportError 拒絕，漏切的元件當場壞掉，而不是靜默成功。
 */
function delegatedGuard(inner: Transport): Transport {
	const refuse = (method: DescMethod) =>
		Promise.reject(
			new DelegatedRuntimeTransportError(
				`/${method.parent.typeName}/${method.name}`,
			),
		);
	return {
		unary: (method, signal, timeoutMs, header, input, contextValues) =>
			isDelegatedRuntime()
				? refuse(method)
				: inner.unary(method, signal, timeoutMs, header, input, contextValues),
		stream: (method, signal, timeoutMs, header, input, contextValues) =>
			isDelegatedRuntime()
				? refuse(method)
				: inner.stream(method, signal, timeoutMs, header, input, contextValues),
	};
}

function authedFetch(isBrowser: boolean): typeof fetch {
	return async (input, init) => {
		const headers = new Headers(init?.headers);
		const authorize = async (skipCache = false) => {
			try {
				await withClerkAuthorization(headers, { skipCache });
			} catch (error) {
				if (error instanceof AuthUnavailableError) {
					throw new ConnectError(error.message, Code.Unavailable);
				}
				throw error;
			}
		};
		if (isBrowser) await authorize();
		const request: RequestInit = {
			...init,
			headers,
			credentials: "include",
			...(isBrowser ? { cache: "no-store" as const } : {}),
		};
		let response = await fetch(input, request);
		if (isBrowser && response.status === 401 && headers.has("Authorization")) {
			await authorize(true);
			response = await fetch(input, request);
		}
		return response;
	};
}

/**
 * 呼叫方以 contextValues 設成 true 時，unauthenticated 不發 AUTH_EXPIRED_EVENT：讀取登入帳號的呼叫
 * （fetchSession、completeSignIn）自己決定怎麼處理「沒有登入」。
 */
export const SUPPRESS_AUTH_EXPIRED = createContextKey(false, {
	description: "suppress AUTH_EXPIRED_EVENT",
});

/**
 * AppService 回 `permission_denied` 加 reason `SELLER_SUSPENDED`（D10）：賣家身分在 session 中途被停權，
 * 廣播與 REST 的 9240 同一個事件，賣家中心的外殼收到後導向停權公告。只廣播、不導頁，錯誤原樣丟出。
 */
export function sellerSuspendedInterceptor(
	emit: (detail: {
		path: string;
		message: string;
	}) => void = emitSellerSuspended,
): Interceptor {
	return (next) => async (req) => {
		try {
			return await next(req);
		} catch (raw) {
			const err = ConnectError.from(raw);
			if (isAppService(req.service)) {
				const d = classifyError(
					factsFromError(err, req.method, req.stream ? undefined : req.message),
				);
				if (
					d.action === "reason" &&
					d.reason === ErrorReason.SELLER_SUSPENDED
				) {
					emit({
						path: `/${req.service.typeName}/${req.method.name}`,
						message: d.message ?? err.rawMessage,
					});
				}
			}
			throw raw;
		}
	};
}

/**
 * *AppService 回 unauthenticated 表示登入已經不在（Clerk 的 session 失效，或帳號已停權、刪除），廣播
 * AUTH_EXPIRED_EVENT，由 auth context 清掉登入狀態（同 lib/api/client.ts 的 401）。公開 service 不需要登入，
 * 它的 unauthenticated 與登入無關，不廣播。
 */
export function authExpiredInterceptor(
	emit: (detail: { path: string; message: string }) => void = emitAuthExpired,
): Interceptor {
	return (next) => async (req) => {
		try {
			return await next(req);
		} catch (raw) {
			const err = ConnectError.from(raw);
			if (
				err.code === Code.Unauthenticated &&
				isAppService(req.service) &&
				!req.contextValues.get(SUPPRESS_AUTH_EXPIRED)
			) {
				emit({
					path: `/${req.service.typeName}/${req.method.name}`,
					message: err.rawMessage || "Unauthorized",
				});
			}
			throw raw;
		}
	};
}

function emitAuthExpired(detail: { path: string; message: string }): void {
	if (
		typeof window === "undefined" ||
		typeof window.dispatchEvent !== "function"
	)
		return;
	window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT, { detail }));
}

function publicFetch(isBrowser: boolean, visitorIp?: string): typeof fetch {
	return async (input, init) => {
		const headers = new Headers(init?.headers);
		if (!isBrowser) {
			// 同 lib/api/client.ts：只在 server 端簽，簽章程式碼不進瀏覽器的 bundle。
			const { signOriginAuth, ORIGIN_AUTH_HEADER } = await import(
				"@/lib/api/origin-auth.server"
			);
			const attestation = await signOriginAuth({
				method: init?.method ?? "GET",
				url: requestUrl(input),
				visitorIp,
			});
			if (attestation) headers.set(ORIGIN_AUTH_HEADER, attestation);
		}
		return fetch(input, {
			...init,
			headers,
			// 公開讀取不帶 cookie：共用快取不會因 cookie 分裂，也不外洩 session。
			credentials: "omit",
			// 不強制 no-store：伺服器回 `public, no-cache` 加弱 ETag，瀏覽器每次都帶 If-None-Match 重新驗證，
			// 沒變時拿到 304（API 設計規範（Connect）§九）；server 端的快取由呼叫方的 unstable_cache 負責。
		});
	};
}

function requestUrl(input: RequestInfo | URL): string {
	if (typeof input === "string") return input;
	return input instanceof URL ? input.href : input.url;
}

// 伺服器端的失敗（REST 的 5xx）才上報；沒收到回應的網路錯誤不算。同一個方法、code、trace 在 30 秒內只報一次，
// 一次 outage 不會淹沒 Sentry（同 lib/api/client.ts 的 reportApiFailure）。
const REPORTED_CODES = new Set([
	Code.Unknown,
	Code.Internal,
	Code.DataLoss,
	Code.Unavailable,
	Code.DeadlineExceeded,
	Code.Unimplemented,
]);
const REPORT_DEDUP_MS = 30_000;
const recentFailures = new Map<string, number>();

/** Test-only：清掉上報的去重狀態。 */
export function __resetConnectReportDedupForTest(): void {
	recentFailures.clear();
}

export function reportInterceptor(
	capture: (err: ConnectError, context: FailureContext) => void = sentryCapture(
		"public",
	),
): Interceptor {
	return (next) => async (req) => {
		try {
			return await next(req);
		} catch (raw) {
			const err = ConnectError.from(raw);
			if (REPORTED_CODES.has(err.code)) {
				const facts = factsFromError(err, req.method);
				if (!facts.noResponse) {
					const context = {
						procedure: `/${req.service.typeName}/${req.method.name}`,
						method: req.requestMethod,
						code: Code[err.code],
						traceId: err.metadata.get("x-trace-id") ?? "no-trace",
					};
					const key = Object.values(context).join(":");
					const now = Date.now();
					const last = recentFailures.get(key);
					if (last === undefined || now - last >= REPORT_DEDUP_MS) {
						recentFailures.set(key, now);
						capture(err, context);
					}
				}
			}
			throw raw;
		}
	};
}

export interface FailureContext {
	procedure: string;
	method: string;
	code: string;
	traceId: string;
}

function sentryCapture(
	mode: "public" | "authed",
): (err: ConnectError, c: FailureContext) => void {
	return (err, c) => {
		void import("@sentry/nextjs")
			.then((Sentry) => {
				Sentry.captureException?.(err, {
					fingerprint: ["api-connect", c.procedure, c.code],
					tags: {
						api_endpoint: c.procedure,
						api_method: c.method,
						api_mode: mode,
						connect_code: c.code,
					},
				});
			})
			.catch(() => {
				// 觀測失敗不能取代原本的結果或錯誤。
			});
	};
}
