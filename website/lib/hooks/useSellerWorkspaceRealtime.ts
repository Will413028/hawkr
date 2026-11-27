"use client";

/**
 * 賣家中心外框的即時同步訂閱者。
 *
 * 後端透過 PayUni / 物流 webhook、售後流程、系統排程改變 order / after-sale 狀態時，工作區的快取
 * （統計卡、訂單列、售後列、儀表板、物流異常）不需手動重整即失效重抓，drawer 不關、URL 不變。
 *
 * 這個 hook 不開串流（每個分頁只有 unread provider 的一條 `WatchNotifications`，D19），聽 provider 轉發的
 * `SELLER_WORKSPACE_SIGNAL_EVENT`：
 *
 *   - 賣家工作區訊號 → 250ms debounce 合併（避免連續多筆通知造成統計卡抖動 / API storm）→
 *     `invalidateQueries` 整個 seller orders / dashboard / after-sales / 物流異常 family。
 *     **不直接 patch local list**，由 TanStack 以 keepPreviousData 平滑刷新。
 *   - 重連後的第一個 frame（訊號可能在斷線期間遺漏）→ 立即做一次完整 invalidation。
 *   - 串流的連線狀態由回傳值暴露（`SellerWorkspaceRealtimeStatusContext` 給訂單頁的狀態列），不做 polling。
 */

import { useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import {
	SELLER_WORKSPACE_SIGNAL_EVENT,
	type SellerWorkspaceSignalEventDetail,
} from "@/lib/notifications/events";
import { useNotificationStreamStatus } from "@/lib/notifications/unread-context";
import { queryKeys } from "@/lib/query/keys";
import type { SseConnectionStatus } from "@/lib/sse/client";
import { useQueryIdentity } from "./useQueryIdentity";

/** 250ms 合併連續事件，避免統計卡抖動與 API storm。 */
export const SELLER_WORKSPACE_DEBOUNCE_MS = 250;

/** 賣家工作區串流的連線狀態；訂單頁讀它顯示「即時同步已暫停」。provider 之外是 idle。 */
export const SellerWorkspaceRealtimeStatusContext =
	React.createContext<SseConnectionStatus>("idle");

export function useSellerWorkspaceRealtimeStatus(): SseConnectionStatus {
	return React.useContext(SellerWorkspaceRealtimeStatusContext);
}

/** `enabled` 為 false（賣場還沒建立完成）時不開連線。回傳串流的連線狀態。 */
export function useSellerWorkspaceRealtime(
	enabled = true,
): SseConnectionStatus {
	const status = useNotificationStreamStatus();
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	const timerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);

	const invalidateWorkspace = React.useCallback(() => {
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
		if (!enabled) return;
		let closed = false;
		const clearTimer = () => {
			if (timerRef.current) clearTimeout(timerRef.current);
			timerRef.current = null;
		};
		const onSignal = (event: Event) => {
			if (closed) return;
			const detail = (event as CustomEvent<SellerWorkspaceSignalEventDetail>)
				.detail;
			if (detail?.kind === "change") {
				clearTimer();
				timerRef.current = setTimeout(() => {
					timerRef.current = null;
					invalidateWorkspace();
				}, SELLER_WORKSPACE_DEBOUNCE_MS);
			} else if (detail?.kind === "reconnected") {
				clearTimer();
				invalidateWorkspace();
			}
		};
		window.addEventListener(SELLER_WORKSPACE_SIGNAL_EVENT, onSignal);
		return () => {
			closed = true;
			clearTimer();
			window.removeEventListener(SELLER_WORKSPACE_SIGNAL_EVENT, onSignal);
		};
	}, [enabled, invalidateWorkspace]);

	return enabled ? status : "idle";
}
