import type { RoleSlot } from "./active-role";

export { getStoredActiveRole as getActiveRole } from "./active-role";
export type { RoleSlot };

export interface Identity {
	accountId: string;
	/** 帳號有 ROLE_SELLER（契約的 `Account.roles`）。每個帳號都是買家，不另外表示。 */
	isSeller: boolean;
	nickname?: string;
	avatarKey?: string;
}
