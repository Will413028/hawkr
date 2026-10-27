import { BACKEND_BASE_URL } from "@/services/backend";

/**
 * 建立（POST）與工作階段列表（GET）是同一條路徑的兩個方法。指向同一份字面值而不是
 * 各寫一次：路徑改了只改一處，且下面兩個具名 key 讓呼叫端讀得出自己在做哪件事。
 */
const SESSIONS_PATH = "/api/v1/admin/delegated-operations";

export const DELEGATED_OPERATION_UPSTREAM = {
	preflight: (storeId: string) =>
		`/api/v1/admin/sellers/${encodeURIComponent(storeId)}/delegated-operation/preflight`,
	create: SESSIONS_PATH,
	/**
	 * 跨工作階段治理查詢（56 §11.3 P1）。身分同樣是 ADM session；它**不吃** operation
	 * credential —— 讓憑證持有者列得出全平台的代行活動，等於把治理視角交給被治理的人。
	 */
	sessions: SESSIONS_PATH,
	// 治理三支：身分一律是 ADM session（`requireAdmin`），任一支都不接受也不需要
	// operation credential —— 跨工作階段回看是治理權限，不是代行權限。
	auditLogs: (sessionId: string) =>
		`/api/v1/admin/delegated-operations/${encodeURIComponent(sessionId)}/audit-logs`,
	/**
	 * 跨工作階段稽核（56 §11.3 P1）。`audit-logs` 是**靜態段**，與上面那支的
	 * `:session_id/audit-logs` 共存於同一棵路由樹 —— gateway 的註解特別標了這件事，
	 * 因為改成 `delegated-operation-audit-logs` 之類的名字會讓 bff-boundary 的
	 * `isDelegatedOperationNamespace` 認不出來，前端就能用泛用 api.get 繞過整套 parser。
	 */
	allAuditLogs: "/api/v1/admin/delegated-operations/audit-logs",
	summary: (sessionId: string) =>
		`/api/v1/admin/delegated-operations/${encodeURIComponent(sessionId)}/summary`,
	revoke: (sessionId: string) =>
		`/api/v1/admin/delegated-operations/${encodeURIComponent(sessionId)}/revoke`,
	// 授權列七支（backend#2444）：管「這家店可不可以被代行、範圍多大」，發生在任何
	// 工作階段之前。身分同為 ADM session，不接受也不需要 operation credential。
	//
	// ⚠️ 路徑一律含 `delegated-operations` 這個 segment：`bff-boundary.ts` 的
	// `isDelegatedOperationNamespace` 只認這個字面值來拒絕 catch-all 代理。改成
	// `delegation-authorizations` 之類的名字，前端就能用泛用 `api.get` 繞過整套
	// parser／`safeUpstreamError`／no-store 保護，而且不會有任何東西轉紅。
	capabilities: "/api/v1/admin/delegated-operations/capabilities",
	authorizations: "/api/v1/admin/delegated-operations/authorizations",
	authorization: (storeId: string) =>
		`/api/v1/admin/delegated-operations/authorizations/${encodeURIComponent(storeId)}`,
	authorizationRevoke: (storeId: string) =>
		`/api/v1/admin/delegated-operations/authorizations/${encodeURIComponent(storeId)}/revoke`,
	authorizationHistory: (storeId: string) =>
		`/api/v1/admin/delegated-operations/authorizations/${encodeURIComponent(storeId)}/history`,
} as const;

export const DELEGATED_OPERATION_UPSTREAM_TIMEOUT_MS = 10_000;

export interface AdminUpstreamRequest {
	/** PUT 是授權範圍調整（全量取代，backend#2444 的第 5 支）。 */
	method?: "GET" | "POST" | "PUT" | "DELETE";
	/** 管理員的 Access JWT 或本機 dev token。 */
	adminToken: string;
	body?: unknown;
	idempotencyKey?: string;
}

/**
 * A deliberately narrow server-to-server transport. It does not accept a
 * browser Headers object, so authorization/identity context cannot be smuggled
 * into the gateway. Authentication is the administrator's Access JWT
 * (obtained server-side, sent as a Bearer token).
 */
export async function adminUpstreamFetch(
	path: string,
	request: AdminUpstreamRequest,
): Promise<Response> {
	const controller = new AbortController();
	const timeout = setTimeout(
		() => controller.abort(),
		DELEGATED_OPERATION_UPSTREAM_TIMEOUT_MS,
	);
	const headers = new Headers({
		Accept: "application/json",
		Authorization: `Bearer ${request.adminToken}`,
		"X-Requested-With": "XMLHttpRequest",
	});
	if (request.body !== undefined) {
		headers.set("Content-Type", "application/json");
	}
	if (request.idempotencyKey) {
		headers.set("Idempotency-Key", request.idempotencyKey);
	}

	const target = `${BACKEND_BASE_URL}${path}`;

	try {
		return await fetch(target, {
			method: request.method ?? "GET",
			headers,
			body:
				request.body === undefined ? undefined : JSON.stringify(request.body),
			cache: "no-store",
			redirect: "manual",
			signal: controller.signal,
		});
	} finally {
		clearTimeout(timeout);
	}
}

export async function readUpstreamJson(response: Response): Promise<unknown> {
	return response.json().catch(() => null);
}

type UnknownRecord = Record<string, unknown>;

function isRecord(value: unknown): value is UnknownRecord {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** Hawkr success envelopes are `{ code: 0, data: ... }`. */
export function unwrapUpstreamData(payload: unknown): unknown | null {
	if (!isRecord(payload) || payload.code !== 0 || !("data" in payload)) {
		return null;
	}
	return payload.data;
}

/** Never pass an unknown upstream object (which may hold credentials) through. */
export function safeUpstreamError(payload: unknown): {
	code: string | number;
	message: string;
	traceId?: string;
} {
	if (!isRecord(payload)) {
		return { code: "UPSTREAM_FAILURE", message: "代行營運服務暫時無法使用" };
	}
	const code =
		(typeof payload.code === "number" &&
			Number.isSafeInteger(payload.code) &&
			payload.code !== 0) ||
		(typeof payload.code === "string" &&
			/^[A-Z][A-Z0-9_]{1,79}$/u.test(payload.code))
			? payload.code
			: "UPSTREAM_FAILURE";
	const traceId =
		typeof payload.traceId === "string" &&
		/^[A-Za-z0-9._:-]{1,128}$/u.test(payload.traceId)
			? payload.traceId
			: undefined;
	// Do not forward an upstream message. Unknown services may echo credentials,
	// request bodies, or internal topology in free-form error text.
	return {
		code,
		message: "代行營運服務暫時無法使用",
		...(traceId ? { traceId } : {}),
	};
}
