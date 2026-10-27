"use client";

import {
	type DelegationAuthorizationGrant,
	type DelegationAuthorizationHistoryPage,
	type DelegationAuthorizationLookup,
	type DelegationAuthorizationRevocation,
	type DelegationAuthorizationScope,
	type DelegationAuthorizationUpdate,
	type DelegationCapabilityCatalog,
	delegationAuthorizationErrorMessage,
	parseDelegationAuthorizationGrant,
	parseDelegationAuthorizationHistoryPage,
	parseDelegationAuthorizationLookup,
	parseDelegationAuthorizationRevocation,
	parseDelegationAuthorizationUpdate,
	parseDelegationCapabilityCatalog,
} from "@/features/delegated-operations/authorization";
import {
	buildDelegatedAllAuditLogSearch,
	buildDelegatedAuditLogSearch,
	buildDelegatedSessionSearch,
	type DelegatedAllAuditLogQuery,
	type DelegatedAuditLogPage,
	type DelegatedAuditLogQuery,
	type DelegatedOperationSessionPage,
	type DelegatedSessionListQuery,
	type DelegatedSessionReport,
	type DelegatedSessionRevocation,
	parseDelegatedAuditLogPage,
	parseDelegatedOperationSessionPage,
	parseDelegatedSessionReport,
	parseDelegatedSessionRevocation,
} from "@/features/delegated-operations/governance";
import {
	type DelegatedOperationPreflight,
	parseDelegatedOperationPreflight,
} from "@/features/delegated-operations/model";

interface ApiEnvelope {
	code?: number | string;
	message?: string;
	traceId?: string;
	data?: unknown;
	/** preflight 被拒時 BFF 一併帶回的業務碼（9264／9269／9272／9286…），供技術參考顯示。 */
	deniedCode?: number;
}

export class DelegatedOperationClientError extends Error {
	constructor(
		message: string,
		readonly status: number,
		readonly code?: number | string,
		readonly traceId?: string,
		readonly deniedCode?: number,
	) {
		super(message);
		this.name = "DelegatedOperationClientError";
	}
}

async function readEnvelope(response: Response): Promise<ApiEnvelope> {
	if (response.status === 204) return {};
	const raw = await response.json().catch(() => null);
	return raw && typeof raw === "object" ? (raw as ApiEnvelope) : {};
}

async function requestEnvelope(
	url: string,
	init: RequestInit = {},
): Promise<ApiEnvelope> {
	let response: Response;
	try {
		response = await fetch(url, {
			...init,
			cache: "no-store",
			credentials: "same-origin",
			headers: {
				Accept: "application/json",
				"X-Requested-With": "XMLHttpRequest",
				...(init.headers ?? {}),
			},
		});
	} catch {
		throw new DelegatedOperationClientError(
			"無法連線到代行營運服務，請確認網路後再試一次。",
			0,
			"TRANSPORT_ERROR",
		);
	}

	const envelope = await readEnvelope(response);
	if (!response.ok || (envelope.code !== undefined && envelope.code !== 0)) {
		throw new DelegatedOperationClientError(
			envelope.message || "代行營運服務暫時無法使用，請稍後再試。",
			response.status,
			envelope.code,
			envelope.traceId,
			Number.isSafeInteger(envelope.deniedCode) &&
				(envelope.deniedCode as number) > 0
				? envelope.deniedCode
				: undefined,
		);
	}
	return envelope;
}

function parsePreflight(data: unknown): DelegatedOperationPreflight {
	const parsed = parseDelegatedOperationPreflight(data);
	if (!parsed) {
		throw new DelegatedOperationClientError(
			"代行資格回應格式不完整，已停止進入工作區。請聯絡技術人員確認服務版本。",
			502,
			"INVALID_PREFLIGHT_CONTRACT",
		);
	}
	return parsed;
}

export async function getDelegatedOperationPreflight(
	storeId: string,
): Promise<DelegatedOperationPreflight> {
	const envelope = await requestEnvelope(
		`/api/admin/merchants/${encodeURIComponent(storeId)}/delegated-operation/preflight`,
	);
	return parsePreflight(envelope.data);
}

