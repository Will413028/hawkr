import { type NextRequest, NextResponse } from "next/server";
import {
	isDelegatedOperationPreflightDenied,
	parseDelegatedOperationPreflight,
	parseDelegatedOperationPreflightDenial,
} from "@/features/delegated-operations/model";
import { isSafeOpaqueId } from "@/features/delegated-operations/policy";
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

type Context = { params: Promise<{ storeId: string }> };

export async function GET(request: NextRequest, context: Context) {
	const { storeId } = await context.params;
	if (!isSafeOpaqueId(storeId)) {
		return NextResponse.json(
			{ code: "INVALID_STORE_ID", message: "商店識別碼格式不正確" },
			{ status: 400 },
		);
	}

	const adminToken = await adminSession.token(request);
	if (!adminToken) {
		return NextResponse.json(
			{ code: 401, message: "Unauthorized" },
			{ status: 401 },
		);
	}

	try {
		const upstream = await adminUpstreamFetch(
			DELEGATED_OPERATION_UPSTREAM.preflight(storeId),
			{ adminToken },
		);
		const payload = await readUpstreamJson(upstream);
		if (upstream.status === 404) {
			return NextResponse.json(
				{
					code: "DELEGATED_OPERATION_UNAVAILABLE",
					message: "此環境尚未啟用代行營運服務",
				},
				{ status: 404 },
			);
		}
		if (!upstream.ok) {
			return NextResponse.json(safeUpstreamError(payload), {
				status: upstream.status,
			});
		}

		const data = unwrapUpstreamData(payload);
		if (isDelegatedOperationPreflightDenied(data)) {
			// 原因由後端下發（deniedLabel），不在這裡改寫也不在前端建對照表：
			// 9269「已有其他管理員正在代行」、9286「從未開通」、9264「已撤回或已到期」、
			// 9272「已停權或關閉」的下一步完全不同，壓成同一句等於把後端已經講清楚的事丟掉。
			// 這是 `safeUpstreamError`「不透傳上游訊息」的**例外**，界線是：這一欄
			// 屬於已定義的 preflight 契約、且經過 parser 的長度／控制字元校驗，
			// 不是任意服務的自由錯誤字串。
			const denial = parseDelegatedOperationPreflightDenial(data);
			return NextResponse.json(
				{
					code: "DELEGATED_OPERATION_DENIED",
					message: denial?.deniedLabel ?? "此店家目前不可建立代行工作階段",
					...(denial ? { deniedCode: denial.deniedCode } : {}),
				},
				{ status: 403 },
			);
		}
		const preflight = parseDelegatedOperationPreflight(data);
		if (!preflight || preflight.target.storeId !== storeId) {
			return NextResponse.json(
				{
					code: "INVALID_DELEGATED_OPERATION_CONTRACT",
					message: "代行營運資格回應無法驗證",
				},
				{ status: 502 },
			);
		}

		return NextResponse.json(
			{ code: 0, data: preflight },
			{ headers: { "Cache-Control": "no-store" } },
		);
	} catch (error) {
		const timedOut = error instanceof Error && error.name === "AbortError";
		return NextResponse.json(
			{
				code: timedOut ? "UPSTREAM_TIMEOUT" : "UPSTREAM_UNREACHABLE",
				message: timedOut ? "代行營運資格檢查逾時" : "無法連線至代行營運服務",
			},
			{ status: timedOut ? 504 : 502 },
		);
	}
}
