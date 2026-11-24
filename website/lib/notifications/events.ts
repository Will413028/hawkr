import type { NotificationItem } from "./types";

/**
 * Window-level CustomEvent name used by `useUnreadCount` to forward the
 * SSE-pushed `notification.new` item to non-context consumers (notably
 * `NotificationToastBridge`).
 *
 * Why not surface this through the unread context: the bridge mounts as
 * a sibling of the provider — its useEffect handlers don't need the count
 * itself, only the inbound item — and adding subscriber plumbing to the
 * context just to deliver one event would couple the badge consumer
 * surface (`NotificationBell`, list page mutate paths) to something they
 * don't care about. Window events keep the bridge optional and lazy.
 *
 * Bridge logic: on receive, toast the item directly. The legacy
 * count-rise path that re-fetches `/app/notifications?pageSize=10`
 * remains as a fallback for count changes that aren't a fresh push
 * (cross-tab unread state churn, race conditions, etc.) and naturally
 * dedupes against the SSE-driven toast via the bridge's `seenIdsRef`.
 */
export const NOTIFICATION_NEW_EVENT = "hk:notification:new";

export interface NotificationNewEventDetail {
	item: NotificationItem;
	unreadCount: number;
}
