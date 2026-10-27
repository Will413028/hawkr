import type { MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	CaseInquiryKind,
	CaseInquiryReplyChannel,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/case_inquiry_pb";
import type {
	CreateCaseInquiryRequestSchema,
	RecordCaseInquiryReplyRequestSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { goTrimmedRuneCount, trimGoSpace } from "@hawkr/api/text/server-text";
import { fromLocalInput } from "@/app/admin/merchants/[id]/measure";

// 案件詢問的對話框：輸入的驗證、請求組裝、失敗句（3c-b 設計書 §5.3、§5.4 (b)）。能不能做來自伺服器（Case.can_create_inquiry、
// CaseInquiry.can_record_reply、can_withdraw）；欄位的長度與範圍由伺服器判斷並回已核准的句子（inquiry.kind_required、message_length、
// channel_required、body_length、receive_time_range、due_range），這裡只擋有核准文案的「沒填給賣家的說明」。

/** 詢問、記下回覆、撤回失敗（網路層，J15）：沒有寫入。 */
export const INQUIRY_FAILED_TEXT =
	"記下失敗，沒有寫入；請確認網路狀態後再試一次。";
/**
 * 選的附件超過伺服器的上限（CaseInquiry.reply_evidence_max）時，在上傳之前擋下的一句。
 * Will 2026-10-20 核准。
 */
export function inquiryEvidenceOverText(max: number): string {
	return `附件最多 ${max} 份，請移除多的檔案。`;
}
/** J6：沒填給賣家的說明。 */
export const INQUIRY_MESSAGE_REQUIRED_TEXT = "請填寫給賣家的說明。";
/** J11：這筆詢問先前已記下回覆，這次沒有再記一筆。 */
export const INQUIRY_ALREADY_REPLIED_TEXT =
	"這筆詢問先前已記下回覆，這次沒有再記一筆。";
/** J14：這筆詢問先前已撤回。 */
export const INQUIRY_ALREADY_WITHDRAWN_TEXT = "這筆詢問先前已撤回。";
/** M4：撤回之後才收到的回覆，不能記在詢問上。 */
export const INQUIRY_LATE_REPLY_TEXT =
	"結案或撤回之後才收到的回覆，請寫在案件的內部註記。";
/** I17：結案對話框在等賣家時的提示（不擋送出）。 */
export const CLOSE_WITH_OPEN_INQUIRY_TEXT =
	"還有一筆詢問在等賣家回覆；結案會撤回它。";

// ---------------------------------------------------------------------------
// 詢問賣家

export interface InquiryForm {
	kind: CaseInquiryKind;
	/** 給賣家的說明。 */
	message: string;
	/** 期限，`datetime-local` 的字串（台北時間）；空字串＝不設，由伺服器取建立後 72 小時（J4）。 */
	dueTime: string;
}

export const EMPTY_INQUIRY_FORM: InquiryForm = {
	kind: CaseInquiryKind.UNSPECIFIED,
	message: "",
	dueTime: "",
};

export function validateInquiry(form: InquiryForm): string | undefined {
	if (goTrimmedRuneCount(form.message) === 0) {
		return INQUIRY_MESSAGE_REQUIRED_TEXT;
	}
	return undefined;
}

/** 輸入內容的鍵：與送出的請求一一對應（去掉前後空白後的內容），request_id 依它換新（同 CreateMeasure）。 */
export function inquiryRequestKey(caseId: string, form: InquiryForm): string {
	return JSON.stringify([
		caseId,
		form.kind,
		trimGoSpace(form.message),
		form.dueTime,
	]);
}

export function buildCreateInquiry(
	requestId: string,
	caseId: string,
	form: InquiryForm,
): MessageInitShape<typeof CreateCaseInquiryRequestSchema> {
	const due = fromLocalInput(form.dueTime);
	return {
		requestId,
		caseId,
		kind: form.kind,
		message: trimGoSpace(form.message),
		...(due ? { dueTime: timestampFromDate(due) } : {}),
	};
}

// ---------------------------------------------------------------------------
// 記下回覆

export interface InquiryReplyForm {
	channel: CaseInquiryReplyChannel;
	/** 收到的時間，`datetime-local` 的字串（台北時間）。 */
	receiveTime: string;
	body: string;
	/** 選的附件檔（份數上限是 CaseInquiry.reply_evidence_max，送出時才上傳成機敏檔）。 */
	files: readonly File[];
}

export function emptyReplyForm(receiveTime: string): InquiryReplyForm {
	return {
		channel: CaseInquiryReplyChannel.UNSPECIFIED,
		receiveTime,
		body: "",
		files: [],
	};
}

/**
 * 回覆的請求。附件已上傳完成，`evidenceTokens` 是機敏上傳的 staged token（B27）。時間讀不出來（沒填）時不帶這個欄位，
 * 由伺服器回 `inquiry.receive_time_range`。時間原樣送出（精確到秒）；與詢問建立同一秒的時間由伺服器視為建立當下，client 不比較。
 */
export function buildInquiryReply(
	inquiryId: string,
	form: InquiryReplyForm,
	evidenceTokens: readonly string[],
): MessageInitShape<typeof RecordCaseInquiryReplyRequestSchema> {
	const receive = fromLocalInput(form.receiveTime);
	return {
		id: inquiryId,
		channel: form.channel,
		body: trimGoSpace(form.body),
		evidenceTokens: [...evidenceTokens],
		...(receive ? { receiveTime: timestampFromDate(receive) } : {}),
	};
}
