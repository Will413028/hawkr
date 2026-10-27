import {
	clearDelegatedCookieOptions,
	getDelegatedCookieName,
	isDelegatedEndProven,
	serializeDelegatedCookie,
} from "@/features/delegated-operations/server/session";
import {
	assertDelegatedBrowserRequest,
	DelegatedRequestRejectedError,
} from "../../_csrf";
import { numericSegment, readOperationCredential } from "../../_forward";
import {
	callDelegatedUpstream,
	DelegatedUpstreamUnavailableError,
	delegatedJson,
	UPSTREAM_FAILURE,
} from "../../_upstream";

/**
 * `POST /adm/api/session/end` → Gateway `DELETE /admin/delegated-operations/current`
 * （56 號 §4.5 L1041、§6.1.5）。
 *
 * 呼叫序（L2787、L2817）：**BFF 先打一次 `GET current` 驗 target 綁定，不符回 409 且
 * 根本不 DELETE**。驗的是「工作區路由的 `storeId` ＝ 工作階段的 target」，與 Gateway
 * 內部已經拿掉的那次 actor 比對是兩件不同的事 —— 一個放著沒動的舊分頁按下「結束
 * 代行」，結束掉的會是**另一家店**正在進行的工作階段。
 *
 * ⚠️ `GET current` 自己失敗時**仍然送 DELETE**：那代表這把 credential 根本換不到工作
 * 階段，沒有任何「別人的工作階段」會被誤結束，而 DELETE 是冪等的、會回
 * `{"ended": true}` 讓操作者乾淨離開。反過來把它擋掉，操作者就卡在「結束結果待確認」。
 *
 * ⚠️ **只有伺服器明確證明已終止才清 cookie**（`isDelegatedEndProven`）：狀態碼本身
 * 不是證明 —— 籠統的 401 可能只是上游拒絕，籠統的 404 可能是端點沒部署，兩者都不得
 * 讓瀏覽器丟掉一把可能還活著的憑證。
 */

export const dynamic = "force-dynamic";

interface EndRequestBody {
	/** 工作區當前路由的 storeId；與工作階段快照的 target 不符即 409。 */
	storeId?: unknown;
}

function readTargetStoreId(body: unknown): string {
	if (typeof body !== "object" || body === null) {
		throw new DelegatedRequestRejectedError("INVALID_BODY", 400);
	}
	const storeId = (body as EndRequestBody).storeId;
	if (typeof storeId !== "string") {
		throw new DelegatedRequestRejectedError("INVALID_BODY", 400);
	}
	return numericSegment(storeId);
}

function targetStoreIdOf(data: unknown): string | null {
	if (typeof data !== "object" || data === null) return null;
	const target = (data as { target?: unknown }).target;
	if (typeof target !== "object" || target === null) return null;
	const storeId = (target as { storeId?: unknown }).storeId;
	return typeof storeId === "string" && storeId !== "" ? storeId : null;
}

export async function POST(request: Request): Promise<Response> {
	let credential: string | null;
	let expectedStoreId: string;
	try {
		assertDelegatedBrowserRequest(request);
		credential = readOperationCredential(request);
		if (!credential) {
			throw new DelegatedRequestRejectedError("CREDENTIAL_MISSING", 401);
		}
		expectedStoreId = readTargetStoreId(await request.json().catch(() => null));
	} catch (error) {
		if (error instanceof DelegatedRequestRejectedError) {
			return delegatedJson({ code: error.code }, error.status);
		}
		throw error;
	}

	try {
		const current = await callDelegatedUpstream({
			method: "GET",
			path: "/admin/delegated-operations/current",
			timeout: "session",
			credential,
			visitorIp: request.headers.get("cf-connecting-ip"),
		});
		if (current.ok) {
			const actualStoreId = targetStoreIdOf(current.data);
			if (actualStoreId !== null && actualStoreId !== expectedStoreId) {
				return delegatedJson(
					{ code: "TARGET_MISMATCH", traceId: current.traceId },
					409,
				);
			}
		}

		const ended = await callDelegatedUpstream({
			method: "DELETE",
			path: "/admin/delegated-operations/current",
			timeout: "session",
			credential,
			visitorIp: request.headers.get("cf-connecting-ip"),
		});
		const proven = isDelegatedEndProven(ended.status, {
			code: ended.code,
			data: ended.data,
		});
		if (!proven) {
			return delegatedJson(
				{
					code: ended.code ?? UPSTREAM_FAILURE,
					traceId: ended.traceId,
				},
				ended.status >= 400 && ended.status <= 599 ? ended.status : 502,
			);
		}

		const body = delegatedJson(
			{ code: 0, data: { ended: true }, traceId: ended.traceId },
			200,
		);
		body.headers.append(
			"Set-Cookie",
			serializeDelegatedCookie(
				getDelegatedCookieName(),
				"",
				clearDelegatedCookieOptions(),
			),
		);
		return body;
	} catch (error) {
		if (error instanceof DelegatedUpstreamUnavailableError) {
			// 逾時＝結果不明：**不清 cookie**。清掉就等於宣稱已結束，而工作階段可能
			// 還活著 —— 操作者會回到 ADM，代行卻仍在有效期內。
			return delegatedJson(
				{ code: UPSTREAM_FAILURE },
				error.timedOut ? 504 : 502,
			);
		}
		throw error;
	}
}
