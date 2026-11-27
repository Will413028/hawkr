import {
	CaseInquiryKind,
	CaseInquiryReplyChannel,
	CaseInquiryState,
	CaseInquiryWithdrawCause,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/case_inquiry_pb";
import { unknownText } from "@/lib/ui-text";


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
	[CaseInquiryReplyChannel.SELLER_CENTER]: "賣家中心",
};

/** 管理員記下系統外回覆時可選的管道，順序同契約；賣家中心是賣家自己回覆的管道，不在選項裡。 */
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
