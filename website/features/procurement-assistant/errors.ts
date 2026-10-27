export interface ProcurementError {
	code: string;
	detail: string;
}

/**
 * 這一輪打在一個已經不存在的 request 上——對話被刪掉（後端 2026-08-11：
 * 軟刪的對話不再接受新 turn）、或深連結指向別人的／不存在的需求。後端一律
 * 回 404 `"request not found"`，那是內部字串，不能直接端到使用者面前。
 */
export const CONVERSATION_GONE: ProcurementError = {
	code: "CONVERSATION_GONE",
	detail: "這個對話已經不在了，請開一個新對話重新開始。",
};

/** 後端把「找不到 request」表達成 404＋這句英文。 */
export function isRequestNotFound(message: string): boolean {
	return message.toLowerCase().includes("request not found");
}

/**
 * 把一個 ApiClientError 收斂成畫面用的錯誤。turns／answers／rewinds 三個端點
 * 都是對同一個 requestId 下命令，所以「request 不在了」對三者是同一件事，
 * 統一在這裡翻譯，不在每個呼叫點各寫一次。
 */
export function procurementErrorFromStatus(
	status: number,
	reason: string,
	message: string,
): ProcurementError {
	if (status === 404 || isRequestNotFound(message)) return CONVERSATION_GONE;
	return parseProcurementError(reason, message);
}

/**
 * procurement 端點的業務錯誤現在是 {code, reason, message} 三個獨立欄位
 * （backend-payuni AIProcurementService 遷移，2026-08-16）——reason 是機器碼，
 * message 是純人看文字，不再需要切字串還原。
 */
export function parseProcurementError(
	reason: string,
	message: string,
): ProcurementError {
	if (!reason) {
		return { code: "UNKNOWN", detail: message };
	}
	return { code: reason, detail: message };
}
