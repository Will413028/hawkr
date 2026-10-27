import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { commonPolicyCategory } from "@hawkr/ui-text/keys";
import { staffTextOrUnknown } from "@/lib/ui-text";

// 政策分類（common.PolicyCategory，step10e-reports-brief R10）在 admin 的子集與顯示文字。子集由伺服器驗證（backend 的
// internal/platform/policy），這裡的常數只決定下拉選項的內容與順序；第二個 client 要用子集（10f 的檢舉表單）時，改成 proto 的
// custom option 由 uitext-gen 生成三端常數（設計 step10e-3ca-design C3），這份常數就隨之刪除。順序同契約的數字（UNSPECIFIED 不是有效值）。

/** 店家治理的檢舉人可選的類別（ReporterSelectable）：全部除了 PLATFORM_ABUSE 與社群內容的類別。待分案的通報（還沒有平台判定類別，生效的類別就是檢舉人選的）的類別篩選用它。 */
export const REPORTER_CATEGORIES: readonly PolicyCategory[] = [
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

/** 店家治理的平台判定類別與認定可用的類別（Assessable）：全部除了 OTHER 與社群內容的類別（OTHER 只給檢舉人選，理由要指到條款）。 */
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

/** 店家治理的全部 13 個值（不含社群內容的 SPAM、HARASSMENT、HATE）：生效的類別（在案件裡的是平台判定類別，其他是檢舉人選的類別）可以是任何一個，所有通報清單的類別篩選用它。 */
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
