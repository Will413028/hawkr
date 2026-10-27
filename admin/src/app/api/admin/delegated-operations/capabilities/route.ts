import type { NextRequest } from "next/server";
import { parseDelegationCapabilityCatalog } from "@/features/delegated-operations/authorization";
import {
	contractViolation,
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

/**
 * 代行能力目錄（值／風險／**直接**依賴）。授權設定介面的勾選清單與連動規則都靠它。
 *
 * 依賴閉包在 DB 沒有落點，gateway 的 authz package 是唯一權威。取不到就回錯，
 * **不得**由這一層補一份預設目錄：補了之後畫面照樣打得開，而它允許送出的組合是誰
 * 也沒有在維護的猜測。呼叫端據此 fail-closed（不開對話框）。
 */
export async function GET(request: NextRequest) {
	const adminToken = await adminSession.token(request);
	if (!adminToken) return unauthorized();

	try {
		const upstream = await adminUpstreamFetch(
			DELEGATED_OPERATION_UPSTREAM.capabilities,
			{ adminToken },
		);
		const payload = await readUpstreamJson(upstream);
		if (!upstream.ok) return upstreamRejection(payload, upstream.status);

		const parsed = parseDelegationCapabilityCatalog(
			unwrapUpstreamData(payload),
		);
		if (!parsed) return contractViolation("代行能力目錄回應無法驗證");
		return ok(parsed);
	} catch (error) {
		return transportFailure(error);
	}
}
