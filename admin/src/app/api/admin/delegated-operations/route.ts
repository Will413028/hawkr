import { type NextRequest, NextResponse } from "next/server";
import {
	buildDelegatedSessionSearch,
	DELEGATED_AUDIT_MAX_PAGE_SIZE,
	isDelegatedSessionId,
	parseDelegatedOperationSessionPage,
} from "@/features/delegated-operations/governance";
import {
	canRenderDelegatedWorkspace,
	deriveDelegatedOperationState,
	isDelegatedOperationPreflightDenied,
	isSameDelegatedOperationSession,
	parseDelegatedOperationPreflight,
	parseDelegatedOperationSessionSummary,
	parseStartDelegatedOperationRequest,
} from "@/features/delegated-operations/model";
import {
	contractViolation,
	invalidRequest,
	ok,
	pickDisplayCodeFilter,
	pickNumericIdFilter,
	pickPositiveInt,
	pickTimestamp,
	transportFailure,
	unauthorized,
	upstreamRejection,
} from "@/features/delegated-operations/server/governance-route";
import {
	buildHandoffLocation,
	isDelegatedHandoffCode,
	isSameOriginSubmission,
	WEB_ORIGIN,
} from "@/features/delegated-operations/server/handoff";
import {
	adminUpstreamFetch,
	DELEGATED_OPERATION_UPSTREAM,
	readUpstreamJson,
	safeUpstreamError,
	unwrapUpstreamData,
} from "@/features/delegated-operations/server/upstream";
import { adminSession } from "@/lib/session";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

type UnknownRecord = Record<string, unknown>;
const MAX_START_REQUEST_BYTES = 16 * 1024;

/**
 * 這支 route 的唯一呼叫者是瀏覽器的**頂層表單導覽**：工作區在另一個 host，
 * 操作者必須跟著 303 走過去，而 `fetch` 讀不到 3xx 的 `Location`
 * （redirect: "manual" 得到的是 opaqueredirect，狀態 0、標頭全空）。
 * 因此成功與失敗一律以 303 作答（POST/Redirect/GET），失敗把機器可讀的碼
 * 帶回商家頁由入口元件顯示。
 */
const REDIRECT_HEADERS: Record<string, string> = {
	// Location 帶著一次性兌換碼，不得進任何快取。
	// `Referrer-Policy` 不在這裡設：`next.config.ts` 的 `headers()` 會覆蓋同名值
	// （實測本機回應為全站的 strict-origin-when-cross-origin），寫了也不生效。
	// 真正需要壓住 Referer 的是「帶著 ?code= 的那一份文件」，而那一跳在 website
	// 的 `/adm/enter` 自己送 `Referrer-Policy: no-referrer`。
	"Cache-Control": "no-store",
};

