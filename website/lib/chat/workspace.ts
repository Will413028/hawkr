import type { RoleSlot } from "@/lib/auth/active-role";

export function chatWorkspaceRole(pathname: string): RoleSlot {
	return pathname.startsWith("/seller-center") ? "seller" : "buyer";
}
