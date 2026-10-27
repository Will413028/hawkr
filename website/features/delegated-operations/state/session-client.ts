import {
	type DelegatedSessionSummary,
	readDelegatedSessionSummary,
} from "./session";

/**
 * 代行工作階段三支 session 端點的瀏覽器側讀取（打 website 自己的 BFF，同源相對
 * 路徑，credential 由 `Path=/adm` 的 httpOnly cookie 自動附帶）。
 *
 * BFF 的回應形狀：成功 `{code:0,data:…}`；失敗 `{code,traceId}` —— **沒有
 * message**（不透傳，§4.4.4）。使用者看到的字串一律由前端自己決定。
 */

/** 根前綴恆為 "delegated"：結束代行時清空代行快取是一次 removeQueries(["delegated"])，
 *  且絕不誤刪買家／賣家快取（§4.5 定案第 4 條）。 */
export const DELEGATED_QUERY_ROOT = ["delegated"] as const;

/** 不帶 identity：它本身就是 identity 的來源。 */
export const DELEGATED_SESSION_QUERY_KEY = [
	"delegated",
	"session",
	"current",
] as const;

export interface DelegatedEndSummary {
	sessionId: string;
	displayCode: string;
	issuedAt: string;
	endedAt: string | null;
	durationSeconds: string;
	status: string;
	operationCount: string;
	riskOperationCount: string;
	pendingAuditCount: string;
	notifiedSeller: boolean;
	fulfillmentActionCount: string;
	financeActionCount: string;
	auditRetentionDays: string;
}

export class DelegatedSessionError extends Error {
	readonly status: number;
	readonly code: number | string | null;
	readonly traceId: string | null;

	constructor(status: number, code: number | string | null, traceId: unknown) {
		super(`delegated session request failed (${status}/${String(code)})`);
		this.name = "DelegatedSessionError";
		this.status = status;
		this.code = code;
		this.traceId = typeof traceId === "string" ? traceId : null;
	}
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

async function callSessionBff(
	path: string,
	init: RequestInit = {},
): Promise<unknown> {
	const method = (init.method ?? "GET").toUpperCase();
	const headers = new Headers(init.headers);
	headers.set("Accept", "application/json");
	// 非 GET 一律帶：BFF 的 CSRF 檢查同時要求它與同源 Origin（L1045）。
	if (method !== "GET") headers.set("X-Requested-With", "XMLHttpRequest");
	const response = await fetch(path, { ...init, headers, cache: "no-store" });
	let payload: unknown = null;
	try {
		payload = await response.json();
	} catch {
		payload = null;
	}
	const envelope = isRecord(payload) ? payload : null;
	const code = envelope?.code ?? null;
	if (!response.ok || code !== 0) {
		throw new DelegatedSessionError(
			response.status,
			typeof code === "number" || typeof code === "string" ? code : null,
			envelope?.traceId,
		);
	}
	return envelope?.data ?? null;
}

/**
 * 工作區倒數與終態偵測的唯一來源。
 *
 * ⚠️ 這支端點在 gateway 側是 `counts_as_activity=false` —— **輪詢不算活動**，
 * 這是 idle expiry 能不能成立的分水嶺。前端不得為了「保持連線」另打任何端點。
 */
export async function fetchDelegatedSessionCurrent(
	signal?: AbortSignal,
): Promise<DelegatedSessionSummary> {
	const data = await callSessionBff("/adm/api/session/current", { signal });
	const summary = readDelegatedSessionSummary(data);
	if (!summary) throw new DelegatedSessionError(200, "CONTRACT_MISMATCH", null);
	return summary;
}

/**
 * 結束摘要綠卡的唯一資料來源。
 *
 * ⚠️ 不是 `DELETE current`（它的 data 恰為 `{"ended":true}`，一個數字都沒有），
 * 也不是 `{sessionId}/summary`（那支掛 requireAdmin()，www host 打不到）。
 */
export async function fetchDelegatedEndSummary(
	signal?: AbortSignal,
): Promise<DelegatedEndSummary> {
	const data = await callSessionBff("/adm/api/session/summary", { signal });
	if (!isRecord(data)) {
		throw new DelegatedSessionError(200, "CONTRACT_MISMATCH", null);
	}
	return data as unknown as DelegatedEndSummary;
}

/**
 * 結束本次代行。
 *
 * 只有伺服器**明確證明**已終止（`data.ended === true`）才算結束；其餘一律視為
 * 「結果不明」並維持封鎖 —— 在狀態不明時繼續寫入，才是真正危險的那一半。
 *
 * ⚠️ `storeId` 是**必要參數**：BFF 拿它先打一次 `GET current` 驗 target 綁定，不符
 * 就回 409 且根本不 DELETE（§6.1.5 L2787）。一個放著沒動的舊分頁按下「結束代行」，
 * 結束掉的會是另一家店正在進行的工作階段。
 */
export async function endDelegatedSession(storeId: string): Promise<void> {
	const data = await callSessionBff("/adm/api/session/end", {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ storeId }),
	});
	if (!isRecord(data) || data.ended !== true) {
		throw new DelegatedSessionError(200, "END_UNPROVEN", null);
	}
}
