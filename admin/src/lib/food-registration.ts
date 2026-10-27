import type { MessageInitShape } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
	type FoodRegistration,
	FoodRegistrationEndCause,
	FoodRegistrationInbox,
	FoodRegistrationState,
	KYCSellerType,
	type ListFoodRegistrationsRequestSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import type { BadgeVariant } from "@/components/UI";
import { formatDateTimeShort } from "@/lib/format";
import { unknownText } from "@/lib/ui-text";
import { firstVisibleLabel } from "@/lib/unavailability";

// 食品業者登錄審核（SellerAdminService.ListFoodRegistrations、GetFoodRegistration、DecideFoodRegistration、
// RevokeFoodRegistration）的顯示判斷與請求組法。能不能核准、退回、撤銷只讀伺服器的 can_approve、can_reject、can_revoke，這裡不重新推導。

export const FADENBOOK_URL = "https://fadenbook.fda.gov.tw/";
export const FADENBOOK_LINK_TEXT = "在食藥署查詢";
export const REJECT_REASON_EMPTY_TEXT =
	"請填寫退回理由，賣家會看到它，據此確認字號與食藥署登錄資料是否一致。";
export const REVOKE_REASON_EMPTY_TEXT =
	"請填寫給賣家的理由，賣家會看到它，據此確認字號與食藥署登錄資料是否一致。";
export const REVOKE_BUTTON_TEXT = "撤銷登錄";
export const REVOKE_DIALOG_TEXT =
	"撤銷後，店面與商品頁立即不再顯示這個字號，賣家需要重新送審。若屬冒用或不實登錄，請另外記錄違規認定。這家店若有同一字號的待審件，請一併退回。";
export const REVOKE_REASON_LABEL = "給賣家的理由（必填）";

/** 歷史的分頁（待審是收件匣，分頁見 LIST_TABS）。 */
export const STATE_TABS = [
	FoodRegistrationState.PENDING,
	FoodRegistrationState.APPROVED,
	FoodRegistrationState.REJECTED,
	FoodRegistrationState.WITHDRAWN,
	FoodRegistrationState.ENDED,
] as const;

export const STATE_LABEL: Record<
	Exclude<FoodRegistrationState, FoodRegistrationState.UNSPECIFIED>,
	string
> = {
	[FoodRegistrationState.PENDING]: "待審核",
	[FoodRegistrationState.APPROVED]: "已核准",
	[FoodRegistrationState.REJECTED]: "已退回",
	[FoodRegistrationState.WITHDRAWN]: "已撤回",
	[FoodRegistrationState.ENDED]: "已結束",
};

export interface ListTab {
	key: string;
	label: string;
	state: FoodRegistrationState;
	/** 只有收件匣的兩個分頁帶隊列；歷史不可以指定，伺服器回 invalid_argument。 */
	inbox: FoodRegistrationInbox;
	emptyText: string;
}

/** 清單的分頁：待審分成現在可以裁定的（DECIDABLE）與還在等商家認證的（AWAITING_KYC），其餘是歷史。 */
export const LIST_TABS: readonly ListTab[] = [
	{
		key: "decidable",
		label: STATE_LABEL[FoodRegistrationState.PENDING],
		state: FoodRegistrationState.PENDING,
		inbox: FoodRegistrationInbox.DECIDABLE,
		emptyText: "沒有待審核的食品業者登錄",
	},
	{
		key: "awaiting-kyc",
		label: "等商家認證",
		state: FoodRegistrationState.PENDING,
		inbox: FoodRegistrationInbox.AWAITING_KYC,
		emptyText: "沒有等待商家認證的食品業者登錄",
	},
	...STATE_TABS.slice(1).map((state) => ({
		key: `state-${state}`,
		label: STATE_LABEL[state],
		state,
		inbox: FoodRegistrationInbox.UNSPECIFIED,
		emptyText: "沒有符合的食品業者登錄",
	})),
];

const STATE_VARIANT: Record<
	Exclude<FoodRegistrationState, FoodRegistrationState.UNSPECIFIED>,
	BadgeVariant
> = {
	[FoodRegistrationState.PENDING]: "warning",
	[FoodRegistrationState.APPROVED]: "success",
	[FoodRegistrationState.REJECTED]: "danger",
	[FoodRegistrationState.WITHDRAWN]: "gray",
	[FoodRegistrationState.ENDED]: "gray",
};

