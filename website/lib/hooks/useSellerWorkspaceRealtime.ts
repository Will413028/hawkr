"use client";

/**
 * 賣家中心外框的即時同步訂閱者。
 *
 * 後端透過 PayUni / 物流 webhook、售後流程、系統排程改變 order / after-sale 狀態時，工作區的快取
 * （統計卡、訂單列、售後列、儀表板、物流異常）不需手動重整即失效重抓，drawer 不關、URL 不變。
 *
 * 這個 hook 不開連線：整個分頁唯一一條 `/app/notifications/stream` 由 `NotificationUnreadProvider`
 * 持有，賣家事件與「可能漏事件」的訊號以 window 事件分派（`SELLER_WORKSPACE_EVENT`、
 * `NOTIFICATION_STREAM_RESYNC_EVENT`），外框掛一個訂閱者即可。
 *
 *   - 賣家事件 → 250ms debounce 合併（避免連續多筆通知造成統計卡抖動 / API storm）→
 *     `invalidateQueries` 整個 seller orders / dashboard / after-sales / 物流異常 family。
 *     **不直接 patch local list**，由 TanStack 以 keepPreviousData 平滑刷新。
 *   - resync（`ready.bufferMiss`、重連預算用完）→ 立即做一次完整 invalidation，補上斷線期間遺漏。
 *   - 串流不可用的狀態由 `useNotificationStreamStatus` 暴露，不做 polling。
 */

import { useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import {
	NOTIFICATION_STREAM_RESYNC_EVENT,
	SELLER_WORKSPACE_EVENT,
} from "@/lib/notifications/events";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

/** 250ms 合併連續事件，避免統計卡抖動與 API storm。 */
export const SELLER_WORKSPACE_DEBOUNCE_MS = 250;

export function useSellerWorkspaceRealtime(): void {
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	const timerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);

	const invalidateWorkspace = React.useCallback(() => {
		// queryKeys.seller.X(identity) 末位為 params slot；slice 掉得到 family prefix，
		// TanStack 前綴比對同時涵蓋 list 與 detail/drawer（如 [...,"orders"] 同時命中
		// 訂單列與單筆 order detail [...,"orders",orderNo,{}]）。
		const family = (key: readonly unknown[]) => key.slice(0, -1);
		queryClient.invalidateQueries({
			queryKey: family(queryKeys.seller.orders(identity)),
		});
		queryClient.invalidateQueries({
			queryKey: queryKeys.seller.dashboard(identity),
		});
		queryClient.invalidateQueries({
			queryKey: family(queryKeys.seller.orderCounts(identity)),
		});
		queryClient.invalidateQueries({
			queryKey: family(queryKeys.seller.afterSales(identity)),
		});
		queryClient.invalidateQueries({
			queryKey: family(queryKeys.seller.shipmentAnomalies(identity)),
		});
		queryClient.invalidateQueries({
			queryKey: queryKeys.seller.shipmentAnomalyStats(identity),
		});
	}, [queryClient, identity]);

	React.useEffect(() => {
		const clearTimer = () => {
			if (timerRef.current) clearTimeout(timerRef.current);
			timerRef.current = null;
		};
		const onWorkspaceEvent = () => {
			clearTimer();
			timerRef.current = setTimeout(() => {
				timerRef.current = null;
				invalidateWorkspace();
			}, SELLER_WORKSPACE_DEBOUNCE_MS);
		};
		const onResync = () => {
			clearTimer();
			invalidateWorkspace();
		};
		window.addEventListener(SELLER_WORKSPACE_EVENT, onWorkspaceEvent);
		window.addEventListener(NOTIFICATION_STREAM_RESYNC_EVENT, onResync);
		return () => {
			clearTimer();
			window.removeEventListener(SELLER_WORKSPACE_EVENT, onWorkspaceEvent);
			window.removeEventListener(NOTIFICATION_STREAM_RESYNC_EVENT, onResync);
		};
	}, [invalidateWorkspace]);
}
