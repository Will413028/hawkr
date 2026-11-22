import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import type { CommunityReport } from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import type {
	CreateReportResponse,
	GetMyReportResponse,
	StoreReport,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import { goTrimmedRuneCount } from "@hawkr/api/text/server-text";
import {
	MODERATION_SUBMIT_REPORT_REQUEST_CATEGORY,
	SELLER_CREATE_REPORT_REQUEST_CATEGORY,
} from "@hawkr/ui-text/accepted";
import type { UiMessage } from "@hawkr/ui-text/format";
import {
	commonPolicyCategory,
	moderationCommunityReportOutcomeUnset,
	moderationReportOutcome,
	sellerReportOutcome,
	sellerStoreReportOutcomeUnset,
} from "@hawkr/ui-text/keys";
import { optionalText, type UiText } from "@/lib/i18n/ui-text";

export const DESCRIPTION_MAX = 2000;
export const ORDER_NO_MAX = 64;
export const EVIDENCE_MAX = 5;

/** 檢舉人可選的類別：`CreateReportRequest.category` 的 accepted_policy_categories（生成的常數，順序同 proto）。 */
const SELECTABLE: ReadonlySet<PolicyCategory> = new Set(
	SELLER_CREATE_REPORT_REQUEST_CATEGORY,
);

export interface CategoryOption {
	value: PolicyCategory;
	label: string;
}

/** 類別清單：`CreateReportRequest.category` 接受的值（依 proto 的順序）；標籤取自共用文案目錄，沒有文字的值不列。 */
export function reportCategoryOptions(ut: UiText): CategoryOption[] {
	const options: CategoryOption[] = [];
	for (const value of SELLER_CREATE_REPORT_REQUEST_CATEGORY) {
		const label = optionalText(ut, commonPolicyCategory(value));
		if (label !== undefined) options.push({ value, label });
	}
	return options;
}

export type ReportFormError =
	| "categoryRequired"
	| "descriptionRequired"
	| "descriptionTooLong"
	| "orderNoTooLong";

export interface ReportDraft {
	category: PolicyCategory | undefined;
	description: string;
	orderNo: string;
}

/** 送出前的欄位檢查，規則與伺服器一致（修剪與以字元計長）。 */
export function validateReportDraft(
	draft: ReportDraft,
): Partial<Record<"category" | "description" | "orderNo", ReportFormError>> {
	const errors: ReturnType<typeof validateReportDraft> = {};
	if (draft.category === undefined || !SELECTABLE.has(draft.category)) {
		errors.category = "categoryRequired";
	}
	const length = goTrimmedRuneCount(draft.description);
	if (length === 0) errors.description = "descriptionRequired";
	else if (length > DESCRIPTION_MAX) errors.description = "descriptionTooLong";
	if (goTrimmedRuneCount(draft.orderNo) > ORDER_NO_MAX) {
		errors.orderNo = "orderNoTooLong";
	}
	return errors;
}

/** 成功後給檢舉人看的句子：補充時是伺服器的 notice，否則是 [received]。 */
export function reportSuccessText(
	response: Pick<CreateReportResponse, "supplemented" | "notice">,
	received: string,
): string {
	if (response.supplemented && response.notice?.message) {
		return response.notice.message;
	}
	return received;
}

/** 店家檢舉的結果文字：沒設結果是「審核中」，設了照目錄；目錄沒有文字的值回 undefined。 */
export function storeReportOutcomeMessage(
	report: Pick<StoreReport, "outcome">,
): UiMessage | undefined {
	return report.outcome === undefined
		? sellerStoreReportOutcomeUnset()
		: sellerReportOutcome(report.outcome);
}

/** 社群檢舉的結果文字：沒設結果是「審核中」，設了照目錄；目錄沒有文字的值回 undefined。 */
export function communityReportOutcomeMessage(
	report: Pick<CommunityReport, "outcome">,
): UiMessage | undefined {
	return report.outcome === undefined
		? moderationCommunityReportOutcomeUnset()
		: moderationReportOutcome(report.outcome);
}

/** 佐證在檢舉本身的張數：不屬於任何補充說明的那些。 */
export function ownEvidenceCount(
	detail: Pick<GetMyReportResponse, "supplements" | "evidence">,
): number {
	const inSupplements = new Set(
		detail.supplements.flatMap((s) => s.evidencePositions),
	);
	return detail.evidence.filter((e) => !inSupplements.has(e.position)).length;
}

export type SupplementFormError = "descriptionRequired" | "descriptionTooLong";

/** 補充說明送出前的檢查，規則同檢舉說明（修剪與以字元計長）。 */
export function validateSupplementDescription(
	description: string,
): SupplementFormError | undefined {
	const length = goTrimmedRuneCount(description);
	if (length === 0) return "descriptionRequired";
	if (length > DESCRIPTION_MAX) return "descriptionTooLong";
	return undefined;
}

/** 評價與聊天訊息檢舉的說明上限（`SubmitReportRequest.description`，修剪後以字元計）。 */
export const COMMUNITY_DESCRIPTION_MAX = 500;

/** 評價與聊天訊息檢舉可選的類別：`SubmitReportRequest.category` 的 accepted_policy_categories；標籤取自共用文案目錄。 */
export function communityReportCategoryOptions(ut: UiText): CategoryOption[] {
	const options: CategoryOption[] = [];
	for (const value of MODERATION_SUBMIT_REPORT_REQUEST_CATEGORY) {
		const label = optionalText(ut, commonPolicyCategory(value));
		if (label !== undefined) options.push({ value, label });
	}
	return options;
}

/** 選了「其他」才要說明；說明有填就不能超過上限（規則同伺服器：修剪與以字元計長）。 */
export function validateCommunityReport(draft: {
	category: PolicyCategory | undefined;
	description: string;
}): Partial<
	Record<
		"category" | "description",
		"categoryRequired" | "descriptionRequired" | "descriptionTooLong"
	>
> {
	const errors: ReturnType<typeof validateCommunityReport> = {};
	if (
		draft.category === undefined ||
		!MODERATION_SUBMIT_REPORT_REQUEST_CATEGORY.includes(draft.category)
	) {
		errors.category = "categoryRequired";
	}
	const length = goTrimmedRuneCount(draft.description);
	if (length > COMMUNITY_DESCRIPTION_MAX) {
		errors.description = "descriptionTooLong";
	} else if (draft.category === PolicyCategory.OTHER && length === 0) {
		errors.description = "descriptionRequired";
	}
	return errors;
}
