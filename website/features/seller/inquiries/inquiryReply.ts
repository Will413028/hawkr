import { goTrimmedRuneCount } from "@hawkr/api/text/server-text";
import { INQUIRY_REPLY_MAX_LENGTH } from "@/lib/seller/inquiries";

export interface ReplyDraftState {
	length: number;
	tooLong: boolean;
	canSubmit: boolean;
}

/**
 * 回覆表單的送出條件：內文去掉頭尾空白後有字、不超過上限（以 rune 計，與伺服器同一個算法），沒有上傳失敗的附件，
 * 且不在送出中。附件張數的上限由伺服器給（`replyAttachmentMax`），由上傳元件限制。
 */
export function replyDraftState(input: {
	body: string;
	hasFailedPhoto: boolean;
	busy: boolean;
}): ReplyDraftState {
	const length = goTrimmedRuneCount(input.body);
	const tooLong = length > INQUIRY_REPLY_MAX_LENGTH;
	return {
		length,
		tooLong,
		canSubmit: length > 0 && !tooLong && !input.hasFailedPhoto && !input.busy,
	};
}

/**
 * `evidence_staged_tokens[i]` 指的是送出的第 i 個 token；用 token 記錄錯誤，之後增刪照片才不會標錯張。
 */
export function itemErrorsByToken(
	byIndex: Record<number, string>,
	submittedTokens: readonly string[],
): Record<string, string> {
	return Object.fromEntries(
		Object.entries(byIndex).flatMap(([index, message]) => {
			const token = submittedTokens[Number(index)];
			return token === undefined ? [] : [[token, message]];
		}),
	);
}
