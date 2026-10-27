import { getEnv } from "@/lib/config/env";

/**
 * 代行 BFF 唯一的上游轉送點（56 號 §4.5 L1040）。
 *
 * 形狀對照 `hawkr-admin` 的 `server/upstream.ts`（那份已過九支契約測試），
 * 三處刻意不同：
 * 1. **逾時參數化**而非模組級常數（§7.3.1 三檔：session 10s／單筆 mutation 15s／
 *    批次 60s）。admin 那支只有 session 端點，一個常數夠用；這裡不夠。
 * 2. **不接受瀏覽器的 `Headers` 物件**：授權語境不能從瀏覽器夾帶進來，能送到
 *    gateway 的 header 是下面這份寫死的 allow-list。
 * 3. **回應的 `message` 一律不透傳**，只留 `code` 與 `traceId`：同一個碼在兩個
 *    地方各有一份文案就會漂移，而使用者看到的字串由前端自己決定。
 *
 * ⚠️ 這支檔案在 `scripts/check-transport.ts` 的 ALLOWLIST 內（唯一被允許自組
 * gateway URL 的代行檔案）。**不要再開第二支** —— 「只有一個轉送點」正是代行區
 * 最需要的保證，而那份 allowlist 就是它的機械保證：任何第二支自己組 URL 的
 * route 會讓 gate 轉紅。
 */

/** §7.3.1 三檔逾時預算。媒體 upload session 兩支歸 mutation 檔。 */
const TIMEOUT_MS: Record<DelegatedUpstreamTimeout, number> = {
	session: 10_000,
	mutation: 15_000,
	batch: 60_000,
};

export type DelegatedUpstreamTimeout = "session" | "mutation" | "batch";

export interface DelegatedUpstreamRequest {
	method: "GET" | "POST" | "PATCH" | "DELETE";
	/** Gateway 路徑（`/api/v1` 之後），一條路徑對一條 gateway 路徑。 */
	path: string;
	timeout: DelegatedUpstreamTimeout;
	/** operation credential；S6 兌換這一支刻意不帶。 */
	credential?: string | null;
	idempotencyKey?: string | null;
	/**
	 * 管理員的真實位址（`cf-connecting-ip`，Cloudflare 邊緣寫的、client 設不了）。
	 * 不帶會讓簽章的 `p` 變成 `ssr`，整個 ADM 的代行流量共用一個限流桶。
	 */
	visitorIp?: string | null;
	json?: unknown;
}

export interface DelegatedUpstreamResult {
	/** HTTP 傳輸成功且業務碼為 0。 */
	ok: boolean;
	status: number;
	/** 業務碼；上游沒回可解析的 envelope 時為 null。 */
	code: number | null;
	data: unknown;
	traceId: string | null;
}

/** BFF 自己的失敗碼（字串），與 gateway 的 int32 業務碼刻意不同型別。 */
export const UPSTREAM_FAILURE = "UPSTREAM_FAILURE" as const;

export class DelegatedUpstreamUnavailableError extends Error {
	/**
	 * 逾時與其他網路失敗要分得出來：逾時的那一半**後端可能已經寫入**（504，操作者
	 * 必須以同一把 `Idempotency-Key` 重試而不是重填表單），連不上的那一半沒有。
	 */
	readonly timedOut: boolean;

	constructor(cause?: unknown) {
		super("delegated upstream unavailable");
		this.name = "DelegatedUpstreamUnavailableError";
		this.cause = cause;
		this.timedOut =
			cause instanceof Error &&
			(cause.name === "TimeoutError" || cause.name === "AbortError");
	}
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

export async function callDelegatedUpstream(
	request: DelegatedUpstreamRequest,
): Promise<DelegatedUpstreamResult> {
	const url = `${getEnv().serverApiBase}${request.path}`;
	// 恰五個名字（§4.5 L1040），下面四個條件式各補一個。多送一個都是在擴大
	// 「瀏覽器能影響上游語境的面積」，即使那個 header 現在看起來無害。
	const headers = new Headers({
		// Gateway 的 CSRF guard 對所有 unsafe method 都要求它；GET 帶著也無害，
		// 而少一個分支就少一個「這支忘了帶」的失敗形狀。
		"X-Requested-With": "XMLHttpRequest",
	});
	if (request.credential) {
		headers.set("X-HK-Operation-Session", request.credential);
	}
	if (request.idempotencyKey) {
		headers.set("Idempotency-Key", request.idempotencyKey);
	}
	if (request.json !== undefined) {
		headers.set("Content-Type", "application/json");
	}
	// 簽章失敗回 null 是設計上的 fail-open（gateway 驗不過就退回 peer address）。
	// 不得因為代行是敏感路徑就改成 throw：那會讓「沒設 ORIGIN_AUTH_SECRET 的環境」
	// 整個工作區不能用，而它本來只是少了一層限流身分。
	const { signOriginAuth, ORIGIN_AUTH_HEADER } = await import(
		"@/lib/api/origin-auth.server"
	);
	const attestation = await signOriginAuth({
		method: request.method,
		url,
		visitorIp: request.visitorIp ?? undefined,
	});
	if (attestation) headers.set(ORIGIN_AUTH_HEADER, attestation);

	let response: Response;
	try {
		response = await fetch(url, {
			method: request.method,
			headers,
			body:
				request.json === undefined ? undefined : JSON.stringify(request.json),
			cache: "no-store",
			// §6.1.0 明令上游不得回 3xx。跟隨重導會讓一個設定錯誤（例：反代把
			// /admin/* 導去登入頁）以 200 ＋ 一份不是 envelope 的 body 抵達，看起來
			// 像「上游回了怪東西」；manual 讓它以 3xx 現形，由呼叫端判成上游失敗。
			redirect: "manual",
			signal: AbortSignal.timeout(TIMEOUT_MS[request.timeout]),
		});
	} catch (error) {
		throw new DelegatedUpstreamUnavailableError(error);
	}

	let payload: unknown = null;
	try {
		payload = await response.json();
	} catch {
		payload = null;
	}
	const envelope = isRecord(payload) ? payload : null;
	const code =
		envelope && typeof envelope.code === "number" ? envelope.code : null;
	return {
		ok: response.ok && code === 0,
		status: response.status,
		code,
		data: envelope ? (envelope.data ?? null) : null,
		traceId:
			envelope && typeof envelope.traceId === "string"
				? envelope.traceId
				: null,
	};
}

/** 轉給瀏覽器的回應：只有 code／data／traceId，never message。 */
export function delegatedJson(
	body: { code: number | string; data?: unknown; traceId?: string | null },
	status: number,
): Response {
	return Response.json(
		{
			code: body.code,
			...(body.data === undefined ? {} : { data: body.data }),
			...(body.traceId ? { traceId: body.traceId } : {}),
		},
		{ status, headers: { "Cache-Control": "private, no-store" } },
	);
}
