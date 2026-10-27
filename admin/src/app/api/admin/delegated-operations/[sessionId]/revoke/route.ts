import { type NextRequest, NextResponse } from "next/server";
import {
	isDelegatedRevokeReason,
	isDelegatedSessionId,
	parseDelegatedSessionRevocation,
} from "@/features/delegated-operations/governance";
import {
	contractViolation,
	invalidRequest,
	invalidSessionId,
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

type Context = { params: Promise<{ sessionId: string }> };

const MAX_REVOKE_REQUEST_BYTES = 4 * 1024;

function isValidIdempotencyKey(value: unknown): value is string {
	return (
		typeof value === "string" &&
		value.length >= 8 &&
		value.length <= 128 &&
		/^[A-Za-z0-9._:-]+$/u.test(value)
	);
}

/**
 * 撤銷是 ADM 治理動作：由主控台對任一工作階段執行，只要 ADM session，**不接受也
 * 不需要** operation credential。撤銷即時生效，工作區的下一次輪詢與任何代行寫入
 * 立即 fail-closed。
 *
 * 已到期的工作階段仍可撤銷（留下治理痕跡）；重複撤銷回同一組 `revokedAt` 與
 * `revision`，因此重試是安全的。
 */
export async function POST(request: NextRequest, context: Context) {
	const { sessionId } = await context.params;
	if (!isDelegatedSessionId(sessionId)) return invalidSessionId();

	// 這支是瀏覽器 `fetch` 而不是頂層表單導覽（不跨 host），所以自訂標頭送得出去；
	// Origin 與 `X-Requested-With` 是同一件事的兩層，跨站表單兩者都湊不齊。
	if (!isSameOriginSubmission(request.headers)) {
		return NextResponse.json(
			{ code: "CROSS_ORIGIN_REJECTED", message: "來源不符" },
			{ status: 403 },
		);
	}
	if (request.headers.get("x-requested-with") !== "XMLHttpRequest") {
		return invalidRequest("XHR_REQUIRED", "撤銷請求必須由主控台發出");
	}

	const adminToken = await adminSession.token(request);
	if (!adminToken) return unauthorized();

	const declaredLength = request.headers.get("content-length");
	if (
		declaredLength &&
		/^\d+$/u.test(declaredLength) &&
		Number(declaredLength) > MAX_REVOKE_REQUEST_BYTES
	) {
		return invalidRequest("REQUEST_TOO_LARGE", "撤銷理由內容過大");
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
			"請填寫 1–200 字的撤銷理由",
		);
	}
	if (!isValidIdempotencyKey(idempotencyKey)) {
		// 重試必須沿用同一把鍵，否則「撤銷已生效但回應沒收到」的重試會被當成第二次
		// 撤銷；由瀏覽器產生、伺服器只驗形狀。
		return invalidRequest(
			"IDEMPOTENCY_KEY_REQUIRED",
			"撤銷工作階段需要有效的 Idempotency-Key",
		);
	}

	try {
		const upstream = await adminUpstreamFetch(
			DELEGATED_OPERATION_UPSTREAM.revoke(sessionId),
			{
				method: "POST",
				adminToken,
				idempotencyKey,
				body: { revokeReason: revokeReason.trim() },
			},
		);
		const payload = await readUpstreamJson(upstream);
		if (!upstream.ok) return upstreamRejection(payload, upstream.status);

		const parsed = parseDelegatedSessionRevocation(unwrapUpstreamData(payload));
		if (!parsed) return contractViolation("撤銷代行工作階段回應無法驗證");
		if (parsed.sessionId !== sessionId || parsed.status !== "revoked") {
			// 宣稱成功卻沒有回出「這個工作階段已 revoked」＝一份假的撤銷證明。
			return contractViolation("撤銷結果與請求對象不符");
		}
		return ok(parsed);
	} catch (error) {
		return transportFailure(error);
	}
}
