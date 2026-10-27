import type { NextRequest } from "next/server";
import {
	buildDelegatedAllAuditLogSearch,
	DELEGATED_AUDIT_ACTIONS,
	DELEGATED_AUDIT_MAX_PAGE_SIZE,
	DELEGATED_AUDIT_RESOURCE_TYPES,
	DELEGATED_AUDIT_RESULTS,
	type DelegatedAuditAction,
	type DelegatedAuditResourceType,
	type DelegatedAuditResult,
	isDelegatedNumericId,
	parseDelegatedAuditLogPage,
} from "@/features/delegated-operations/governance";
import {
	contractViolation,
	invalidRequest,
	ok,
	pickPositiveInt,
	pickTimestamp,
	transportFailure,
	unauthorized,
	upstreamRejection,
} from "@/features/delegated-operations/server/governance-route";
import {
	adminUpstreamFetch,
	DELEGATED_OPERATION_UPSTREAM,
	readUpstreamJson,
	unwrapUpstreamData,
} from "@/features/delegated-operations/server/upstream";
import { adminSession } from "@/lib/session";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

/**
 * 跨工作階段稽核紀錄（`GET /api/v1/admin/delegated-operations/audit-logs`，56 §11.3 P1）。
 *
 * 與 `[sessionId]/audit-logs` 是**兩種視角、同一種列**：那支答「這一場代行做了什麼」，
 * 這支答「這家店／這位管理員跨多場代行被改過什麼」。回應形狀完全相同，因此共用
 * `parseDelegatedAuditLogPage` —— 為它另建一份 parser 等於製造一個會各自漂移的副本。
 *
 * 這支端點在 backend PR #2723 就交付了，但直到 backend#2740 之前**零前端消費者**：
 * 管理員只能一場一場點進去看。兩支專為它建的索引
 * （`idx_aoal_target_created`／`idx_aoal_actor_created`）在那段期間一直在付寫入成本
 * 而沒有任何查詢在用。
 */

/** 值域外的篩選值在這裡落地成「不篩選」會靜默回全集，所以一律當成參數錯。 */
function pickFilter<T extends string>(
	raw: string | null,
	allowed: readonly T[],
): { ok: boolean; value: T | "" } {
	if (raw === null || raw === "") return { ok: true, value: "" };
	return (allowed as readonly string[]).includes(raw)
		? { ok: true, value: raw as T }
		: { ok: false, value: "" };
}

/**
 * 兩個 id 篩選欄。空字串＝不篩選；非空但不是合法數字 id ＝參數錯。
 *
 * 不合法時**不能**靜默降級成「不篩選」：那會把「你打錯了店家 id」變成「這是全平台
 * 的稽核紀錄」，而畫面上兩者只差在筆數多寡，看的人不會發現自己拿到的是別家的列。
 */
function pickIdFilter(raw: string | null): { ok: boolean; value: string } {
	if (raw === null || raw === "") return { ok: true, value: "" };
	return isDelegatedNumericId(raw)
		? { ok: true, value: raw }
		: { ok: false, value: "" };
}

export async function GET(request: NextRequest) {
	const adminToken = await adminSession.token(request);
	if (!adminToken) return unauthorized();

	const params = request.nextUrl.searchParams;
	const page = pickPositiveInt(params.get("page"), 1);
	const pageSize = pickPositiveInt(params.get("page_size"), 20);
	const actorAdminId = pickIdFilter(params.get("actor_admin_id"));
	const storeId = pickIdFilter(params.get("store_id"));
	const action = pickFilter<DelegatedAuditAction>(
		params.get("action"),
		DELEGATED_AUDIT_ACTIONS,
	);
	const resourceType = pickFilter<DelegatedAuditResourceType>(
		params.get("resource_type"),
		DELEGATED_AUDIT_RESOURCE_TYPES,
	);
	const result = pickFilter<DelegatedAuditResult>(
		params.get("result"),
		DELEGATED_AUDIT_RESULTS,
	);
	const dateFrom = pickTimestamp(params.get("date_from"));
	const dateTo = pickTimestamp(params.get("date_to"));
	if (
		page === null ||
		pageSize === null ||
		pageSize > DELEGATED_AUDIT_MAX_PAGE_SIZE ||
		!actorAdminId.ok ||
		!storeId.ok ||
		!action.ok ||
		!resourceType.ok ||
		!result.ok ||
		!dateFrom.ok ||
		!dateTo.ok
	) {
		return invalidRequest("INVALID_AUDIT_QUERY", "稽核查詢條件不正確");
	}

	const search = buildDelegatedAllAuditLogSearch({
		page,
		pageSize,
		actorAdminId: actorAdminId.value,
		storeId: storeId.value,
		action: action.value,
		resourceType: resourceType.value,
		result: result.value,
		dateFrom: dateFrom.value,
		dateTo: dateTo.value,
	});

	try {
		const upstream = await adminUpstreamFetch(
			`${DELEGATED_OPERATION_UPSTREAM.allAuditLogs}?${search.toString()}`,
			{ adminToken },
		);
		const payload = await readUpstreamJson(upstream);
		if (!upstream.ok) return upstreamRejection(payload, upstream.status);

		const parsed = parseDelegatedAuditLogPage(unwrapUpstreamData(payload));
		if (!parsed) return contractViolation("代行稽核紀錄回應無法驗證");
		return ok(parsed);
	} catch (error) {
		return transportFailure(error);
	}
}
