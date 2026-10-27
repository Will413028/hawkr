import { ConnectError } from "@connectrpc/connect";
import { errorText, isNoResponse } from "@hawkr/api/connect/errors";
import { submitError } from "@hawkr/api/connect/submit";
import { defaultErrorTextZhTW } from "@hawkr/ui-text/zh-TW";

/**
 * 錯誤顯示給員工的文字：伺服器的 LocalizedMessage，否則該 code 的目錄預設文字（取消當成 internal）。
 * 後台的語系固定 zh-TW（ADR 2026-10-17-multi-locale-architecture D1），所以在這裡綁一次，呼叫點不傳預設文字、也不補寫死的後備句。
 */
export function errorMessage(err: unknown): string {
	return errorText(err, defaultErrorTextZhTW);
}

/**
 * 連線（connect-query）的治理操作失敗：伺服器的 LocalizedMessage 原樣顯示（例如「店家還有未完成的訂單…」），
 * 沒有時用呼叫端的文字。不以訊息字串比對做特判（K1）；VERSION_CONFLICT 與欄位錯誤這些本頁不會用到，照目錄的預設文字顯示。
 */
export function connectErrorText(caught: unknown, fallback: string): string {
	// 網路層失敗（沒有收到回應）換成呼叫端的文字。
	if (isNoResponse(caught)) return fallback;
	// 本頁自己拋出的前置條件錯誤（例如帳號 ID 還沒載入）是一般的 Error：照它的訊息講。
	if (caught instanceof Error && !(caught instanceof ConnectError)) {
		return caught.message || fallback;
	}
	const failure = submitError(caught, defaultErrorTextZhTW);
	return failure.kind === "message" ? failure.message : errorMessage(caught);
}
