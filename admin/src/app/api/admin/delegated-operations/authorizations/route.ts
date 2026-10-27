import { type NextRequest, NextResponse } from "next/server";
import { parseDelegationAuthorizationGrant } from "@/features/delegated-operations/authorization";
import {
	isAuthorizationStoreId,
	isRequestTooLarge,
	isValidAuthorizationIdempotencyKey,
	parseAuthorizationScopeBody,
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

/**
 * 開通／復權（首建與復權共用同一支，`uq_sda_store` 使每店終生一列）。
 *
 * 實際走了哪一條由回應的 `outcome` 回報（created／reinstated），不由呼叫方指定：
 * 讓瀏覽器宣告「這是首建」，它就會在一列已撤回的授權上宣稱建立了新授權。
 */
export async function POST(request: NextRequest) {
	if (!isSameOriginSubmission(request.headers)) {
		return NextResponse.json(
			{ code: "CROSS_ORIGIN_REJECTED", message: "來源不符" },
			{ status: 403 },
		);
	}
	if (request.headers.get("x-requested-with") !== "XMLHttpRequest") {
		return invalidRequest("XHR_REQUIRED", "授權設定請求必須由主控台發出");
	}

	const adminToken = await adminSession.token(request);
	if (!adminToken) return unauthorized();

	if (isRequestTooLarge(request)) {
		return invalidRequest("REQUEST_TOO_LARGE", "授權設定內容過大");
	}

	const body: unknown = await request.json().catch(() => null);
	const storeId =
		typeof body === "object" && body !== null
			? (body as { storeId?: unknown }).storeId
			: undefined;
	const idempotencyKey =
		typeof body === "object" && body !== null
			? (body as { idempotencyKey?: unknown }).idempotencyKey
			: undefined;
	if (!isAuthorizationStoreId(storeId)) {
		return invalidRequest("INVALID_STORE_ID", "商店識別碼格式不正確");
	}
	if (!isValidAuthorizationIdempotencyKey(idempotencyKey)) {
		// 重試必須沿用同一把鍵：開通已生效但回應沒送到時，換一把新鍵會讓後端把
		// 重試當成第二次開通，而第二次會撞上「已有有效授權」回 9285。
		return invalidRequest(
			"IDEMPOTENCY_KEY_REQUIRED",
			"開通代行授權需要有效的 Idempotency-Key",
		);
	}
	const scope = parseAuthorizationScopeBody(body);
	if (!scope.ok) return scope.response;

	try {
		const upstream = await adminUpstreamFetch(
			DELEGATED_OPERATION_UPSTREAM.authorizations,
			{
				method: "POST",
				adminToken,
				idempotencyKey,
				body: { storeId, ...scope.value },
			},
		);
		const payload = await readUpstreamJson(upstream);
		if (!upstream.ok) return upstreamRejection(payload, upstream.status);

		const parsed = parseDelegationAuthorizationGrant(
			unwrapUpstreamData(payload),
		);
		if (!parsed) return contractViolation("開通代行授權回應無法驗證");
		if (parsed.authorization.storeId !== storeId) {
			// 回的是另一家店的授權列：畫面會把它當成「這家店已開通」而顯示一份不屬於
			// 它的範圍，管理員接著會對著它按下撤回。
			return contractViolation("開通結果與請求對象不符");
		}
		return ok(parsed);
	} catch (error) {
		return transportFailure(error);
	}
}
