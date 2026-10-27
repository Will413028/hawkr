import type { NextRequest } from "next/server";
import {
	DELEGATION_AUTHORIZATION_MAX_PAGE_SIZE,
	parseDelegationAuthorizationHistoryPage,
} from "@/features/delegated-operations/authorization";
import { isAuthorizationStoreId } from "@/features/delegated-operations/server/authorization-route";
import {
	contractViolation,
	invalidRequest,
	ok,
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

type Context = { params: Promise<{ storeId: string }> };

function boundedPageParam(raw: string | null, max: number): number | null {
	if (raw === null || raw === "") return null;
	if (!/^\d{1,6}$/u.test(raw)) return Number.NaN;
	const value = Number(raw);
	return value >= 1 && value <= max ? value : Number.NaN;
}

/**
 * 授權變更歷程（append-only 稽核，新到舊）。
 *
 * 這張表存在的理由：每店終生一列 ⇒ 復權會就地覆寫前一次撤回的 `revokedAt`／
 * `revokedByAdminId`／`revokeReason`，「這家店曾經被撤回過」在授權列上不留痕跡。
 * 從未開通過的店回 9286 而不是空清單——「查無此店的授權」與「有授權但沒改過」是
 * 兩件事，而後者不可能發生（開通本身就會留下第一列）。
 */
export async function GET(request: NextRequest, context: Context) {
	const { storeId } = await context.params;
	if (!isAuthorizationStoreId(storeId)) {
		return invalidRequest("INVALID_STORE_ID", "商店識別碼格式不正確");
	}

	const adminToken = await adminSession.token(request);
	if (!adminToken) return unauthorized();

	// gateway 對越界分頁明確回 9260 而不是靜默夾值，這一層跟著擋：把一個必然被拒的
	// 請求送出去，只換來一句與分頁無關的通用錯誤。
	const page = boundedPageParam(
		request.nextUrl.searchParams.get("page"),
		999999,
	);
	const pageSize = boundedPageParam(
		request.nextUrl.searchParams.get("page_size"),
		DELEGATION_AUTHORIZATION_MAX_PAGE_SIZE,
	);
	if (Number.isNaN(page) || Number.isNaN(pageSize)) {
		return invalidRequest("INVALID_PAGING", "分頁參數超出允許範圍");
	}

	const search = new URLSearchParams();
	search.set("page", String(page ?? 1));
	search.set("page_size", String(pageSize ?? 20));

	try {
		const upstream = await adminUpstreamFetch(
			`${DELEGATED_OPERATION_UPSTREAM.authorizationHistory(storeId)}?${search.toString()}`,
			{ adminToken },
		);
		const payload = await readUpstreamJson(upstream);
		if (!upstream.ok) return upstreamRejection(payload, upstream.status);

		const parsed = parseDelegationAuthorizationHistoryPage(
			unwrapUpstreamData(payload),
		);
		if (!parsed) return contractViolation("代行授權變更歷程回應無法驗證");
		for (const entry of parsed.records) {
			if (entry.storeId !== storeId) {
				return contractViolation("代行授權變更歷程與查詢對象不符");
			}
		}
		return ok(parsed);
	} catch (error) {
		return transportFailure(error);
	}
}
