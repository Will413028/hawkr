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
import { BuyerAppService } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { ChatAppService } from "@hawkr/api/gen/hawkr/chat/v1alpha1/chat_pb";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import {
	AUTH_EXPIRED_EVENT,
	emitAccountSuspended,
	emitSellerSuspended,
} from "@/lib/api/client";
import {
	DelegatedRuntimeTransportError,
	isDelegatedRuntime,
} from "@/lib/api/delegated-runtime";
import { getEnv } from "@/lib/config/env";
import { activeLocale } from "@/lib/intl/active-locale";
import type { AppLocale } from "@/lib/intl/locales";


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
	 * 伺服器端（RSC、server action）代訪客讀取時要轉送的 session cookie（`lib/auth/session-cookie.server.ts`
	 * 的 `sessionCookieHeader()`）。只有 authedTransport 用，也只在 GET 送出：伺服器發出的請求沒有瀏覽器的
	 * Sec-Fetch-Site／Origin，單體對 cookie 認證的寫入會拒絕，寫入由瀏覽器送（D6）。瀏覽器端不用。
	 */
	cookie?: string;
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
 * 登入與否都能呼叫的寫入型 beacon（例如 SellerPublicService.RecordProductView：登入者不計數，由伺服器判斷，
 * client 不判斷登入狀態）。帶同 authedTransport 的同源 session cookie；沒登入就是沒有 cookie，照常匿名送出，
 * 不丟錯、不發 AUTH_EXPIRED_EVENT、不導向登入。
 * 只給寫入用：讀取（GET）仍走 publicTransport，才能共用快取。server 端沒有 session，等同匿名。
 */
export function optionalAuthTransport(
	opts: PublicTransportOptions = {},
): Transport {
	const isBrowser = typeof window !== "undefined";
	return createConnectTransport({
		baseUrl: connectBaseUrl(isBrowser),
		useHttpGet: true,
		defaultTimeoutMs: getEnv().apiTimeoutMs,
		fetch: optionalAuthFetch(isBrowser, opts.visitorIp),
		interceptors: [
			languageInterceptor(opts),
			reportInterceptor(),
			retryInterceptor({ server: !isBrowser }),
		],
	});
}

function optionalAuthFetch(
	isBrowser: boolean,
	visitorIp: string | undefined,
): typeof fetch {
	if (!isBrowser) return publicFetch(false, visitorIp);
	return (input, init) =>
		fetch(input, { ...init, credentials: "include", cache: "no-store" });
}

/**
 * 帶登入憑證的 transport：*AppService 的方法。憑證是同源的 `__Host-hk_session` cookie（HttpOnly，JS 讀不到），
 * 由瀏覽器隨請求送出；不帶 `Authorization`。
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
			fetch: authedFetch(isBrowser, opts.cookie),
			interceptors: [
				languageInterceptor(opts),
				reportInterceptor(sentryCapture("authed")),
				authExpiredInterceptor(),
				suspensionInterceptor(),
				retryInterceptor({ server: !isBrowser }),
			],
		}),
	);
}

/**
 * 長連線（server streaming）專用的 transport：與 authedTransport 相同的語系、上報、登入失效與停權處理，
 * 但沒有 `defaultTimeoutMs`（串流的壽命由伺服器的 `max_lifetime` 決定，逾時會把正常的串流切斷），
 * 也沒有重試 interceptor（重連由串流 client 以最後的 seq 自己做，不能由 transport 重送整個請求）。
 */
