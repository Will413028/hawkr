import { Code, ConnectError } from "@connectrpc/connect";
import { isNoResponse } from "@hawkr/api/connect/errors";

/** permission_denied 與其他沒有專屬文字的失敗：刻意不分成因（後端對這一類回同一段訊息）。 */
export const STEP_UP_DEFAULT_MESSAGE =
	"驗證沒有通過。請確認您登入的 Google 帳號，就是收到這封邀請信的那個信箱。";

/** session 不存在或已失效（unauthenticated、或伺服器沒給 nonce）：要回到信件連結重新兌換。 */
export const STEP_UP_SESSION_GONE_MESSAGE =
	"連結的使用期限已過，請重新開啟信件中的連結。";

/**
 * Google step-up（VerifySession）失敗時顯示的文字，依 Connect 的 code 分：
 * - unauthenticated：沒有 session 或 session 已過期，要回到信件連結重新兌換。
 * - resource_exhausted：每個來源的嘗試額度用完。
 * - unavailable、deadline_exceeded 與沒收到回應的傳輸失敗：Google 公鑰暫時問不到或網路問題，可以稍後重試。
 * - 其餘（含 permission_denied）：不分成因的預設文字，不讓人逐項試探。
 */
export function stepUpFailureMessage(error: unknown): string {
	const err = ConnectError.from(error);
	if (err.code === Code.Unauthenticated) {
		return STEP_UP_SESSION_GONE_MESSAGE;
	}
	if (err.code === Code.ResourceExhausted) {
		return "短時間內驗證太多次。請稍待幾分鐘後再試一次。";
	}
	if (
		err.code === Code.Unavailable ||
		err.code === Code.DeadlineExceeded ||
		isNoResponse(err)
	) {
		return "暫時無法驗證，請稍後再試。";
	}
	return STEP_UP_DEFAULT_MESSAGE;
}

/**
 * 畫 Google 按鈕之前的檢查：伺服器沒給 nonce ＝ 這張邀請沒有 session，帶不出能被接受的 token，
 * 回要顯示的文字（不畫按鈕）；可以畫就回 null。
 */
export function stepUpBlockedMessage(googleNonce: string): string | null {
	return googleNonce ? null : STEP_UP_SESSION_GONE_MESSAGE;
}
