import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import type {
	CreateReportResponse,
	GetMyReportResponse,
	StoreReport,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import { goTrimmedRuneCount } from "@hawkr/api/text/server-text";
import type { UiMessage } from "@hawkr/ui-text/format";
import {
	commonPolicyCategory,
	sellerReportOutcome,
	sellerStoreReportOutcomeUnset,
} from "@hawkr/ui-text/keys";
import { optionalText, type UiText } from "@/lib/i18n/ui-text";

export const DESCRIPTION_MAX = 2000;
export const ORDER_NO_MAX = 64;
export const EVIDENCE_MAX = 5;
/** 一則檢舉的補充說明上限（report.proto；去重補充與主動補充共用）。 */
export const MAX_SUPPLEMENTS = 10;

/** 檢舉人不能選的類別：UNSPECIFIED 不是值，PLATFORM_ABUSE 只給平台判定（report.proto）。 */
const NOT_SELECTABLE: ReadonlySet<PolicyCategory> = new Set([
	PolicyCategory.UNSPECIFIED,
	PolicyCategory.PLATFORM_ABUSE,
]);

export interface CategoryOption {
	value: PolicyCategory;
	label: string;
}

/** 類別清單：契約的 enum 值（依值排序）除去不可選的；標籤取自共用文案目錄，沒有文字的值不列。 */
export function reportCategoryOptions(ut: UiText): CategoryOption[] {
	const options: CategoryOption[] = [];
	for (const value of Object.values(PolicyCategory)) {
		if (typeof value !== "number" || NOT_SELECTABLE.has(value)) continue;
		const label = optionalText(ut, commonPolicyCategory(value));
		if (label !== undefined) options.push({ value, label });
	}
	return options.sort((a, b) => a.value - b.value);
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
	if (draft.category === undefined || NOT_SELECTABLE.has(draft.category)) {
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

/** 補充入口：還沒有結果（outcome 沒設）而且補充未滿上限。 */
export function canSupplement(
	detail: Pick<GetMyReportResponse, "report" | "supplements">,
): boolean {
	return (
		detail.report !== undefined &&
		detail.report.outcome === undefined &&
		detail.supplements.length < MAX_SUPPLEMENTS
	);
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
