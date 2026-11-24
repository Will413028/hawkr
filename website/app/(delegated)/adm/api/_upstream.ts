import type { DescService } from "@bufbuild/protobuf";
import {
	type Client,
	Code,
	ConnectError,
	createClient,
} from "@connectrpc/connect";
import { SocialDelegatedService } from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import { getEnv } from "@/lib/config/env";
import { connectBaseUrl, publicTransport } from "@/lib/connect/transport";

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
	// 機敏檔的一段（單體端讀 body 的時限是 90 秒，這裡多留 10 秒給回應）。
	file: 100_000,
};

export type DelegatedUpstreamTimeout =
	| "session"
	| "mutation"
	| "batch"
	| "file";

export interface DelegatedUpstreamRequest {
	method: "GET" | "POST" | "PATCH" | "DELETE";
	/** Gateway 路徑（`/api/v1` 之後），一條路徑對一條 gateway 路徑。 */
	path: string;
	timeout: DelegatedUpstreamTimeout;
	/** operation credential；S6 兌換這一支刻意不帶。 */
	credential?: string | null;
	idempotencyKey?: string | null;
	/**
	 * 管理員的真實位址（`x-real-ip`，Vercel 邊緣寫的、client 設不了）。
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
			(cause instanceof ConnectError && cause.code === Code.DeadlineExceeded) ||
			(cause instanceof Error &&
				(cause.name === "TimeoutError" || cause.name === "AbortError"));
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

export interface DelegatedFileRequest {
	method: "GET" | "PUT";
	/** 單體根目錄之後的路徑（`/api/files/delegated/…`），一條路徑對一條單體路徑。 */
	path: string;
	timeout: DelegatedUpstreamTimeout;
	credential: string;
	visitorIp?: string | null;
	/** PUT 的原始位元組，原樣送出；`Content-Length` 由它的長度決定。 */
	body?: ArrayBuffer;
}

/**
 * 代行 BFF 對單體檔案端點（`/api/files/delegated/…`，不經 Connect、不是 envelope）的轉送：憑證同樣只是
 * `X-HK-Operation-Session`，body 與回應都是原始位元組，所以回傳的是未解析的 `Response`（呼叫端決定放行哪些標頭）。
 * 3xx 不跟隨（`redirect: "manual"`），逾時與連不上丟 `DelegatedUpstreamUnavailableError`。
 */
