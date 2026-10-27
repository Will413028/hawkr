/**
 * KYC 的流程狀態（`GetMyKYC` 的 `application.flow`）：KYC 與 PayUni 兩條軸合成的結果，給畫面顯示。
 * 還沒有申請（或申請已取消）時 GetMyKYC 回 not_found，那就是 `NOT_STARTED`，不是錯誤。
 */

import { create } from "@bufbuild/protobuf";
import {
	type KYCApplication,
	type KYCFlow,
	KYCFlowSchema,
	KYCFlowStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import type { UiMessage } from "@hawkr/ui-text/format";
import {
	commonUnknown,
	sellerKYCFlowStatus,
	sellerKYCFlowStatusAction,
	sellerKYCFlowStatusBannerDescription,
	sellerKYCFlowStatusBannerTitle,
	sellerKYCFlowStatusGateHeading,
	sellerKYCFlowStatusGateMessage,
} from "@hawkr/ui-text/keys";
import { optionalText, type UiText } from "../i18n/ui-text";
import { payuniHasState, payuniStateTitle } from "./payuni";

export function notStartedFlow(): KYCFlow {
	return create(KYCFlowSchema, {
		status: KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
	});
}

/** 申請的流程狀態；沒有申請就是 NOT_STARTED。 */
export function flowOf(app: KYCApplication | null | undefined): KYCFlow {
	return app?.flow ?? notStartedFlow();
}

/** 申請已核准、進入 PayUni 階段（綁定、審核、補件、核准、被拒）：錢包顯示 PayUni 的狀態。 */
export function kycInPayuniStage(status: KYCFlowStatus | undefined): boolean {
	switch (status) {
		case KYCFlowStatus.KYC_FLOW_STATUS_BINDING:
		case KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED:
		case KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED:
		case KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW:
		case KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO:
		case KYCFlowStatus.KYC_FLOW_STATUS_APPROVED:
		case KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED:
			return true;
		default:
			return false;
	}
}

export type StatusBannerTone =
	| "neutral"
	| "info"
	| "success"
	| "warning"
	| "danger";

export interface CertStatusBanner {
	tone: StatusBannerTone;
	title: string;
	description: string;
	/** 沒有行動時不顯示按鈕。 */
	ctaLabel?: string;
}

/** 認證閘門卡片（錢包、物流設定在認證前顯示）的文字。 */
export interface CertGateCopy {
	heading: string;
	message: (subject: string) => string;
}

/**
 * 流程狀態在畫面上的階段：名詞、行動、橫幅與閘門的文字都取自共用文案目錄（`KYCFlowStatus` 的名詞與變體）；
 * 橫幅的語氣與按鈕文字是版面，留在 client，以階段為單位。
 */
type FlowStage =
	| "before_submit"
	| "platform_review"
	| "payuni_review"
	| "needs_info"
	| "not_passed"
	| "approved";

function stageOf(status: KYCFlowStatus | undefined): FlowStage | undefined {
	switch (status) {
		case KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED:
		case KYCFlowStatus.KYC_FLOW_STATUS_DRAFT:
			return "before_submit";
		case KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED:
			return "platform_review";
		case KYCFlowStatus.KYC_FLOW_STATUS_BINDING:
		case KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED:
		case KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED:
		case KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW:
			return "payuni_review";
		case KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO:
			return "needs_info";
		case KYCFlowStatus.KYC_FLOW_STATUS_REJECTED:
		case KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED:
			return "not_passed";
		case KYCFlowStatus.KYC_FLOW_STATUS_APPROVED:
			return "approved";
		default:
			return undefined;
	}
}

const STAGE_BANNER: Record<
	FlowStage,
	{ tone: StatusBannerTone; ctaLabel: string }
> = {
	before_submit: { tone: "neutral", ctaLabel: "開始認證" },
	platform_review: { tone: "info", ctaLabel: "查看進度" },
	payuni_review: { tone: "info", ctaLabel: "查看進度" },
	needs_info: { tone: "warning", ctaLabel: "前往補件" },
	not_passed: { tone: "danger", ctaLabel: "查看原因" },
	approved: { tone: "success", ctaLabel: "前往錢包" },
};

/** 沒有狀態或不認得的狀態與「尚未開始」相同。 */
const FALLBACK_STATUS = KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED;

/** 目錄認得的狀態；沒有狀態或不認得的狀態換成「尚未開始」。 */
function knownOrFallback(status: KYCFlowStatus | undefined): KYCFlowStatus {
	return status !== undefined && stageOf(status) !== undefined
		? status
		: FALLBACK_STATUS;
}

function textOrUnknown(ut: UiText, m: UiMessage | undefined): string {
	return ut(m ?? commonUnknown());
}

/** 賣家看到的流程狀態：「已認證」只看店家的公開條件，已認證時一律是已通過。 */
export function shownFlowStatus(
	certified: boolean,
	status: KYCFlowStatus | undefined,
): KYCFlowStatus | undefined {
	return certified ? KYCFlowStatus.KYC_FLOW_STATUS_APPROVED : status;
}

/** 申請在審核中（平台或 PayUni）：已送出、還沒有結果。 */
export function kycFlowInReview(status: KYCFlowStatus | undefined): boolean {
	switch (status) {
		case KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED:
		case KYCFlowStatus.KYC_FLOW_STATUS_BINDING:
		case KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED:
		case KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED:
		case KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW:
			return true;
		default:
			return false;
	}
}

/** 流程狀態的顯示文字（開店進度卡與認證入口共用）；UNSPECIFIED 與不認得的狀態是共用文案目錄的「未知」。 */
export function kycFlowStatusLabel(
	ut: UiText,
	status: KYCFlowStatus | undefined,
): string {
	return (
		(status !== undefined && optionalText(ut, sellerKYCFlowStatus(status))) ||
		ut(commonUnknown())
	);
}

/** 流程狀態的行動文字：卡片與按鈕引導賣家做哪一步；已通過沒有下一步。沒有狀態或不認得的狀態與「尚未開始」相同。 */
export function kycFlowActionLabel(
	ut: UiText,
	status: KYCFlowStatus | undefined,
): string | undefined {
	return optionalText(ut, sellerKYCFlowStatusAction(knownOrFallback(status)));
}

/**
 * 認證頁與錢包入口的流程橫幅；沒有狀態時與「尚未開始」相同。PayUni 階段沒有橫幅文字（目錄 omit，狀態由 `KYCFlow.payuni`
 * 呈現），回 null。
 */
export function kycFlowBanner(
	ut: UiText,
	status: KYCFlowStatus | undefined,
): CertStatusBanner | null {
	const shown = knownOrFallback(status);
	const title = sellerKYCFlowStatusBannerTitle(shown);
	const description = sellerKYCFlowStatusBannerDescription(shown);
	if (title === undefined || description === undefined) return null;
	return {
		...STAGE_BANNER[stageOf(shown) ?? "before_submit"],
		title: ut(title),
		description: ut(description),
	};
}

/**
 * 認證前的閘門卡片文字與按鈕；沒有流程時與「尚未開始」相同。有 PayUni 狀態時標題是 PayUni 的標題、內容是伺服器的說明
 * （不帶 `{subject}`）；其餘是目錄的文字，目錄沒有文字的狀態（PayUni 階段卻沒有 `payuni`：契約被違反）是共用文案目錄的「未知」，
 * 與 seller-app 一致，閘門的位置不會是空白。
 */
export function kycFlowGate(
	ut: UiText,
	flow: KYCFlow | null | undefined,
): {
	copy: CertGateCopy;
	action: string;
} {
	const shown = knownOrFallback(flow?.status);
	const action =
		optionalText(ut, sellerKYCFlowStatusAction(shown)) ??
		textOrUnknown(ut, sellerKYCFlowStatusAction(FALLBACK_STATUS));
	const payuni = flow?.payuni;
	if (payuniHasState(payuni)) {
		const explanation = payuni.explanation?.message ?? "";
		return {
			copy: {
				heading: payuniStateTitle(ut, payuni.state),
				message: () => explanation,
			},
			action,
		};
	}
	const heading = sellerKYCFlowStatusGateHeading(shown);
	if (heading === undefined) {
		return {
			copy: {
				heading: ut(commonUnknown()),
				message: () => ut(commonUnknown()),
			},
			action,
		};
	}
	return {
		copy: {
			heading: ut(heading),
			message: (subject) =>
				textOrUnknown(ut, sellerKYCFlowStatusGateMessage(shown, { subject })),
		},
		action,
	};
}
