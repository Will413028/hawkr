import {
	createDelegatedCookieOptions,
	getDelegatedCookieName,
	isSafeOperationCredential,
	serializeDelegatedCookie,
} from "@/features/delegated-operations/server/session";
import {
	DELEGATION_HANDOFF_INVALID_CODE,
	readDelegatedSessionSummary,
} from "@/features/delegated-operations/state/session";
import {
	callDelegatedUpstream,
	DelegatedUpstreamUnavailableError,
} from "../api/_upstream";

/**
 * 交接碼兌換（56 號 §4.5 L1028、S6）。
 *
 * ADM 與代行工作區在兩個不同 host，兩邊的憑證都是 host-only cookie 彼此讀不到。
 * 跨 host 傳遞的是一把**單次、60 秒、與工作階段綁定的兌換碼**；credential 本身
 * 從不離開伺服器之間的連線 —— 它在這支 route handler 內被換到、直接寫進 cookie。
 *
 * ⚠️ **兌換碼絕不出現在 303 的 `Location` 裡**（它是「單獨持有即可操作」的憑證，
 * 放進 URL 等同放進瀏覽器歷史、`Referer` 與任何記錄 URL 的中介層）。
 * ⚠️ **不得自動重試**：code 是單次的，重試必然再失敗。
 * ⚠️ **不得把 gateway 的 `message` 顯示出來**（message 不透傳，§4.4.4）。
 */

export const dynamic = "force-dynamic";

/** `dhof_` ＋ 43 字 base64url ＝ 48 字（§8.1.6）。 */
const HANDOFF_CODE_PATTERN = /^dhof_[A-Za-z0-9_-]{43}$/;

const SECURITY_HEADERS: Record<string, string> = {
	// 導向工作區的那一跳不得把 ?code= 帶進 Referer。
	"Referrer-Policy": "no-referrer",
	"Cache-Control": "private, no-store",
};

function redirect(location: string, setCookie?: string): Response {
	const headers = new Headers({ ...SECURITY_HEADERS, Location: location });
	if (setCookie) headers.append("Set-Cookie", setCookie);
	return new Response(null, { status: 303, headers });
}

function failure(reason: "handoff_invalid" | "upstream"): Response {
	return redirect(`/adm/unavailable?reason=${reason}`);
}

export async function GET(request: Request): Promise<Response> {
	const code = new URL(request.url).searchParams.get("code") ?? "";
	// 形狀不對就地拒絕：不打上游，也不用另一個碼區分「格式錯」與「不存在」——
	// 任何能區分兩者的回應都是一支可以用來收窄猜測空間的探測器。
	if (!HANDOFF_CODE_PATTERN.test(code)) return failure("handoff_invalid");

	let result: Awaited<ReturnType<typeof callDelegatedUpstream>>;
	try {
		result = await callDelegatedUpstream({
			method: "POST",
			path: "/admin/delegated-operations/handoff/redeem",
			timeout: "session",
			// Cloudflare 邊緣寫的、client 設不了。不帶就會簽成 p="ssr"，整個 ADM 的
			// 代行流量共用一個限流桶（限流主體＝per-admin-IP）。
			visitorIp: request.headers.get("cf-connecting-ip"),
			json: { handoffCode: code },
		});
	} catch (error) {
		if (error instanceof DelegatedUpstreamUnavailableError) {
			return failure("upstream");
		}
		throw error;
	}

	if (!result.ok) {
		// 未知／已兌換／code 逾時／工作階段已不可用四種成因共用 9282，這是刻意的。
		return failure(
			result.code === DELEGATION_HANDOFF_INVALID_CODE
				? "handoff_invalid"
				: "upstream",
		);
	}

	const payload =
		typeof result.data === "object" && result.data !== null
			? (result.data as Record<string, unknown>)
			: null;
	const credential = payload?.operationCredential;
	const summary = readDelegatedSessionSummary(payload?.session);
	if (!isSafeOperationCredential(credential) || !summary) {
		return failure("upstream");
	}

	const options = createDelegatedCookieOptions(summary.expiresAt);
	// 兌換成功但工作階段已經沒有剩餘時間：寫一個 Max-Age ≤ 0 的 cookie 只會讓
	// 操作者進到工作區再被踢出來，症狀看起來像「進去就被踢出來」。
	if (!options) return failure("handoff_invalid");

	return redirect(
		`/adm/${encodeURIComponent(summary.target.storeId)}/products`,
		serializeDelegatedCookie(getDelegatedCookieName(), credential, options),
	);
}
