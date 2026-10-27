import type { QueryClient } from "@tanstack/react-query";
import type { QueryIdentity } from "@/lib/query/keys";
import { queryKeys } from "@/lib/query/keys";

export type NotificationListRefetchType = "active" | "all" | "none";

/**
 * Mark every notification-list variant for one account/workspace as stale.
 *
 * Notification list keys vary by role, category and page size. Keeping the
 * identity prefix here prevents a realtime seller event from invalidating
 * buyer lists, another account's cache, or notification settings.
 */
export function invalidateNotificationLists(
	queryClient: QueryClient,
	identity: QueryIdentity,
	refetchType: NotificationListRefetchType,
) {
	return queryClient.invalidateQueries({
		queryKey: queryKeys.notifications.lists(identity),
		refetchType,
	});
}
