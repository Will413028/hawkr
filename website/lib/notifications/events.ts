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

/**
 * Window-level event for seller-workspace changes (`seller.order.updated`,
 * `seller.after_sale.updated`). They ride the same `/app/notifications/stream`
 * connection that `NotificationUnreadProvider` owns; the provider re-publishes
 * them here so the workspace shell can subscribe without opening a second
 * connection (same reason as `NOTIFICATION_NEW_EVENT`).
 */
export const SELLER_WORKSPACE_EVENT = "hk:seller:workspace";

export interface SellerWorkspaceEventDetail {
	kind: "order" | "after_sale";
	orderNo: string;
	afterSaleCaseNo?: string;
	status?: string;
}

/**
 * Window-level event published once when the stream may have missed events:
 * the server reported a buffer miss on reconnect, or the reconnect budget ran
 * out (`unavailable`). Subscribers re-read everything they cache instead of
 * trusting incremental events.
 */
export const NOTIFICATION_STREAM_RESYNC_EVENT = "hk:notification:resync";
