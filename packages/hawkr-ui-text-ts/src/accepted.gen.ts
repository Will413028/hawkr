// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";

/** moderation.SubmitReportRequest.category 接受的類別（hawkr.common.v1alpha1.accepted_policy_categories），順序同 proto。 */
export const MODERATION_SUBMIT_REPORT_REQUEST_CATEGORY: readonly PolicyCategory[] = [
	PolicyCategory.SPAM,
	PolicyCategory.HARASSMENT,
	PolicyCategory.HATE,
	PolicyCategory.SEXUAL_CONTENT,
	PolicyCategory.MISLEADING_INFO,
	PolicyCategory.FOOD_SAFETY,
	PolicyCategory.FRAUD,
	PolicyCategory.OTHER,
];

/** seller.CreateReportRequest.category 接受的類別（hawkr.common.v1alpha1.accepted_policy_categories），順序同 proto。 */
export const SELLER_CREATE_REPORT_REQUEST_CATEGORY: readonly PolicyCategory[] = [
	PolicyCategory.FOOD_SAFETY,
	PolicyCategory.PROHIBITED_GOODS,
	PolicyCategory.UNLICENSED_SALE,
	PolicyCategory.MISLEADING_INFO,
	PolicyCategory.FAKE_CERTIFICATION,
	PolicyCategory.COUNTERFEIT,
	PolicyCategory.IP_INFRINGEMENT,
	PolicyCategory.FRAUD,
	PolicyCategory.OFF_PLATFORM_TRADE,
	PolicyCategory.PRIVACY,
	PolicyCategory.IMPERSONATION,
	PolicyCategory.OTHER,
];