/**
 * 建立工作階段是頂層表單導覽（跨 host 的 303 只有瀏覽器跟得動），所以這裡沒有
 * 對應的 start 函式 —— 送出動作在 `OperationPreflightDialog` 的 `<form>` 上。
 * 這把 key 只是隨表單一起送出的欄位，讓同一次申請重試時仍是同一個請求。
 */
export function createDelegatedOperationIdempotencyKey(): string {
	if (!globalThis.crypto?.randomUUID) {
		throw new DelegatedOperationClientError(
			"瀏覽器無法建立安全的操作識別碼，請更新瀏覽器後再試。",
			0,
			"SECURE_RANDOM_UNAVAILABLE",
		);
	}
	return globalThis.crypto.randomUUID();
}

const GOVERNANCE_PREFIX = "/api/admin/delegated-operations";

/**
 * 工作階段列表（治理回看）。
 *
 * 解析不過時**丟例外而不是回空頁**：一份「看起來是空的」列表與一份真的沒有任何代行
 * 活動的列表，在畫面上長得一模一樣，而後者正是治理最想確認的結論。
 */
export async function listDelegatedOperationSessions(
	query: Partial<DelegatedSessionListQuery>,
): Promise<DelegatedOperationSessionPage> {
	const search = buildDelegatedSessionSearch(query);
	const envelope = await requestEnvelope(
		`${GOVERNANCE_PREFIX}?${search.toString()}`,
	);
	const parsed = parseDelegatedOperationSessionPage(envelope.data);
	if (!parsed) {
		throw new DelegatedOperationClientError(
			"代行工作階段列表格式不完整，已停止顯示以免遺漏紀錄。",
			502,
			"INVALID_SESSION_LIST_CONTRACT",
		);
	}
	return parsed;
}

export async function getDelegatedSessionReport(
	sessionId: string,
): Promise<DelegatedSessionReport> {
	const envelope = await requestEnvelope(
		`${GOVERNANCE_PREFIX}/${encodeURIComponent(sessionId)}/summary`,
	);
	const parsed = parseDelegatedSessionReport(envelope.data);
	if (!parsed) {
		throw new DelegatedOperationClientError(
			"代行工作階段摘要格式不完整，無法顯示。",
			502,
			"INVALID_SESSION_REPORT_CONTRACT",
		);
	}
	return parsed;
}

/**
 * 跨工作階段稽核紀錄（治理回看）。
 *
 * 解析不過時**丟例外而不是回空頁**：理由同 listDelegatedOperationSessions —— 一份
 * 「看起來是空的」稽核列表與一份真的沒有任何代行動作的列表，在畫面上長得一模一樣，
 * 而後者正是治理最想確認的結論。
 */
export async function listAllDelegatedAuditLogs(
	query: Partial<DelegatedAllAuditLogQuery>,
): Promise<DelegatedAuditLogPage> {
	const search = buildDelegatedAllAuditLogSearch(query);
	const envelope = await requestEnvelope(
		`${GOVERNANCE_PREFIX}/audit-logs?${search.toString()}`,
	);
	const parsed = parseDelegatedAuditLogPage(envelope.data);
	if (!parsed) {
		throw new DelegatedOperationClientError(
			"代行稽核紀錄格式不完整，已停止顯示以免遺漏紀錄。",
			502,
			"INVALID_AUDIT_LOG_CONTRACT",
		);
	}
	return parsed;
}

export async function listDelegatedAuditLogs(
	sessionId: string,
	query: Partial<DelegatedAuditLogQuery>,
): Promise<DelegatedAuditLogPage> {
	const search = buildDelegatedAuditLogSearch(query);
	const envelope = await requestEnvelope(
		`${GOVERNANCE_PREFIX}/${encodeURIComponent(sessionId)}/audit-logs?${search.toString()}`,
	);
	const parsed = parseDelegatedAuditLogPage(envelope.data);
	if (!parsed) {
		throw new DelegatedOperationClientError(
			"代行稽核紀錄格式不完整，已停止顯示以免遺漏紀錄。",
			502,
			"INVALID_AUDIT_LOG_CONTRACT",
		);
	}
	return parsed;
}

