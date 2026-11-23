import type { RoleSlot } from "@/lib/auth/active-role";
import type { ChatSide } from "./types";

export function chatWorkspaceRole(pathname: string): RoleSlot {
	return pathname.startsWith("/seller-center") ? "seller" : "buyer";
}

/** 工作區（角色）對應的對話方：賣家中心是店家方。 */
export function chatSideOf(role: RoleSlot | null | undefined): ChatSide {
	return role === "seller" ? "store" : "buyer";
}
