import type { NextRequest } from "next/server";
import {
	buildDelegatedAuditLogSearch,
	DELEGATED_AUDIT_ACTIONS,
	DELEGATED_AUDIT_MAX_PAGE_SIZE,
	DELEGATED_AUDIT_RESOURCE_TYPES,
	DELEGATED_AUDIT_RESULTS,
	type DelegatedAuditAction,
	type DelegatedAuditResourceType,
	type DelegatedAuditResult,
	isDelegatedSessionId,
	parseDelegatedAuditLogPage,
} from "@/features/delegated-operations/governance";
import {
	contractViolation,
	invalidRequest,
	invalidSessionId,
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

type Context = { params: Promise<{ sessionId: string }> };

/** 值域外的篩選值在這裡就落地成「不篩選」會靜默回全集，所以一律當成參數錯。 */
function pickFilter<T extends string>(
	raw: string | null,
	allowed: readonly T[],
): { ok: boolean; value: T | "" } {
	if (raw === null || raw === "") return { ok: true, value: "" };
	return (allowed as readonly string[]).includes(raw)
		? { ok: true, value: raw as T }
		: { ok: false, value: "" };
}

export async function GET(request: NextRequest, context: Context) {
	const { sessionId } = await context.params;
	if (!isDelegatedSessionId(sessionId)) return invalidSessionId();

	const adminToken = await adminSession.token(request);
	if (!adminToken) return unauthorized();

	const params = request.nextUrl.searchParams;
	const page = pickPositiveInt(params.get("page"), 1);
	const pageSize = pickPositiveInt(params.get("page_size"), 20);
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
		!action.ok ||
		!resourceType.ok ||
		!result.ok ||
		!dateFrom.ok ||
		!dateTo.ok
	) {
		return invalidRequest("INVALID_AUDIT_QUERY", "稽核查詢條件不正確");
	}

	const search = buildDelegatedAuditLogSearch({
		page,
		pageSize,
		action: action.value,
		resourceType: resourceType.value,
		result: result.value,
		dateFrom: dateFrom.value,
		dateTo: dateTo.value,
	});

	try {
		const upstream = await adminUpstreamFetch(
			`${DELEGATED_OPERATION_UPSTREAM.auditLogs(sessionId)}?${search.toString()}`,
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
