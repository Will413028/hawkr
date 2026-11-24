/**
 * Window-level CustomEvent：通知串流的收件匣訊號（`WatchNotifications` 的 `inbox`）轉發給不在
 * unread context 裡的訂閱者（`NotificationToastBridge`、發票 hook）。訊號只說「收件匣有變化」
 * 與狀態重讀提示，不帶通知內容。
 */
export const NOTIFICATION_SIGNAL_EVENT = "hk:notification:signal";

export interface NotificationSignalEventDetail {
	/** 異常斷線後重連的廣播不帶未讀數（沒有伺服器給的值）。 */
	unreadCount?: number;
	latestInboxItemId?: string;
	/** `StateRefreshHint` 的值；空表示不需要重讀。 */
	hints: number[];
}

/**
 * Window-level CustomEvent：賣家工作區的資料變化（`WatchNotifications` 的 `sellerWorkspace`）與異常斷線後的重連，
 * 由 unread provider 的單一串流轉發給 `useSellerWorkspaceRealtime`；賣家頁面不另開串流。
 */
export const SELLER_WORKSPACE_SIGNAL_EVENT = "hk:notification:seller-workspace";

export interface SellerWorkspaceSignalEventDetail {
	kind: "change" | "reconnected";
}
