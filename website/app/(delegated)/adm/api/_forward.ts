import {
	getDelegatedCookieName,
	isSafeOperationCredential,
} from "@/features/delegated-operations/server/session";
import {
	assertDelegatedBrowserRequest,
	DelegatedRequestRejectedError,
	readIdempotencyKey,
} from "./_csrf";
import {
	callDelegatedUpstream,
	type DelegatedUpstreamResult,
	type DelegatedUpstreamTimeout,
	DelegatedUpstreamUnavailableError,
	delegatedJson,
	UPSTREAM_FAILURE,
} from "./_upstream";

/**
 * 每一支代行 BFF route 的共同軀幹（56 號 §4.5 L1043）。
 *
 * **一條路徑對一條 gateway 路徑，禁止 catch-all `[...path]` 代理**：catch-all 會讓
 * 「代行到底能打哪些端點」失去可列舉性，而那正是 §8.1.0 D1 在 Gateway 側花力氣建立
 * 的東西。所以這裡提供的是**共用的檢查與轉送順序**，不是共用的路徑 —— 每一支 route
 * 檔案自己寫死它要打的那一條 gateway 路徑。
 *
 * 順序是有理由的（下面 `forwardDelegated` 的實際順序，逐條對得上）：
 * 1. path param 值域 —— 純函式、不看任何身分，且它決定的是「這個 URL 有沒有對應
 *    的上游路徑」，那本來就是公開事實。
 * 2. 保留 header ／ CSRF —— **先於**憑證檢查：跨站來的請求不該從回應裡分辨出這個
 *    瀏覽器有沒有代行憑證。
 * 3. 憑證 —— 沒有就 **401 且完全不打 gateway**。
 * 4. `Idempotency-Key`、query allow-list、body。
 */

/** 上游未回可解析 envelope 時，BFF 自己的碼（字串，見 `_csrf.ts` 的理由）。 */
export { UPSTREAM_FAILURE } from "./_upstream";

/** TSID 與其他純數字 id（storeId／productId／postId）。 */
const NUMERIC_ID_PATTERN = /^\d{1,19}$/;
/** 訂單編號（`HK260728001` 形狀）。 */
const ORDER_NO_PATTERN = /^[A-Za-z0-9-]{1,64}$/;
/** 媒體 upload session（UUID 或 TSID，兩種都可能）。 */
const MEDIA_SESSION_PATTERN = /^[A-Za-z0-9_-]{1,64}$/;

function segment(value: string, pattern: RegExp): string {
	if (!pattern.test(value)) {
		throw new DelegatedRequestRejectedError("INVALID_PATH_PARAM", 400);
	}
	return value;
}

/**
 * ⚠️ 這些不是「防呆」，是**組 gateway URL 前的唯一過濾**。放行一個含 `/` 或 `?`
 * 的值，等於讓瀏覽器決定要打哪一條 gateway 路徑 —— 那把「代行只能打這 20 條」的
 * 保證整個拆掉。
 */
export function numericSegment(value: string): string {
	return segment(value, NUMERIC_ID_PATTERN);
}

export function orderNoSegment(value: string): string {
	return segment(value, ORDER_NO_PATTERN);
}

export function mediaSessionSegment(value: string): string {
	return segment(value, MEDIA_SESSION_PATTERN);
}

/**
 * 從 `Cookie` header 取 operation credential。
 *
 * 不走 `next/headers` 的 `cookies()`：那會把每一支 route handler 綁進一個 request
 * scope，於是它們就只能在跑起來的伺服器上被驗證。這裡的 handler 是純
 * `Request → Response`，可以在測試裡直接呼叫。
 *
 * cookie **名字**的權威在 `features/delegated-operations/server/session.ts`
 * （全 repo 只有 `/adm/enter` 與 `/adm/api/session/end` 能寫它）；這裡只讀。
 */
export function readOperationCredential(request: Request): string | null {
	const header = request.headers.get("cookie");
	if (!header) return null;
	const name = getDelegatedCookieName();
	for (const pair of header.split(";")) {
		const eq = pair.indexOf("=");
		if (eq === -1) continue;
		if (pair.slice(0, eq).trim() !== name) continue;
		const value = pair.slice(eq + 1).trim();
		return isSafeOperationCredential(value) ? value : null;
	}
	return null;
}