export function streamTransport(opts: TransportOptions = {}): Transport {
	const isBrowser = typeof window !== "undefined";
	return delegatedGuard(
		createConnectTransport({
			baseUrl: connectBaseUrl(isBrowser),
			fetch: authedFetch(isBrowser, opts.cookie),
			interceptors: [
				languageInterceptor(opts),
				reportInterceptor(sentryCapture("authed")),
				authExpiredInterceptor(),
				suspensionInterceptor(),
			],
		}),
	);
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

function authedFetch(isBrowser: boolean, cookie?: string): typeof fetch {
	return async (input, init) => {
		const headers = new Headers(init?.headers);
		const method = init?.method ?? "GET";
		if (!isBrowser && cookie && (method === "GET" || method === "HEAD")) {
			headers.set("Cookie", cookie);
		}
		return fetch(input, {
			...init,
			headers,
			credentials: "include",
			...(isBrowser ? { cache: "no-store" as const } : {}),
		});
	};
}

/**
 * 呼叫方以 contextValues 設成 true 時，unauthenticated 不發 AUTH_EXPIRED_EVENT：讀取登入帳號的呼叫
 * （fetchSession）自己決定怎麼處理「沒有登入」。
 */
export const SUPPRESS_AUTH_EXPIRED = createContextKey(false, {
	description: "suppress AUTH_EXPIRED_EVENT",
});

type SuspensionEmit = (detail: { path: string; message: string }) => void;

const procedureKey = (service: DescService, method: DescMethod) =>
	`${service.typeName}/${method.name}`;

/**
 * 使用者沒有主動觸發的寫入（頁面載入或瀏覽時自動送出）：停權帳號收到 `ACCOUNT_SUSPENDED` 時靜默略過，
 * 不廣播、不轉到受限畫面（A7-D17、A7-D18）。key 是 `<service typeName>/<method name>`；
 * 新增自動觸發的寫入時加在這裡，其餘寫入都視為使用者主動。
 * 只列 `*AppService` 的方法：後端只對登入的 App 受眾回 `ACCOUNT_SUSPENDED`，公開、受邀與代行的 service
 * 以訪客身分執行，永遠不會收到它（`accessGuest`）。
 */
export const BACKGROUND_WRITE_METHODS: ReadonlySet<string> = new Set([
	procedureKey(BuyerAppService, BuyerAppService.method.recordVisit),
	procedureKey(ChatAppService, ChatAppService.method.markConversationRead),
]);

/**
 * *AppService 的 `permission_denied` 加停權 reason，錯誤只分類一次、依 reason 分派，都只廣播、不登出、不導頁，
 * 錯誤原樣丟出：
 * - `SELLER_SUSPENDED`（D10）：賣家身分在 session 中途被停權，與 REST 的 9240 同一個事件，賣家中心的外殼收到後
 *   導向停權公告。
 * - `ACCOUNT_SUSPENDED`：帳號被停權（受限登入，停權者仍能登入、瀏覽與刪除帳號）。使用者主動的寫入廣播一次，
 *   AuthProvider 收到後重讀登入狀態，頁面載入後才被停權的使用者因此看到橫幅；背景寫入
 *   （`BACKGROUND_WRITE_METHODS`）不廣播。
 */
export function suspensionInterceptor(
	emitSeller: SuspensionEmit = emitSellerSuspended,
	emitAccount: SuspensionEmit = emitAccountSuspended,
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
				if (d.action === "reason") {
					const detail = {
						path: `/${req.service.typeName}/${req.method.name}`,
						message: d.message ?? err.rawMessage,
					};
					if (d.reason === ErrorReason.SELLER_SUSPENDED) {
						emitSeller(detail);
					} else if (
						d.reason === ErrorReason.ACCOUNT_SUSPENDED &&
						!BACKGROUND_WRITE_METHODS.has(procedureKey(req.service, req.method))
					) {
						emitAccount(detail);
					}
				}
			}
			throw raw;
		}
	};
}

/**
 * *AppService 回 unauthenticated 表示登入已經不在（session 過期或被撤銷，或帳號已停權、刪除），廣播
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
			credentials: "omit",
		});
	};
}

function requestUrl(input: RequestInfo | URL): string {
	if (typeof input === "string") return input;
	return input instanceof URL ? input.href : input.url;
}

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
			});
	};
}
