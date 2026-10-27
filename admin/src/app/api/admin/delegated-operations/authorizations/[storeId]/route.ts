import { type NextRequest, NextResponse } from "next/server";
import {
	parseDelegationAuthorizationLookup,
	parseDelegationAuthorizationUpdate,
} from "@/features/delegated-operations/authorization";
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

type Context = { params: Promise<{ storeId: string }> };

/**
 * 單店授權詳情。查無授權列**不是錯誤**：上游回 200 ＋ `exists=false`，本層原樣轉出。
 * 用 404 表示「尚未開通」會讓它與「上游掛了」在畫面上長得一模一樣，而兩者的下一步
 * 分別是「按下開通」與「找維運」。
 */
export async function GET(request: NextRequest, context: Context) {
	const { storeId } = await context.params;
	if (!isAuthorizationStoreId(storeId)) {
		return invalidRequest("INVALID_STORE_ID", "商店識別碼格式不正確");
	}

	const adminToken = await adminSession.token(request);
	if (!adminToken) return unauthorized();

	try {
		const upstream = await adminUpstreamFetch(
			DELEGATED_OPERATION_UPSTREAM.authorization(storeId),
			{ adminToken },
		);
		const payload = await readUpstreamJson(upstream);
		if (!upstream.ok) return upstreamRejection(payload, upstream.status);

		const parsed = parseDelegationAuthorizationLookup(
			unwrapUpstreamData(payload),
		);
		if (!parsed) return contractViolation("單店代行授權回應無法驗證");
		if (parsed.authorization && parsed.authorization.storeId !== storeId) {
			return contractViolation("代行授權與查詢對象不符");
		}
		return ok(parsed);
	} catch (error) {
		return transportFailure(error);
	}
}

/**
 * 調整授權範圍。**全量取代，不是 partial update**：`grantedCapabilities` 送什麼就是
 * 什麼。與開通刻意分開兩支——合成一支 upsert 會讓管理員對著一張過時分頁按下「調整
 * 範圍」時，把一個因故撤回的店靜默復權，而畫面上沒有任何訊號。
 */
export async function PUT(request: NextRequest, context: Context) {
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
		return invalidRequest("XHR_REQUIRED", "授權設定請求必須由主控台發出");
	}

	const adminToken = await adminSession.token(request);
	if (!adminToken) return unauthorized();

	if (isRequestTooLarge(request)) {
		return invalidRequest("REQUEST_TOO_LARGE", "授權設定內容過大");
	}

	const body: unknown = await request.json().catch(() => null);
	const idempotencyKey =
		typeof body === "object" && body !== null
			? (body as { idempotencyKey?: unknown }).idempotencyKey
			: undefined;
	if (!isValidAuthorizationIdempotencyKey(idempotencyKey)) {
		return invalidRequest(
			"IDEMPOTENCY_KEY_REQUIRED",
			"調整代行授權需要有效的 Idempotency-Key",
		);
	}
	const scope = parseAuthorizationScopeBody(body);
	if (!scope.ok) return scope.response;

	try {
		const upstream = await adminUpstreamFetch(
			DELEGATED_OPERATION_UPSTREAM.authorization(storeId),
			{
				method: "PUT",
				adminToken,
				idempotencyKey,
				// storeId 只走 path，不進 body：同一個值出現在兩處，遲早有一次不一致。
				body: scope.value,
			},
		);
		const payload = await readUpstreamJson(upstream);
		if (!upstream.ok) return upstreamRejection(payload, upstream.status);

		const parsed = parseDelegationAuthorizationUpdate(
			unwrapUpstreamData(payload),
		);
		if (!parsed) return contractViolation("調整代行授權回應無法驗證");
		if (parsed.authorization.storeId !== storeId) {
			return contractViolation("調整結果與請求對象不符");
		}
		return ok(parsed);
	} catch (error) {
		return transportFailure(error);
	}
}
