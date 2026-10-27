import { NextResponse } from "next/server";
import {
	isDelegatedDisplayCode,
	isDelegatedNumericId,
} from "@/features/delegated-operations/governance";
import { safeUpstreamError } from "./upstream";

/**
 * 治理三支 BFF 共用的作答形狀。抽出來的理由不是省行數，是讓三支端點對同一種
 * 失敗只有一個答案：稽核紀錄、摘要、撤銷若各自對「上游拒絕」回不同的形狀，前端
 * 就得為同一件事寫三份錯誤處理，而漏掉的那一份會把失敗顯示成空資料。
 */

export const NO_STORE_HEADERS = { "Cache-Control": "no-store" } as const;

export function unauthorized(): NextResponse {
	return NextResponse.json(
		{ code: 401, message: "Unauthorized" },
		{ status: 401 },
	);
}

export function invalidSessionId(): NextResponse {
	return NextResponse.json(
		{ code: "INVALID_SESSION_ID", message: "工作階段識別碼格式不正確" },
		{ status: 400 },
	);
}

export function invalidRequest(code: string, message: string): NextResponse {
	return NextResponse.json({ code, message }, { status: 400 });
}

/** 上游 message 一律不透傳（可能夾帶憑證或內部拓撲），只留 code 與 traceId。 */
export function upstreamRejection(
	payload: unknown,
	status: number,
): NextResponse {
	return NextResponse.json(safeUpstreamError(payload), { status });
}

/**
 * 回應解析不過＝契約錯，回 502 而不是空資料：一份**看起來是空的**稽核紀錄，與
 * 一份真的沒有任何操作的稽核紀錄，在畫面上長得一模一樣。
 */
export function contractViolation(message: string): NextResponse {
	return NextResponse.json(
		{ code: "INVALID_DELEGATED_OPERATION_CONTRACT", message },
		{ status: 502 },
	);
}

export function transportFailure(error: unknown): NextResponse {
	const timedOut = error instanceof Error && error.name === "AbortError";
	return NextResponse.json(
		{
			code: timedOut ? "UPSTREAM_TIMEOUT" : "UPSTREAM_UNREACHABLE",
			message: timedOut ? "代行營運服務回應逾時" : "無法連線至代行營運服務",
		},
		{ status: timedOut ? 504 : 502 },
	);
}

export function ok(data: unknown): NextResponse {
	return NextResponse.json({ code: 0, data }, { headers: NO_STORE_HEADERS });
}

/**
 * 治理列表兩支（單一工作階段的稽核紀錄、跨工作階段的工作階段列表）共用的查詢參數
 * 檢查。共用而不是各寫一份：兩支對同一個 `page` / `date_from` 若各有一套判準，其中
 * 一套放寬的那天不會有任何東西轉紅，只是那一支開始把越界值送給後端換 9260。
 *
 * 全部走「不合法就當成參數錯」而不是「落地成不篩選」：後者會靜默回全集，而操作者
 * 以為自己已經縮到一家店了。
 */
export function pickPositiveInt(
	raw: string | null,
	fallback: number,
): number | null {
	if (raw === null || raw === "") return fallback;
	if (!/^\d{1,6}$/u.test(raw)) return null;
	const parsed = Number(raw);
	return parsed >= 1 ? parsed : null;
}

/** RFC 3339 UTC；只做形狀檢查，時間語意由後端負責。 */
export function pickTimestamp(raw: string | null): {
	ok: boolean;
	value: string;
} {
	if (raw === null || raw === "") return { ok: true, value: "" };
	return /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?Z$/u.test(raw) &&
		Number.isFinite(Date.parse(raw))
		? { ok: true, value: raw }
		: { ok: false, value: "" };
}

/** `actor_admin_id` / `store_id`：TSID 數字字串，精確比對。 */
export function pickNumericIdFilter(raw: string | null): {
	ok: boolean;
	value: string;
} {
	if (raw === null || raw === "") return { ok: true, value: "" };
	return isDelegatedNumericId(raw)
		? { ok: true, value: raw }
		: { ok: false, value: "" };
}

/** `display_code`：`OPS-` ＋ 13 碼，精確比對。 */
export function pickDisplayCodeFilter(raw: string | null): {
	ok: boolean;
	value: string;
} {
	if (raw === null || raw === "") return { ok: true, value: "" };
	return isDelegatedDisplayCode(raw)
		? { ok: true, value: raw }
		: { ok: false, value: "" };
}
