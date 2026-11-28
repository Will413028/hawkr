import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { commonPolicyCategory } from "@hawkr/ui-text/keys";
import { staffTextOrUnknown } from "@/lib/ui-text";

/**
 * 店家治理的平台判定類別與認定可用的類別：全部除了 OTHER 與社群內容的類別（OTHER 只給檢舉人選，理由要指到條款）。伺服器的 `Assessable` 另外收社群內容的
 * SPAM、HARASSMENT、HATE、SEXUAL_CONTENT（內容下架的認定用）；這份下拉是子集，內容下架用 `CONTENT_TAKEDOWN_CATEGORIES`。
 */
export const ASSESSABLE_CATEGORIES: readonly PolicyCategory[] = [
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
	PolicyCategory.PLATFORM_ABUSE,
];

/** 社群內容下架的認定可用的類別，與伺服器的 `policy.Assessable()`（全部除了 OTHER）一致：店家治理的認定類別加上社群內容的類別。 */
export const CONTENT_TAKEDOWN_CATEGORIES: readonly PolicyCategory[] = [
	...ASSESSABLE_CATEGORIES,
	PolicyCategory.SPAM,
	PolicyCategory.HARASSMENT,
	PolicyCategory.HATE,
	PolicyCategory.SEXUAL_CONTENT,
];

/** 社群檢舉案件對買家寫的內容（隱藏、警告、停權）可用的認定類別，與伺服器的 `policy.CommunityAssessable()` 一致。 */
export const COMMUNITY_ASSESSABLE_CATEGORIES: readonly PolicyCategory[] = [
	PolicyCategory.MISLEADING_INFO,
	PolicyCategory.IP_INFRINGEMENT,
	PolicyCategory.FRAUD,
	PolicyCategory.OFF_PLATFORM_TRADE,
	PolicyCategory.PRIVACY,
	PolicyCategory.IMPERSONATION,
	PolicyCategory.PLATFORM_ABUSE,
	PolicyCategory.SPAM,
	PolicyCategory.HARASSMENT,
	PolicyCategory.HATE,
	PolicyCategory.SEXUAL_CONTENT,
];

/** 店家治理的全部 13 個值（不含社群內容的 SPAM、HARASSMENT、HATE、SEXUAL_CONTENT）：生效的類別（在案件裡的是平台判定類別，其他是檢舉人選的類別）可以是任何一個，所有通報清單的類別篩選用它。 */
export const ALL_CATEGORIES: readonly PolicyCategory[] = [
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
	PolicyCategory.PLATFORM_ABUSE,
	PolicyCategory.OTHER,
];

/** 政策分類的顯示文字，取自共用文案目錄；UNSPECIFIED 與不認得的值是「未知」。 */
export function policyCategoryText(category: PolicyCategory): string {
	return staffTextOrUnknown(commonPolicyCategory(category));
}
