import { type Timestamp, timestampDate } from "@bufbuild/protobuf/wkt";
import {
	InquiryKind,
	InquiryReplyChannel,
	InquiryState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import type { ChipColor } from "@/components/ui/Chip";
import type { SellerInquiry } from "@/lib/seller/inquiries";

/**
 * `/seller/inquiries` 的顯示規則。契約只給列舉；名詞與句子放在 website 的 messages（`seller.inquiries`），這裡只決定
 * 用哪個 key，UNSPECIFIED 與不認得的值回 undefined，由呼叫端不顯示。能不能回覆讀伺服器的 `canReply`，逾期只是標記。
 */

export const INQUIRIES_PATH = "/seller/inquiries";

export function inquiryHref(inquiry: Pick<SellerInquiry, "id">): string {
	return `${INQUIRIES_PATH}/${encodeURIComponent(inquiry.id)}`;
}

export type InquiryKindKey = "records" | "explanation";
export type InquiryStateKey = "open" | "replied" | "withdrawn";
export type InquiryChannelKey =
	| "phone"
	| "email"
	| "messaging"
	| "other"
	| "sellerCenter";

export function inquiryKindKey(kind: InquiryKind): InquiryKindKey | undefined {
	switch (kind) {
		case InquiryKind.RECORDS:
			return "records";
		case InquiryKind.EXPLANATION:
			return "explanation";
		default:
			return undefined;
	}
}

export function inquiryStateKey(
	state: InquiryState,
): InquiryStateKey | undefined {
	switch (state) {
		case InquiryState.OPEN:
			return "open";
		case InquiryState.REPLIED:
			return "replied";
		case InquiryState.WITHDRAWN:
			return "withdrawn";
		default:
			return undefined;
	}
}

export function inquiryChannelKey(
	channel: InquiryReplyChannel,
): InquiryChannelKey | undefined {
	switch (channel) {
		case InquiryReplyChannel.PHONE:
			return "phone";
		case InquiryReplyChannel.EMAIL:
			return "email";
		case InquiryReplyChannel.MESSAGING:
			return "messaging";
		case InquiryReplyChannel.OTHER:
			return "other";
		case InquiryReplyChannel.SELLER_CENTER:
			return "sellerCenter";
		default:
			return undefined;
	}
}

/** 狀態標籤的顏色：開放中是需要動作的提醒色，已回覆是完成，已撤回是中性。 */
export function inquiryStateColor(state: InquiryState): ChipColor {
	switch (state) {
		case InquiryState.OPEN:
			return "warning";
		case InquiryState.REPLIED:
			return "success";
		default:
			return "default";
	}
}

/** 賣家自己在賣家中心回覆的，與管理員代記的系統外回覆分開說。 */
export function replyIsOffSystem(inquiry: SellerInquiry): boolean {
	return (
		inquiry.reply !== undefined &&
		inquiry.reply.channel !== InquiryReplyChannel.SELLER_CENTER
	);
}

export type InquiryDeadline =
	| { kind: "none" }
	| { kind: "due"; due: Date }
	| { kind: "overdue"; due: Date };

/**
 * 只有開放中的詢問有期限；逾期是 `now` 超過 `due_time`，逾期後仍可回覆（顯示標記，不擋）。
 */
export function inquiryDeadline(
	inquiry: Pick<SellerInquiry, "state" | "replyDeadline">,
	now: Date,
): InquiryDeadline {
	const dueTime: Timestamp | undefined = inquiry.replyDeadline?.dueTime;
	if (inquiry.state !== InquiryState.OPEN || dueTime === undefined) {
		return { kind: "none" };
	}
	const due = timestampDate(dueTime);
	return now.getTime() > due.getTime()
		? { kind: "overdue", due }
		: { kind: "due", due };
}

export type InquiryFilter = "all" | "open" | "replied" | "withdrawn";

export const INQUIRY_FILTERS: readonly InquiryFilter[] = [
	"all",
	"open",
	"replied",
	"withdrawn",
];

export function inquiryFilterState(filter: InquiryFilter): InquiryState {
	switch (filter) {
		case "open":
			return InquiryState.OPEN;
		case "replied":
			return InquiryState.REPLIED;
		case "withdrawn":
			return InquiryState.WITHDRAWN;
		default:
			return InquiryState.UNSPECIFIED;
	}
}

/** 詳情頁的主體：能回覆顯示表單，否則依狀態唯讀。 */
export type InquiryBody = "form" | "replied" | "withdrawn" | "readonly";

export function inquiryBody(inquiry: SellerInquiry): InquiryBody {
	if (inquiry.canReply) return "form";
	if (inquiry.state === InquiryState.REPLIED && inquiry.reply) return "replied";
	if (inquiry.state === InquiryState.WITHDRAWN) return "withdrawn";
	return "readonly";
}