/** 狀態的顯示文字；UNSPECIFIED 與不認得的值是「未知」（共用文案目錄的 common.unknown）。 */
export function stateLabel(state: FoodRegistrationState): string {
	return (
		(STATE_LABEL as Partial<Record<FoodRegistrationState, string>>)[state] ??
		unknownText()
	);
}

export function stateBadge(state: FoodRegistrationState): {
	label: string;
	variant: BadgeVariant;
} {
	return {
		label: stateLabel(state),
		variant:
			(STATE_VARIANT as Partial<Record<FoodRegistrationState, BadgeVariant>>)[
				state
			] ?? "gray",
	};
}

const END_CAUSE_LABEL: Record<
	Exclude<FoodRegistrationEndCause, FoodRegistrationEndCause.UNSPECIFIED>,
	string
> = {
	[FoodRegistrationEndCause.SUPERSEDED]: "被新核准的字號取代",
	[FoodRegistrationEndCause.CLEARED]: "賣家清除",
	[FoodRegistrationEndCause.REVOKED]: "平台撤銷",
	[FoodRegistrationEndCause.STORE_RETIRED]: "商店已關閉",
};

/** 登錄結束原因的顯示文字；UNSPECIFIED 與不認得的值是「未知」。 */
export function endCauseLabel(cause: FoodRegistrationEndCause): string {
	return (
		(END_CAUSE_LABEL as Partial<Record<FoodRegistrationEndCause, string>>)[
			cause
		] ?? unknownText()
	);
}

type ListInit = MessageInitShape<typeof ListFoodRegistrationsRequestSchema>;

export function listRequest(input: {
	state: FoodRegistrationState;
	inbox: FoodRegistrationInbox;
	keyword: string;
	pageToken: string;
	pageSize: number;
}): ListInit {
	return {
		state: input.state,
		inbox: input.inbox,
		query: input.keyword.trim(),
		pageToken: input.pageToken,
		pageSize: input.pageSize,
	};
}

/** 核對時要比對的 KYC 主體：企業是公司名稱與統編，個人是真實姓名；沒有申請時沒有主體。 */
export function subjectRows(
	registration: FoodRegistration,
): { label: string; value: string }[] {
	const subject = registration.subject;
	if (!subject) return [];
	if (subject.sellerType === KYCSellerType.KYC_SELLER_TYPE_COMPANY) {
		return [
			{ label: "公司名稱", value: subject.companyName || "-" },
			{ label: "統一編號", value: subject.taxId || "-" },
			{ label: "負責人", value: subject.realName || "-" },
		];
	}
	return [{ label: "真實姓名", value: subject.realName || "-" }];
}

/**
 * 不能裁定時詳情頁要顯示的那一句，只讀伺服器：核准的第一個原因（核准能做時看退回的）；第一個原因是結構性的（登錄不在待審）時沒有入口、不說明；
 * 可以裁定時沒有句子。伺服器列出全部原因，畫面只顯示第一個。
 */
export function decideBlockedLabel(
	registration: FoodRegistration,
): string | undefined {
	return firstVisibleLabel(
		registration.approveUnavailabilities.length > 0
			? registration.approveUnavailabilities
			: registration.rejectUnavailabilities,
	);
}

export function submitTimeText(registration: FoodRegistration): string {
	return registration.submitTime
		? formatDateTimeShort(timestampDate(registration.submitTime))
		: "-";
}

export function decideTimeText(registration: FoodRegistration): string {
	return registration.decideTime
		? formatDateTimeShort(timestampDate(registration.decideTime))
		: "-";
}

export function endTimeText(registration: FoodRegistration): string {
	return registration.endTime
		? formatDateTimeShort(timestampDate(registration.endTime))
		: "-";
}

/** 店家頁的食品業者登錄摘要：核准的字號與待審（或被退回）的登錄；兩者都沒有時沒有列。 */
export function storeFoodRows(store: {
	approvedFoodRegistration?: { registrationNumber: string };
	openFoodRegistration?: {
		id: string;
		state: FoodRegistrationState;
		registrationNumber: string;
	};
}): { label: string; value: string; registrationId?: string }[] {
	const rows: { label: string; value: string; registrationId?: string }[] = [];
	if (store.approvedFoodRegistration) {
		rows.push({
			label: "食品業者登錄字號",
			value: store.approvedFoodRegistration.registrationNumber,
		});
	}
	const open = store.openFoodRegistration;
	if (open) {
		rows.push({
			label: `${stateLabel(open.state)}的登錄`,
			value: open.registrationNumber,
			registrationId: open.id,
		});
	}
	return rows;
}
