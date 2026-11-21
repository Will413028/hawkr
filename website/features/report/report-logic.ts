import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import type { CreateReportResponse } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import { goTrimmedRuneCount } from "@hawkr/api/text/server-text";
import { commonPolicyCategory } from "@hawkr/ui-text/keys";
import { optionalText, type UiText } from "@/lib/i18n/ui-text";

export const DESCRIPTION_MAX = 2000;
export const ORDER_NO_MAX = 64;
export const EVIDENCE_MAX = 5;

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