/**
 * 撤銷。`idempotencyKey` 由呼叫端持有並在重試時沿用：撤銷已生效但回應沒收到時，
 * 換一把新鍵會讓後端把它當成第二次撤銷。
 */
export async function revokeDelegatedSession(input: {
	sessionId: string;
	revokeReason: string;
	idempotencyKey: string;
}): Promise<DelegatedSessionRevocation> {
	const envelope = await requestEnvelope(
		`${GOVERNANCE_PREFIX}/${encodeURIComponent(input.sessionId)}/revoke`,
		{
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({
				revokeReason: input.revokeReason,
				idempotencyKey: input.idempotencyKey,
			}),
		},
	);
	const parsed = parseDelegatedSessionRevocation(envelope.data);
	if (!parsed) {
		throw new DelegatedOperationClientError(
			"撤銷結果無法驗證，請重新整理確認工作階段狀態。",
			502,
			"INVALID_REVOCATION_CONTRACT",
		);
	}
	return parsed;
}

// ==================== 授權列七支（backend#2444） ====================

/**
 * 授權面錯誤 → 給人看的一句話。
 *
 * **一律以業務碼分派，不看 HTTP status**：9286（找不到授權設定）就是 404，而 404
 * 同時也是「這個環境沒有這支端點」的形狀。既有的
 * `MerchantOperationEntry` 有一條 `status === 404 || 503 ⇒「代行營運目前未啟用」`
 * 的分支——那是為 preflight 寫的（它真的會回 404 表示服務未啟用），授權面沿用會把
 * 「這家店還沒開通」講成「代行營運未啟用」，而兩者的下一步分別是「按下開通」與
 * 「找維運」。
 *
 * BFF 的 `safeUpstreamError` 不透傳上游訊息，所以碼是唯一還原得出語意的東西；碼本身
 * 一併顯示供客服／工單追。
 */
export function delegationAuthorizationErrorText(
	error: unknown,
	fallback: string,
): string {
	if (!(error instanceof DelegatedOperationClientError)) {
		return error instanceof Error ? error.message : fallback;
	}
	const base = delegationAuthorizationErrorMessage(
		error.code,
		error.message || fallback,
	);
	const code = typeof error.code === "number" ? `（代碼 ${error.code}）` : "";
	const trace = error.traceId ? `（追蹤編號 ${error.traceId}）` : "";
	return `${base}${code}${trace}`;
}

/**
 * 「這家店可不可以被代行、範圍多大」，發生在任何工作階段之前。身分同為 ADM
 * session，走同一支 `requestEnvelope`（同一份錯誤形狀、同一組 same-origin 標頭）。
 *
 * ⚠️ 全部走 `/api/admin/delegated-operations/**` 這條專用 same-origin route，不走
 * 泛用的 catch-all 代理：後者不會經過 parser、`safeUpstreamError` 與 no-store。
 */
const AUTHORIZATION_PREFIX = "/api/admin/delegated-operations/authorizations";

function jsonInit(method: "POST" | "PUT", body: unknown): RequestInit {
	return {
		method,
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(body),
	};
}

export async function getDelegationCapabilityCatalog(): Promise<DelegationCapabilityCatalog> {
	const envelope = await requestEnvelope(
		"/api/admin/delegated-operations/capabilities",
	);
	const parsed = parseDelegationCapabilityCatalog(envelope.data);
	if (!parsed) {
		// fail-closed：拿不到目錄就不開對話框，不得退回本地依賴表。
		throw new DelegatedOperationClientError(
			"代行能力目錄格式不完整，已停止開啟授權設定。",
			502,
			"INVALID_CAPABILITY_CATALOG_CONTRACT",
		);
	}
	return parsed;
}

