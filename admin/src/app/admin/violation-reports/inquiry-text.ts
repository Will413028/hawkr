import {
	CaseInquiryKind,
	CaseInquiryReplyChannel,
	CaseInquiryState,
	CaseInquiryWithdrawCause,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/case_inquiry_pb";
import { unknownText } from "@/lib/ui-text";

// 案件詢問的名稱（3c-b 設計書 §5.4 (a)(b) I6、I7、I10、I13）。這些列舉只有 admin 用，不在共用文案目錄裡，所以是 admin 的單端表
// （UNSPECIFIED 不在表裡）；不認得的值與 UNSPECIFIED 顯示「未知」。

function textOf<E extends number>(
	table: Partial<Record<E, string>>,
	value: E,
): string {
	return table[value] ?? unknownText();
}

const KIND_TEXT: Record<
	Exclude<CaseInquiryKind, CaseInquiryKind.UNSPECIFIED>,
	string
> = {
	[CaseInquiryKind.RECORDS]: "提供紀錄",
	[CaseInquiryKind.EXPLANATION]: "請說明",
};

/** 詢問的種類選項，順序同契約。 */
export const INQUIRY_KIND_OPTIONS: readonly Exclude<
	CaseInquiryKind,
	CaseInquiryKind.UNSPECIFIED
>[] = [CaseInquiryKind.RECORDS, CaseInquiryKind.EXPLANATION];

export function inquiryKindText(kind: CaseInquiryKind): string {
	return textOf<CaseInquiryKind>(KIND_TEXT, kind);
}

const STATE_TEXT: Record<
	Exclude<CaseInquiryState, CaseInquiryState.UNSPECIFIED>,
	string
> = {
	[CaseInquiryState.OPEN]: "等賣家回覆",
	[CaseInquiryState.REPLIED]: "已回覆",
	[CaseInquiryState.WITHDRAWN]: "已撤回",
};

export function inquiryStateText(state: CaseInquiryState): string {
	return textOf<CaseInquiryState>(STATE_TEXT, state);
}

const CHANNEL_TEXT: Record<
	Exclude<CaseInquiryReplyChannel, CaseInquiryReplyChannel.UNSPECIFIED>,
	string
> = {
	[CaseInquiryReplyChannel.PHONE]: "電話",
	[CaseInquiryReplyChannel.EMAIL]: "Email",
	[CaseInquiryReplyChannel.MESSAGING]: "通訊軟體",
	[CaseInquiryReplyChannel.OTHER]: "其他",
};

/** 回覆管道的選項，順序同契約。 */
export const INQUIRY_CHANNEL_OPTIONS: readonly Exclude<
	CaseInquiryReplyChannel,
	CaseInquiryReplyChannel.UNSPECIFIED
>[] = [
	CaseInquiryReplyChannel.PHONE,
	CaseInquiryReplyChannel.EMAIL,
	CaseInquiryReplyChannel.MESSAGING,
	CaseInquiryReplyChannel.OTHER,
];

export function inquiryChannelText(channel: CaseInquiryReplyChannel): string {
	return textOf<CaseInquiryReplyChannel>(CHANNEL_TEXT, channel);
}

const WITHDRAW_CAUSE_TEXT: Record<
	Exclude<CaseInquiryWithdrawCause, CaseInquiryWithdrawCause.UNSPECIFIED>,
	string
> = {
	[CaseInquiryWithdrawCause.ADMIN]: "管理員撤回",
	[CaseInquiryWithdrawCause.CASE_CLOSED]: "案件結案時撤回",
};

export function inquiryWithdrawCauseText(
	cause: CaseInquiryWithdrawCause,
): string {
	return textOf<CaseInquiryWithdrawCause>(WITHDRAW_CAUSE_TEXT, cause);
}
