import type { RoleSlot } from "@/lib/auth/active-role";
import { stripLocale } from "@/lib/intl/paths";

export function notificationWorkspaceRole(pathname: string): RoleSlot {
	return stripLocale(pathname).startsWith("/seller-center")
		? "seller"
		: "buyer";
}
