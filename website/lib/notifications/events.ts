/**
 * Window-level CustomEvent：通知串流的收件匣訊號（`WatchNotifications` 的 `inbox`）轉發給不在
 * unread context 裡的訂閱者（`NotificationToastBridge`、發票 hook）。訊號只說「收件匣有變化」
 * 與狀態重讀提示，不帶通知內容。
 */
export const NOTIFICATION_SIGNAL_EVENT = "hk:notification:signal";

export interface NotificationSignalEventDetail {
	unreadCount: number;
	latestInboxItemId?: string;
	/** `StateRefreshHint` 的值；空表示不需要重讀。 */
	hints: number[];
}