export async function callDelegatedFile(
	request: DelegatedFileRequest,
): Promise<Response> {
	const url = `${connectBaseUrl(false)}${request.path}`;
	const headers = new Headers({
		"X-HK-Operation-Session": request.credential,
	});
	if (request.body !== undefined) {
		headers.set("Content-Type", "application/octet-stream");
		headers.set("Content-Length", String(request.body.byteLength));
	}
	const { signOriginAuth, ORIGIN_AUTH_HEADER } = await import(
		"@/lib/api/origin-auth.server"
	);
	const attestation = await signOriginAuth({
		method: request.method,
		url,
		visitorIp: request.visitorIp ?? undefined,
	});
	if (attestation) headers.set(ORIGIN_AUTH_HEADER, attestation);
	try {
		return await fetch(url, {
			method: request.method,
			headers,
			body: request.body,
			cache: "no-store",
			redirect: "manual",
			signal: AbortSignal.timeout(TIMEOUT_MS[request.timeout]),
		});
	} catch (error) {
		throw new DelegatedUpstreamUnavailableError(error);
	}
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

/** `GET /admin/delegated-operations/current` 回應 data 裡的目標店家 ID（工作階段綁定的那一家）；讀不到是 null。 */
export function targetStoreIdOf(data: unknown): string | null {
	if (!isRecord(data) || !isRecord(data.target)) return null;
	const storeId = data.target.storeId;
	return typeof storeId === "string" && storeId !== "" ? storeId : null;
}

/**
 * 代行 BFF 對單體 Connect 的呼叫（`SocialDelegatedService`，A7-2d）：憑證是 `X-HK-Operation-Session` 標頭，
 * 與 REST 一樣只在這一支檔案轉送。transport 是 `lib/connect/transport.ts` 的 `publicTransport`（server 端簽
 * origin-auth、不帶 cookie、有逾時），代行 service 不讀顧客或管理員的身分。
 *
 * 回傳與 `callDelegatedUpstream` 同形（`DelegatedUpstreamResult`）：成功的 `data` 是呼叫端整形後的回應，
 * 失敗依 Connect code 對回瀏覽器已認得的業務碼（見 `connectFailure`）。逾時與連不上丟
 * `DelegatedUpstreamUnavailableError`，與 REST 同樣讓前端走「結果不明」。
 */
export async function callDelegatedConnect<
	S extends DescService = typeof SocialDelegatedService,
>(request: {
	/** 要呼叫的代行 service；預設是 `SocialDelegatedService`。 */
	service?: S;
	/** 用 client 發出一個 unary 呼叫並把回應整形成瀏覽器看到的 `data`；`headers` 與 `timeoutMs` 由這裡填。 */
	call: (
		client: Client<S>,
		options: { headers: Headers; timeoutMs: number },
	) => Promise<unknown>;
	timeout: DelegatedUpstreamTimeout;
	credential: string;
	/**
	 * 工作區路由的 `:storeId`；不傳（上傳這類店家由 session 決定、不帶店的呼叫）就不做綁定店比對。Connect 請求沒有 store_id（目標是 session 綁定的店），所以在送出之前先讀 session 的綁定店比對：
	 * 不符回 403／9262 且不打 Connect（一個停在別家店頁面的分頁不能寫到現在綁定的這一家）；讀不到綁定店（session 已結束、
	 * 回應不認得）一律不送，原樣回 session 讀取的失敗。
	 */
	storeId?: string;
	visitorIp?: string | null;
}): Promise<DelegatedUpstreamResult> {
	if (request.storeId !== undefined) {
		const current = await callDelegatedUpstream({
			method: "GET",
			path: "/admin/delegated-operations/current",
			timeout: "session",
			credential: request.credential,
			visitorIp: request.visitorIp,
		});
		if (!current.ok) return current;
		if (targetStoreIdOf(current.data) !== request.storeId) {
			return {
				ok: false,
				status: 403,
				code: 9262,
				data: null,
				traceId: current.traceId,
			};
		}
	}
	const client = createClient(
		(request.service ?? SocialDelegatedService) as S,
		publicTransport({
			// 代行工作區只有 zh-TW（app/(delegated)）。
			locale: "zh-TW",
			visitorIp: request.visitorIp ?? undefined,
		}),
	);
	const headers = new Headers({ "X-HK-Operation-Session": request.credential });
	try {
		const data = await request.call(client, {
			headers,
			timeoutMs: TIMEOUT_MS[request.timeout],
		});
		return { ok: true, status: 200, code: 0, data, traceId: null };
	} catch (raw) {
		const error = ConnectError.from(raw);
		const failure = connectFailure(error);
		if (failure === null) throw new DelegatedUpstreamUnavailableError(raw);
		return { ok: false, data: null, traceId: null, ...failure };
	}
}

/**
 * Connect 的失敗 → 瀏覽器已認得的業務碼與 HTTP 狀態（`_forward.ts` 的 `respond` 原碼原狀態轉出）。
 * 對照以 Connect code 為準（單體不給代行 session 終止的專屬 reason，所以 `failed_precondition`
 * 不能分出「session 已結束」與其他前置條件，一律當暫時性的 9280；session 狀態由前端輪詢的
 * `session/current` 判定終態）。連不上、逾時、取消回 null，由呼叫端當成「結果不明」。
 */
function connectFailure(
	error: ConnectError,
): { status: number; code: number } | null {
	switch (error.code) {
		case Code.Unauthenticated:
			return { status: 401, code: 9271 };
		case Code.PermissionDenied:
			return { status: 403, code: 9262 };
		case Code.NotFound:
			return { status: 404, code: 9275 };
		case Code.InvalidArgument:
			return { status: 400, code: 9260 };
		case Code.Aborted:
			return { status: 409, code: 9266 };
		case Code.ResourceExhausted:
			return { status: 429, code: 9273 };
		case Code.FailedPrecondition:
			return { status: 409, code: 9280 };
		case Code.Unavailable:
		case Code.DeadlineExceeded:
		case Code.Canceled:
			return null;
		default:
			return { status: 502, code: 9280 };
	}
}