export interface DelegatedForwardSpec {
	method: "GET" | "POST" | "PATCH" | "DELETE";
	/** Gateway 路徑（`/api/v1` 之後）。**每支 route 自己寫死**，不從請求推導。 */
	path: string;
	/**
	 * §7.3.1 三檔。**資源端點的讀取也走 `mutation`（15s）那一檔** —— 那一檔的作用域
	 * 是「資源端點」不是「會寫入的端點」；`session` 的 10s 只服務三支 session 端點
	 * （其中 `current` 每 30 秒被輪詢一次，放寬它等於讓一支 hang 住的輪詢吃掉兩輪）。
	 */
	timeout: DelegatedUpstreamTimeout;
	/**
	 * 允許從瀏覽器帶到上游的 query key，逐支端點列舉。
	 *
	 * 清單以 **Gateway handler 真的讀取的 key** 為準（例：訂單列表只讀 `status`
	 * 與分頁兩鍵，沒有 `keyword`／`date_from`）。未列入的 key 一律 **400**，不靜默
	 * 丟棄 —— 靜默丟棄會變成「搜尋框打了字、清單完全沒篩，也沒有任何錯誤」。
	 */
	query?: readonly string[];
}

/** 上游回應 → 瀏覽器回應。`message` 一路不透傳，只有 `code`／`data`／`traceId`。 */
function respond(result: DelegatedUpstreamResult): Response {
	if (result.ok) {
		return delegatedJson(
			{ code: 0, data: result.data ?? null, traceId: result.traceId },
			200,
		);
	}
	// 上游給了業務碼 → 原碼原狀態轉出去。前端對 9270／9271／9272 有終態處置，
	// 塌成單一錯誤碼會讓工作區在工作階段已撤銷時仍顯示「暫時性錯誤，請重試」。
	if (result.code !== null && result.status >= 400 && result.status <= 599) {
		return delegatedJson(
			{ code: result.code, traceId: result.traceId },
			result.status,
		);
	}
	// 2xx 卻沒有可解析的 envelope、或 3xx（§6.1.0 明令上游不得回 3xx，
	// `_upstream.ts` 以 `redirect: "manual"` 讓它現形）——都算上游壞掉。
	return delegatedJson(
		{ code: UPSTREAM_FAILURE, traceId: result.traceId },
		502,
	);
}

function rejection(error: DelegatedRequestRejectedError): Response {
	return delegatedJson({ code: error.code }, error.status);
}

async function readJsonBody(request: Request): Promise<unknown> {
	const raw = await request.text();
	if (raw === "") return undefined;
	try {
		return JSON.parse(raw);
	} catch {
		throw new DelegatedRequestRejectedError("INVALID_BODY", 400);
	}
}

function buildQuery(request: Request, allowed: readonly string[]): string {
	const incoming = new URL(request.url).searchParams;
	const out = new URLSearchParams();
	for (const [key, value] of incoming) {
		if (!allowed.includes(key)) {
			throw new DelegatedRequestRejectedError("INVALID_QUERY", 400);
		}
		out.append(key, value);
	}
	const query = out.toString();
	return query === "" ? "" : `?${query}`;
}

/**
 * 走完整條轉送。
 *
 * ⚠️ 逾時與網路失敗**不是**業務失敗：回 504／502 ＋ `UPSTREAM_FAILURE`，前端據此走
 * 「結果不明」而**不清 cookie、不宣稱已結束**（寫入在狀態不明時被封鎖，才是安全的
 * 那一半）。
 */
export async function forwardDelegated(
	request: Request,
	/**
	 * 給函式時**在 try 內求值** —— path param 的值域檢查（`numericSegment` 等）會
	 * throw，寫在 route 檔案的 caller 端會變成一個沒人接的例外＝ 500。
	 */
	source: DelegatedForwardSpec | (() => DelegatedForwardSpec),
): Promise<Response> {
	let spec: DelegatedForwardSpec;
	let credential: string | null;
	let idempotencyKey: string | null;
	let query: string;
	let json: unknown;
	try {
		spec = typeof source === "function" ? source() : source;
		assertDelegatedBrowserRequest(request);
		credential = readOperationCredential(request);
		if (!credential) {
			throw new DelegatedRequestRejectedError("CREDENTIAL_MISSING", 401);
		}
		idempotencyKey = readIdempotencyKey(request);
		query = buildQuery(request, spec.query ?? []);
		json = spec.method === "GET" ? undefined : await readJsonBody(request);
	} catch (error) {
		if (error instanceof DelegatedRequestRejectedError) return rejection(error);
		throw error;
	}

	try {
		return respond(
			await callDelegatedUpstream({
				method: spec.method,
				path: `${spec.path}${query}`,
				timeout: spec.timeout,
				credential,
				idempotencyKey,
				// Cloudflare 邊緣寫的、client 設不了。不帶就會簽成 p="ssr"，整個 ADM
				// 的代行流量共用一個限流桶（限流主體＝per-admin-IP）。
				visitorIp: request.headers.get("cf-connecting-ip"),
				json,
			}),
		);
	} catch (error) {
		if (error instanceof DelegatedUpstreamUnavailableError) {
			return delegatedJson(
				{ code: UPSTREAM_FAILURE },
				error.timedOut ? 504 : 502,
			);
		}
		throw error;
	}
}
