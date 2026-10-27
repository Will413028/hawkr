import type { RoleSlot } from "@/lib/auth/active-role";

export type NotificationEventWorkspace = "current" | "other" | "unknown";

/**
 * Account-wide notification topics can contain both buyer and seller items.
 * Missing role metadata is never guessed: callers recover the current inbox
 * from REST instead of applying an ambiguous pushed count.
 */
export function notificationEventWorkspace(
	eventRole: RoleSlot | undefined,
	activeRole: RoleSlot | null | undefined,
	eventAccountId?: string,
	activeAccountId?: string,
): NotificationEventWorkspace {
	if (
		eventAccountId != null &&
		activeAccountId != null &&
		eventAccountId !== activeAccountId
	) {
		return "other";
	}
	if (!activeRole) return "current";
	if (!eventRole) return "unknown";
	return eventRole === activeRole ? "current" : "other";
}
