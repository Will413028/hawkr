import type { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import type { RoleSlot } from "./active-role";

export { getStoredActiveRole as getActiveRole } from "./active-role";
export type { RoleSlot };

/** 帳號的停權狀態（契約的 `GetSessionResponse.suspension`）；沒停權時 `Identity.suspension` 不設。 */
export interface IdentitySuspension {
	/** 停權時間，ISO 8601；伺服器沒帶時不設。 */
	suspendedAt?: string;
	/** 政策分類；UNSPECIFIED 表示讀不到停權紀錄，顯示端不當成有效值。 */
	category: PolicyCategory;
	/** 給帳號擁有者看的理由；空字串表示沒有補充說明。 */
	reason: string;
}

export interface Identity {
	accountId: string;
	/** 帳號有 ROLE_SELLER（契約的 `Account.roles`）。每個帳號都是買家，不另外表示。 */
	isSeller: boolean;
	nickname?: string;
	avatarKey?: string;
	suspension?: IdentitySuspension;
}