function isRecord(value: unknown): value is UnknownRecord {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isValidOperationIdempotencyKey(value: unknown): value is string {
	return (
		typeof value === "string" &&
		value.length >= 8 &&
		value.length <= 128 &&
		/^[A-Za-z0-9._:-]+$/u.test(value)
	);
}

function redirect(location: string): Response {
	return new Response(null, {
		status: 303,
		headers: { ...REDIRECT_HEADERS, Location: location },
	});
}

interface StartFailure {
	code: string | number;
	storeId: string;
	idempotencyKey?: string;
	traceId?: string;
}

/**
 * 失敗導回該商家頁並帶上原因碼；未兌換的請求保留 idempotency key，讓對帳失敗
 * 後能查回同一個 session。已兌換的請求不帶 key，重新開始時建立新的請求。
 *
 * 只有「已經認得出目標店家」的失敗走這條；表單本身不成形（媒體型別、長度、
 * 欄位解析）的失敗沒有可歸屬的店家，以狀態碼直接作答 —— 那些從自家表單走
 * 不到，導回一個沒有上下文的列表頁只會讓每一列都跳出同一則錯誤。
 */
function startFailure(failure: StartFailure): Response {
	const query = new URLSearchParams({
		delegation_error: String(failure.code),
		delegation_store: failure.storeId,
	});
	if (failure.idempotencyKey)
		query.set("delegation_key", failure.idempotencyKey);
	if (failure.traceId) query.set("delegation_trace", failure.traceId);
	return redirect(
		`/admin/merchants/${encodeURIComponent(failure.storeId)}?${query.toString()}`,
	);
}

async function readBoundedForm(
	request: NextRequest,
): Promise<{ value: URLSearchParams | null; tooLarge: boolean }> {
	const declaredLength = request.headers.get("content-length");
	if (
		declaredLength &&
		/^\d+$/u.test(declaredLength) &&
		Number(declaredLength) > MAX_START_REQUEST_BYTES
	) {
		return { value: null, tooLarge: true };
	}
	if (!request.body) return { value: null, tooLarge: false };

	const reader = request.body.getReader();
	const decoder = new TextDecoder();
	let bytes = 0;
	let text = "";
	while (true) {
		const { done, value } = await reader.read();
		if (done) break;
		bytes += value.byteLength;
		if (bytes > MAX_START_REQUEST_BYTES) {
			await reader.cancel();
			return { value: null, tooLarge: true };
		}
		text += decoder.decode(value, { stream: true });
	}
	text += decoder.decode();
	return { value: new URLSearchParams(text), tooLarge: false };
}

function optionalField(form: URLSearchParams, name: string): string | null {
	const raw = form.get(name);
	return raw === null || raw.trim().length === 0 ? null : raw.trim();
}

/**
 * 工作階段列表（治理回看，56 §11.3 P1）。與同檔的 POST 是同一條上游路徑的兩個方法，
 * 但性質相反：POST 建立一次代行授權，GET 只回看已經發生的事。
 *
 * ⚠️ 身分只有 ADM session，**不得**接受 operation credential（`adminUpstreamFetch` 這裡
 * 不傳 `operationSession`）：這一支列得出全平台的代行活動，交給憑證持有者等於讓被治理
 * 的人自己讀治理視角。
 *
 * ⚠️ 篩選值不合法時回 400 而不是落地成「不篩選」：後者會回全平台的工作階段，而操作者
 * 以為畫面上這一頁已經縮到他打的那一家店了。
 */
export async function GET(request: NextRequest) {
	const adminToken = await adminSession.token(request);
	if (!adminToken) return unauthorized();

	const params = request.nextUrl.searchParams;
	const page = pickPositiveInt(params.get("page"), 1);
	const pageSize = pickPositiveInt(params.get("page_size"), 20);
	const actorAdminId = pickNumericIdFilter(params.get("actor_admin_id"));
	const storeId = pickNumericIdFilter(params.get("store_id"));
	const displayCode = pickDisplayCodeFilter(params.get("display_code"));
	const dateFrom = pickTimestamp(params.get("date_from"));
	const dateTo = pickTimestamp(params.get("date_to"));
	if (
		page === null ||
		pageSize === null ||
		pageSize > DELEGATED_AUDIT_MAX_PAGE_SIZE ||
		!actorAdminId.ok ||
		!storeId.ok ||
		!displayCode.ok ||
		!dateFrom.ok ||
		!dateTo.ok
	) {
		return invalidRequest("INVALID_SESSION_QUERY", "工作階段查詢條件不正確");
	}

	const search = buildDelegatedSessionSearch({
		page,
		pageSize,
		actorAdminId: actorAdminId.value,
		storeId: storeId.value,
		displayCode: displayCode.value,
		dateFrom: dateFrom.value,
		dateTo: dateTo.value,
	});

	try {
		const upstream = await adminUpstreamFetch(
			`${DELEGATED_OPERATION_UPSTREAM.sessions}?${search.toString()}`,
			{ adminToken },
		);
		const payload = await readUpstreamJson(upstream);
		if (!upstream.ok) return upstreamRejection(payload, upstream.status);

		const parsed = parseDelegatedOperationSessionPage(
			unwrapUpstreamData(payload),
		);
		if (!parsed) return contractViolation("代行工作階段列表回應無法驗證");
		return ok(parsed);
	} catch (error) {
		return transportFailure(error);
	}
}

export async function POST(request: NextRequest) {
	// 頂層表單提交送不出自訂標頭，原本靠 Idempotency-Key 標頭順帶擋下的跨站
	// 提交改由 Origin 檢查承擔（`SameSite=Lax` 是另一層）。
	if (!isSameOriginSubmission(request.headers)) {
		return NextResponse.json(
			{ code: "CROSS_ORIGIN_REJECTED", message: "來源不符" },
			{ status: 403 },
		);
	}

	const adminToken = await adminSession.token(request);
	if (!adminToken) return unauthorized();

	const mediaType = request.headers
		.get("content-type")
		?.split(";", 1)[0]
		.trim()
		.toLowerCase();
	if (mediaType !== "application/x-www-form-urlencoded") {
		return NextResponse.json(
			{ code: "FORM_REQUIRED", message: "代行營運申請必須以表單送出" },
			{ status: 415 },
		);
	}

	const body = await readBoundedForm(request);
	if (body.tooLarge) {
		return NextResponse.json(
			{ code: "REQUEST_TOO_LARGE", message: "代行營運申請內容過大" },
			{ status: 413 },
		);
	}
	const form = body.value;
	if (!form) {
		return NextResponse.json(
			{ code: "INVALID_REQUEST", message: "代行營運申請內容不完整" },
			{ status: 400 },
		);
	}
	const idempotencyKey = form.get("idempotencyKey");
	const ttlRaw = optionalField(form, "ttlSeconds");
	const startRequest = parseStartDelegatedOperationRequest({
		storeId: form.get("storeId"),
		reasonCode: form.get("reasonCode"),
		caseReference: optionalField(form, "caseReference"),
		...(ttlRaw === null ? {} : { ttlSeconds: Number(ttlRaw) }),
	});
	if (!startRequest) {
		return NextResponse.json(
			{ code: "INVALID_REQUEST", message: "代行營運申請內容不完整" },
			{ status: 400 },
		);
	}
	if (!isValidOperationIdempotencyKey(idempotencyKey)) {
		return NextResponse.json(
			{
				code: "IDEMPOTENCY_KEY_REQUIRED",
				message: "建立代行營運工作階段需要有效的 Idempotency-Key",
			},
			{ status: 400 },
		);
	}
	// 沒有設定兌換去向就沒有安全的導向可走，且絕不從 Host 推導。在打任何上游
	// 之前擋下：否則會建出一個沒有人接得住的工作階段，佔住 partial UNIQUE。
	if (!WEB_ORIGIN) {
		return startFailure({
			code: "WEB_ORIGIN_NOT_CONFIGURED",
			storeId: startRequest.storeId,
			idempotencyKey,
		});
	}

	try {
		// Re-fetch preflight on the server so browser-supplied capabilities, TTL,
		// reason codes, or case rules can never define the authority request.
		const preflightUpstream = await adminUpstreamFetch(
			DELEGATED_OPERATION_UPSTREAM.preflight(startRequest.storeId),
			{ adminToken },
		);
		const preflightPayload = await readUpstreamJson(preflightUpstream);
		if (preflightUpstream.status === 404) {
			return startFailure({
				code: "DELEGATED_OPERATION_UNAVAILABLE",
				storeId: startRequest.storeId,
				idempotencyKey,
			});
		}
		if (!preflightUpstream.ok) {
			const error = safeUpstreamError(preflightPayload);
			return startFailure({
				code: error.code,
				storeId: startRequest.storeId,
				idempotencyKey,
				traceId: error.traceId,
			});
		}
		const preflightData = unwrapUpstreamData(preflightPayload);
		if (isDelegatedOperationPreflightDenied(preflightData)) {
			return startFailure({
				code: "DELEGATED_OPERATION_DENIED",
				storeId: startRequest.storeId,
				idempotencyKey,
			});
		}
		const preflight = parseDelegatedOperationPreflight(preflightData);
		if (!preflight || preflight.target.storeId !== startRequest.storeId) {
			return startFailure({
				code: "INVALID_DELEGATED_OPERATION_CONTRACT",
				storeId: startRequest.storeId,
				idempotencyKey,
			});
		}
		const selectedReason = preflight.reasonOptions.find(
			(option) => option.code === startRequest.reasonCode,
		);
		if (!selectedReason) {
			return startFailure({
				code: "DELEGATED_OPERATION_REASON_DENIED",
				storeId: startRequest.storeId,
				idempotencyKey,
			});
		}
		// 只看被選中的那一個原因。preflight 頂層的 `caseReferenceRequired` 是
		// 「任一選項需要」的 OR（gateway `admin_delegation.go` 的 DTO 註解），
		// 拿它當本次必填等於：選單裡只要還有一個需要編號的原因，每一個不需要
		// 編號的原因都會被擋在 CASE_REFERENCE_REQUIRED。
		if (
			selectedReason.caseReferenceRequired === true &&
			!startRequest.caseReference
		) {
			return startFailure({
				code: "CASE_REFERENCE_REQUIRED",
				storeId: startRequest.storeId,
				idempotencyKey,
			});
		}

		// The dialog may pick a shorter window; the preflight value stays the
		// ceiling so a browser-supplied number can only narrow the audited window.
		const requestedTtlSeconds = Math.min(
			startRequest.ttlSeconds ?? preflight.ttlSeconds,
			preflight.ttlSeconds,
		);
		const authoritativeStartRequest = {
			...startRequest,
			capabilities: preflight.capabilities,
			ttlSeconds: requestedTtlSeconds,
		};
		const upstream = await adminUpstreamFetch(
			DELEGATED_OPERATION_UPSTREAM.create,
			{
				method: "POST",
				adminToken,
				idempotencyKey,
				body: authoritativeStartRequest,
			},
		);
		const payload = await readUpstreamJson(upstream);
		// 建立端點的 404 有兩種來源，下一步相反：路由不存在（這個環境沒有代行
		// 營運，該找維運）vs 9286「這家店從來沒有授權列」（該去按開通）。以
		// envelope 的業務碼分派而不是以 status 分派，否則後者會被講成前者。
		// 帶不出業務碼的 404 才是真的「這裡沒有這支端點」。
		if (upstream.status === 404) {
			const error = safeUpstreamError(payload);
			return startFailure({
				code:
					typeof error.code === "number"
						? error.code
						: "DELEGATED_OPERATION_UNAVAILABLE",
				storeId: startRequest.storeId,
				idempotencyKey,
				...(error.traceId ? { traceId: error.traceId } : {}),
			});
		}
		if (!upstream.ok) {
			const error = safeUpstreamError(payload);
			return startFailure({
				code: error.code,
				storeId: startRequest.storeId,
				idempotencyKey,
				traceId: error.traceId,
			});
		}

		// Backend contract; this is never substituted with a dev fixture:
		// { code: 0, data: { handoffCode?, session } }.
		const data = unwrapUpstreamData(payload);
		const handoffCode = isRecord(data) ? data.handoffCode : null;
		if (
			!isRecord(data) ||
			"operationCredential" in data ||
			(!isDelegatedHandoffCode(handoffCode) &&
				handoffCode !== undefined &&
				handoffCode !== "")
		) {
			return startFailure({
				code: "INVALID_DELEGATED_OPERATION_CONTRACT",
				storeId: startRequest.storeId,
				idempotencyKey,
			});
		}
		const summary = parseDelegatedOperationSessionSummary(data.session);
		const sessionId = isRecord(data.session) ? data.session.sessionId : null;
		const validationNow = Date.now();
		const state = summary
			? deriveDelegatedOperationState(
					summary,
					startRequest.storeId,
					validationNow,
				)
			: "invalid";
		const onlyPreflightCapabilities = summary?.capabilities.every(
			(capability) => preflight.capabilities.includes(capability),
		);
		const echoedPreflightContext = summary
			? summary.target.storeId === preflight.target.storeId &&
				summary.target.sellerAccountId === preflight.target.sellerAccountId &&
				summary.actor.adminId === preflight.actor.adminId &&
				summary.reason.code === startRequest.reasonCode &&
				summary.caseReference === startRequest.caseReference &&
				Date.parse(summary.expiresAt) - Date.parse(summary.issuedAt) <=
					requestedTtlSeconds * 1000 &&
				Date.parse(summary.expiresAt) - validationNow <=
					requestedTtlSeconds * 1000
			: false;
		if (
			!summary ||
			!isDelegatedSessionId(sessionId) ||
			!canRenderDelegatedWorkspace(state) ||
			!onlyPreflightCapabilities ||
			!echoedPreflightContext
		) {
			return startFailure({
				code: "INVALID_DELEGATED_OPERATION_CONTRACT",
				storeId: startRequest.storeId,
				idempotencyKey,
			});
		}
		if (!isDelegatedHandoffCode(handoffCode)) {
			return startFailure({
				code: "DELEGATED_OPERATION_ALREADY_REDEEMED",
				storeId: startRequest.storeId,
			});
		}

		// Reconcile by session ID under the administrator's own identity.
		const currentUpstream = await adminUpstreamFetch(
			DELEGATED_OPERATION_UPSTREAM.summary(sessionId),
			{
				adminToken,
			},
		);
		const currentPayload = await readUpstreamJson(currentUpstream);
		if (!currentUpstream.ok) {
			return startFailure({
				code: "DELEGATED_OPERATION_RECONCILIATION_FAILED",
				storeId: startRequest.storeId,
				idempotencyKey,
			});
		}
		const reconciledData = unwrapUpstreamData(currentPayload);
		const reconciledSession = isRecord(reconciledData)
			? reconciledData.session
			: null;
		const currentSummary =
			parseDelegatedOperationSessionSummary(reconciledSession);
		const finalNow = Date.now();
		const finalStartedState = deriveDelegatedOperationState(
			summary,
			startRequest.storeId,
			finalNow,
		);
		const currentState = currentSummary
			? deriveDelegatedOperationState(
					currentSummary,
					startRequest.storeId,
					finalNow,
				)
			: "invalid";
		if (
			!currentSummary ||
			!isRecord(reconciledSession) ||
			reconciledSession.sessionId !== sessionId ||
			!canRenderDelegatedWorkspace(finalStartedState) ||
			!canRenderDelegatedWorkspace(currentState) ||
			!isSameDelegatedOperationSession(summary, currentSummary)
		) {
			return startFailure({
				code: "DELEGATED_OPERATION_RECONCILIATION_MISMATCH",
				storeId: startRequest.storeId,
				idempotencyKey,
			});
		}

		// 對帳通過才把兌換碼交出去；工作區的憑證由 website 在兌換時取得。
		return redirect(buildHandoffLocation(WEB_ORIGIN, handoffCode));
	} catch (error) {
		const timedOut = error instanceof Error && error.name === "AbortError";
		return startFailure({
			code: timedOut ? "UPSTREAM_TIMEOUT" : "UPSTREAM_UNREACHABLE",
			storeId: startRequest.storeId,
			idempotencyKey,
		});
	}
}
