import { type NextRequest, NextResponse } from "next/server";
import { parseDelegationAuthorizationRevocation } from "@/features/delegated-operations/authorization";
import { isDelegatedRevokeReason } from "@/features/delegated-operations/governance";
import {
	isAuthorizationStoreId,
	isRequestTooLarge,
	isValidAuthorizationIdempotencyKey,
} from "@/features/delegated-operations/server/authorization-route";
import {
	contractViolation,
	invalidRequest,
	ok,
	transportFailure,
	unauthorized,
	upstreamRejection,
} from "@/features/delegated-operations/server/governance-route";
import { isSameOriginSubmission } from "@/features/delegated-operations/server/handoff";
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

/**
 * 撤回一家店的代行授權。
 *
 * 撤回三欄（`revokedAt`／`revokedByAdminId`／`revokeReason`）同生同滅，理由必填——
 * 少一個就是一筆解釋不了的撤回。上游在**同一個交易內**連帶中止該店所有進行中的
 * 代行工作階段，被中止的 id 回在 `revokedSessionIds`。
 *
 * 已撤回是冪等成功（`alreadyRevoked=true`），不是錯誤：重試不該把一次已經生效的
 * 治理動作顯示成失敗。該店從未開通才是 9286。
 */
export async function POST(request: NextRequest, context: Context) {
	const { storeId } = await context.params;
	if (!isAuthorizationStoreId(storeId)) {
		return invalidRequest("INVALID_STORE_ID", "商店識別碼格式不正確");
	}
	if (!isSameOriginSubmission(request.headers)) {
		return NextResponse.json(
			{ code: "CROSS_ORIGIN_REJECTED", message: "來源不符" },
			{ status: 403 },
		);
	}
	if (request.headers.get("x-requested-with") !== "XMLHttpRequest") {
		return invalidRequest("XHR_REQUIRED", "撤回請求必須由主控台發出");
	}

	const adminToken = await adminSession.token(request);
	if (!adminToken) return unauthorized();

	if (isRequestTooLarge(request)) {
		return invalidRequest("REQUEST_TOO_LARGE", "撤回理由內容過大");
	}

	const body: unknown = await request.json().catch(() => null);
	const revokeReason =
		typeof body === "object" && body !== null
			? (body as { revokeReason?: unknown }).revokeReason
			: undefined;
	const idempotencyKey =
		typeof body === "object" && body !== null
			? (body as { idempotencyKey?: unknown }).idempotencyKey
			: undefined;
	if (!isDelegatedRevokeReason(revokeReason)) {
		return invalidRequest(
			"REVOKE_REASON_REQUIRED",
			"請填寫 1–200 字的撤回理由",
		);
	}
	if (!isValidAuthorizationIdempotencyKey(idempotencyKey)) {
		return invalidRequest(
			"IDEMPOTENCY_KEY_REQUIRED",
			"撤回代行授權需要有效的 Idempotency-Key",
		);
	}

	try {
		const upstream = await adminUpstreamFetch(
			DELEGATED_OPERATION_UPSTREAM.authorizationRevoke(storeId),
			{
				method: "POST",
				adminToken,
				idempotencyKey,
				body: { revokeReason: revokeReason.trim() },
			},
		);
		const payload = await readUpstreamJson(upstream);
		if (!upstream.ok) return upstreamRejection(payload, upstream.status);

		const parsed = parseDelegationAuthorizationRevocation(
			unwrapUpstreamData(payload),
		);
		if (!parsed) return contractViolation("撤回代行授權回應無法驗證");
		if (
			parsed.authorization.storeId !== storeId ||
			parsed.authorization.revokedAt === null
		) {
			// 宣稱成功卻沒有回出「這一列已撤回」＝一份假的撤回證明。
			return contractViolation("撤回結果與請求對象不符");
		}
		return ok(parsed);
	} catch (error) {
		return transportFailure(error);
	}
}
