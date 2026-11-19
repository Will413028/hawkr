import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { commonPolicyCategory } from "@hawkr/ui-text/keys";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { optionalText, type UiText } from "@/lib/i18n/ui-text";
import type { IdentitySuspension } from "./storage";

/** 停權的政策分類文字；UNSPECIFIED（讀不到停權紀錄）與目錄沒有的值回 undefined，不顯示。 */
export function suspensionCategoryText(
	ut: UiText,
	suspension: IdentitySuspension,
): string | undefined {
	if (suspension.category === PolicyCategory.UNSPECIFIED) return undefined;
	return optionalText(ut, commonPolicyCategory(suspension.category));
}

/** 給帳號擁有者看的理由；空白字串視為沒有。 */
export function suspensionReasonText(
	suspension: IdentitySuspension,
): string | undefined {
	const reason = suspension.reason.trim();
	return reason === "" ? undefined : reason;
}

/** 停權日期（台北時區 YYYY/MM/DD）；伺服器沒帶或解不出來時回 undefined。 */
export function suspensionDateText(
	suspension: IdentitySuspension,
): string | undefined {
	if (!suspension.suspendedAt) return undefined;
	return formatTaipeiDate(suspension.suspendedAt) || undefined;
}
