/**
 * `/vendor/rfq` 的進場決策。
 *
 * 兩種進場：信件連結（fragment 帶 secret，先預覽、由人點擊才兌換），與重新整理或書籤（query 帶
 * 收件人 ID `?r=<id>`，以那張邀請的 cookie 直接讀 session）。兌換成功後網址由前者換成後者。
 * 邀請不可用（`not_found`：到期、撤銷、secret 已換新或不存在）、網址的 `r` 不是合法的收件人 ID、
 * 或沒有那張邀請的有效 session，都顯示連結無效。
 *
 * 錯誤來源有兩種：preview／exchange／GetSession 是 Connect（ConnectError 的 code），RFQ 的讀取是
 * REST（ApiClientError 的 HTTP status）。兩者在這裡收斂成同一組分類。
 */
import { Code, ConnectError } from "@connectrpc/connect";
import { ApiClientError } from "@/lib/api/client";
import { parseInviteSecret, parseRecipientId } from "./invite-link";

export type VendorRfqFailure = "link_invalid" | "rate_limited" | "error";

export interface VendorRfqFailedStep {
	kind: "failed";
	reason: VendorRfqFailure;
}

export type VendorRfqStep =
	| { kind: "preview"; secret: string }
	| { kind: "session"; rfqRecipientId: string }
	| VendorRfqFailedStep;

export function resolveEntryStep(hash: string, search: string): VendorRfqStep {
	const secret = parseInviteSecret(hash);
	// secret 優先：新的一封邀請信可能指向另一筆 RFQ，不沿用網址上舊的收件人。
	if (secret) return { kind: "preview", secret };
	const rfqRecipientId = parseRecipientId(search);
	if (rfqRecipientId) return { kind: "session", rfqRecipientId };
	return { kind: "failed", reason: "link_invalid" };
}

export function resolveInviteFailureStep(error: unknown): VendorRfqFailedStep {
	return { kind: "failed", reason: classifyFailure(error) };
}

/**
 * session 或 RFQ 讀取失敗。只有確定連結／session 不可用（not_found、unauthenticated、invalid_argument、
 * HTTP 401／404）才是連結無效；被節流或後端暫時掛掉時叫業者回去檢查信件連結是錯的指示。
 */
export function resolveSessionFailureStep(error: unknown): VendorRfqFailedStep {
	return { kind: "failed", reason: classifyFailure(error) };
}

function classifyFailure(error: unknown): VendorRfqFailure {
	if (error instanceof ConnectError) {
		// unauthenticated：GetSession 沒有這張邀請的有效 session（cookie 過期、被撤銷或從沒兌換過）；
		// invalid_argument：收件人 ID 不是合法的 ID。
		if (
			error.code === Code.NotFound ||
			error.code === Code.Unauthenticated ||
			error.code === Code.InvalidArgument
		) {
			return "link_invalid";
		}
		if (error.code === Code.ResourceExhausted) return "rate_limited";
		return "error";
	}
	const status = error instanceof ApiClientError ? error.status : 0;
	// 401：gateway 說沒有有效的 vendor session（cookie 過期、撤銷或沒有），同 Connect 的 unauthenticated。
	if (status === 404 || status === 401) return "link_invalid";
	if (status === 429) return "rate_limited";
	return "error";
}
