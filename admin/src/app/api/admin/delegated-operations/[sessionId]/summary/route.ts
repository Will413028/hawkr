import type { NextRequest } from "next/server";
import {
	isDelegatedSessionId,
	parseDelegatedSessionReport,
} from "@/features/delegated-operations/governance";
import {
	contractViolation,
	invalidSessionId,
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

type Context = { params: Promise<{ sessionId: string }> };

/**
 * 治理摘要：可查任意工作階段，身分是 ADM session。
 *
 * ⚠️ 它與工作區那支 `current/summary` 是兩支端點、兩種身分，**不得合併**：
 * credential 持有者若能自行指定 sessionId，就讀得到別人的代行紀錄。
 */
export async function GET(request: NextRequest, context: Context) {
	const { sessionId } = await context.params;
	if (!isDelegatedSessionId(sessionId)) return invalidSessionId();

	const adminToken = await adminSession.token(request);
	if (!adminToken) return unauthorized();

	try {
		const upstream = await adminUpstreamFetch(
			DELEGATED_OPERATION_UPSTREAM.summary(sessionId),
			{ adminToken },
		);
		const payload = await readUpstreamJson(upstream);
		if (!upstream.ok) return upstreamRejection(payload, upstream.status);

		const parsed = parseDelegatedSessionReport(unwrapUpstreamData(payload));
		if (!parsed) return contractViolation("代行工作階段摘要回應無法驗證");
		if (parsed.sessionId !== sessionId) {
			// 回的是另一個工作階段的摘要：撤銷按鈕與稽核紀錄都掛在畫面上的這個 id
			// 之下，讓兩者脫鉤等於把治理動作指向一個沒人看過的對象。
			return contractViolation("代行工作階段摘要與查詢對象不符");
		}
		return ok(parsed);
	} catch (error) {
		return transportFailure(error);
	}
}
