/**
 * 代行工作區的傳輸層（56 號 §4.5 L1052）。
 *
 * 打的是 website 自己的 BFF（`/adm/api/*`，同源相對路徑），憑證是 `Path=/adm` 的
 * httpOnly cookie —— 瀏覽器自動附帶，這一層看不到也碰不到它。
 *
 * ⚠️ **不重用 `lib/api/client.ts` 的 `buildUrl`**：那一支對 `authed` 會自動補上
 * `/app` 前綴（`:295-307`），套到代行路徑會組出 `/adm/api/app/...` 這種上游不存在
 * 的 URL，而且 404 回來的是 Next 的 HTML 頁 ⇒ 前端看到的是 JSON parse 失敗，指不到
 * 真正的原因。
 *
 * 回應形狀由 BFF 決定（`app/(delegated)/adm/api/_upstream.ts`）：成功
 * `{code:0,data}`、失敗 `{code,traceId}` —— **沒有 message**。所以這裡丟出的錯誤
 * 訊息一律是前端自己的字串，`code` 才是可判讀的訊號（終態 9270／9271／9272 由
 * `features/delegated-operations/state/session.ts` 對照）。
 */

import {
	ApiClientError,
	type ApiEnvelope,
	type AuthedFetchOptions,
} from "./client";
import { parseJsonSafe } from "./json";

/** BFF 根：`Path=/adm` 之下才吃得到 operation credential cookie。 */
export const DELEGATED_BFF_PREFIX = "/adm/api";

/** 瀏覽器側的逾時上界。BFF 自己另有三檔預算（§7.3.1），這一層只保證分頁不會
 *  永遠掛著等一個已經沒人回應的請求。 */
const DEFAULT_TIMEOUT_MS = 20_000;

const UNSAFE = new Set(["POST", "PUT", "PATCH", "DELETE"]);

export interface DelegatedFetchOptions extends AuthedFetchOptions {
	/** 所有 mutation 必填（§6.1.0 共通四欄），轉成 `Idempotency-Key` header。 */
	idempotencyKey?: string;
}

/**
 * 資料層工廠接受的傳輸函式。`authedFetch`（賣家）與 `delegatedFetch`（代行）都滿足
 * 它，這正是「同一份 repo 兩條路徑」的接縫。
 */
export type RepoFetcher = <T = unknown>(
	path: string,
	opts?: DelegatedFetchOptions,
) => Promise<ApiEnvelope<T>>;

/**
 * 代行寫入類 mutation 的共通三欄（§6.1.0）。賣家路徑一律不帶（傳 `undefined`），
 * 代行路徑必帶 `operationReason`，PATCH／publish／unpublish 另必帶 `expectedVersion`。
 */
export interface DelegatedMutationMeta {
	expectedVersion?: string;
	operationReason: string;
	/** 預設 `true`；BFF 與 Gateway 都不靠 DB DEFAULT 兜底，缺席時顯式填 true。 */
	notifySeller?: boolean;
}

function normalizePath(path: string): string {
	return path.startsWith("/") ? path : `/${path}`;
}

function appendQuery(
	url: string,
	query?: DelegatedFetchOptions["query"],
): string {
	if (!query) return url;
	const usp = new URLSearchParams();
	for (const [key, value] of Object.entries(query)) {
		if (value == null) continue;
		usp.set(key, String(value));
	}
	const qs = usp.toString();
	if (!qs) return url;
	return url.includes("?") ? `${url}&${qs}` : `${url}?${qs}`;
}

function composeSignals(
	caller: AbortSignal | null | undefined,
	timeout: AbortSignal,
): AbortSignal {
	return caller ? AbortSignal.any([caller, timeout]) : timeout;
}

async function parseEnvelope<T>(res: Response): Promise<ApiEnvelope<T> | null> {
	const text = await res.text();
	if (!text.trim()) return null;
	try {
		// 與 client.ts 同一支 parser：訂單編號、商品 id 是 TSID，經 JSON.parse 會被
		// 進位成另一個數字，而它看起來仍然是一個合法的 id。
		return parseJsonSafe<ApiEnvelope<T>>(text);
	} catch {
		return null;
	}
}

function failure(code: number | undefined, status: number): string {
	// BFF 不透傳上游 message（同一個碼兩份文案必然漂移），所以訊息由這裡產生。
	return code == null
		? `代行請求失敗（HTTP ${status}）`
		: `代行請求失敗（HTTP ${status}／代碼 ${code}）`;
}

export async function delegatedFetch<T = unknown>(
	path: string,
	opts: DelegatedFetchOptions = {},
): Promise<ApiEnvelope<T>> {
	if (typeof window === "undefined") {
		// 相對 URL 在伺服器上 fetch 不了，而且代行憑證只存在於瀏覽器的 cookie jar：
		// 讓它在這裡明講，比讓 undici 丟一句 "Failed to parse URL" 有用。
		throw new Error(
			"delegatedFetch 只能在瀏覽器使用：代行憑證是 Path=/adm 的 httpOnly cookie，伺服器側沒有它",
		);
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
	// BFF 的 CSRF 檢查同時要求它與同源 Origin（L1045）。
	if (UNSAFE.has(method)) headers.set("X-Requested-With", "XMLHttpRequest");
	if (opts.idempotencyKey) headers.set("Idempotency-Key", opts.idempotencyKey);

	const url = appendQuery(
		`${DELEGATED_BFF_PREFIX}${normalizePath(path)}`,
		opts.query,
	);

	const res = await fetch(url, {
		method,
		headers,
		body,
		// 同源請求本來就會送 cookie；寫出來是為了讓「憑證怎麼過去的」在這一支檔案裡
		// 讀得到，而不是散落在呼叫端的預設值知識裡。
		credentials: "same-origin",
		cache: "no-store",
		// 代行回應一律不是 3xx。真的收到就是 middleware 把 BFF 導去了登入頁，此時
		// 跟隨導向會拿到一個 HTTP 200 的 HTML —— 那個失敗形狀無從診斷。
		redirect: "manual",
		signal: composeSignals(
			opts.signal,
			AbortSignal.timeout(opts.timeoutMs ?? DEFAULT_TIMEOUT_MS),
		),
	});

	if (
		res.type === "opaqueredirect" ||
		(res.status >= 300 && res.status < 400)
	) {
		throw new ApiClientError(
			`代行 BFF 被導向（HTTP ${res.status}）：/adm/api 應該排除在 middleware matcher 之外`,
			res.status,
			undefined,
			null,
		);
	}

	const parsed = await parseEnvelope<T>(res);
	const code = parsed?.code;

	if (opts.throwOnHttpError !== false && !res.ok) {
		throw new ApiClientError(
			failure(code, res.status),
			res.status,
			code,
			parsed,
		);
	}

	if (
		opts.throwOnEnvelopeError !== false &&
		parsed &&
		parsed.code != null &&
		parsed.code !== 0
	) {
		throw new ApiClientError(
			failure(parsed.code, res.status),
			res.status,
			parsed.code,
			parsed,
		);
	}

	return (
		parsed ??
		({ code: 0, data: undefined as unknown as T } satisfies ApiEnvelope<T>)
	);
}