export async function getDelegationAuthorization(
	storeId: string,
): Promise<DelegationAuthorizationLookup> {
	const envelope = await requestEnvelope(
		`${AUTHORIZATION_PREFIX}/${encodeURIComponent(storeId)}`,
	);
	const parsed = parseDelegationAuthorizationLookup(envelope.data);
	if (!parsed) {
		throw new DelegatedOperationClientError(
			"代行授權設定格式不完整，無法顯示。",
			502,
			"INVALID_AUTHORIZATION_CONTRACT",
		);
	}
	return parsed;
}

/**
 * 開通／復權。`idempotencyKey` 由呼叫端持有並在重試時沿用：開通已生效但回應沒收到
 * 時，換一把新鍵會讓後端把它當成第二次開通並回 9285。
 */
export async function grantDelegationAuthorization(input: {
	storeId: string;
	scope: DelegationAuthorizationScope;
	idempotencyKey: string;
}): Promise<DelegationAuthorizationGrant> {
	const envelope = await requestEnvelope(
		AUTHORIZATION_PREFIX,
		jsonInit("POST", {
			storeId: input.storeId,
			idempotencyKey: input.idempotencyKey,
			...input.scope,
		}),
	);
	const parsed = parseDelegationAuthorizationGrant(envelope.data);
	if (!parsed) {
		throw new DelegatedOperationClientError(
			"開通結果無法驗證，請重新整理確認授權狀態。",
			502,
			"INVALID_AUTHORIZATION_GRANT_CONTRACT",
		);
	}
	return parsed;
}

export async function updateDelegationAuthorization(input: {
	storeId: string;
	scope: DelegationAuthorizationScope;
	idempotencyKey: string;
}): Promise<DelegationAuthorizationUpdate> {
	const envelope = await requestEnvelope(
		`${AUTHORIZATION_PREFIX}/${encodeURIComponent(input.storeId)}`,
		jsonInit("PUT", {
			idempotencyKey: input.idempotencyKey,
			...input.scope,
		}),
	);
	const parsed = parseDelegationAuthorizationUpdate(envelope.data);
	if (!parsed) {
		throw new DelegatedOperationClientError(
			"調整結果無法驗證，請重新整理確認授權範圍。",
			502,
			"INVALID_AUTHORIZATION_UPDATE_CONTRACT",
		);
	}
	return parsed;
}

export async function revokeDelegationAuthorization(input: {
	storeId: string;
	revokeReason: string;
	idempotencyKey: string;
}): Promise<DelegationAuthorizationRevocation> {
	const envelope = await requestEnvelope(
		`${AUTHORIZATION_PREFIX}/${encodeURIComponent(input.storeId)}/revoke`,
		jsonInit("POST", {
			revokeReason: input.revokeReason,
			idempotencyKey: input.idempotencyKey,
		}),
	);
	const parsed = parseDelegationAuthorizationRevocation(envelope.data);
	if (!parsed) {
		throw new DelegatedOperationClientError(
			"撤回結果無法驗證，請重新整理確認授權狀態。",
			502,
			"INVALID_AUTHORIZATION_REVOCATION_CONTRACT",
		);
	}
	return parsed;
}

export async function listDelegationAuthorizationHistory(
	storeId: string,
	query: { page: number; pageSize: number },
): Promise<DelegationAuthorizationHistoryPage> {
	const search = new URLSearchParams({
		page: String(query.page),
		page_size: String(query.pageSize),
	});
	const envelope = await requestEnvelope(
		`${AUTHORIZATION_PREFIX}/${encodeURIComponent(storeId)}/history?${search.toString()}`,
	);
	const parsed = parseDelegationAuthorizationHistoryPage(envelope.data);
	if (!parsed) {
		throw new DelegatedOperationClientError(
			"代行授權變更歷程格式不完整，已停止顯示以免遺漏紀錄。",
			502,
			"INVALID_AUTHORIZATION_HISTORY_CONTRACT",
		);
	}
	return parsed;
}
