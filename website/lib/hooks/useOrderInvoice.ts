"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import {
	fetchOrderInvoice,
	INVOICE_BIZ_CODE,
	invoiceBizCode,
} from "@/lib/invoices/buyer-repo";
import type { OrderInvoice } from "@/lib/invoices/types";
import {
	NOTIFICATION_NEW_EVENT,
	type NotificationNewEventDetail,
} from "@/lib/notifications/events";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

export interface UseOrderInvoiceResult {
	invoice: OrderInvoice | null;
	loading: boolean;
	/**
	 * 這筆訂單沒有發票（零元訂單、尚未成立開立義務，或這張訂單不是你的）。
	 *
	 * 🔴 Tier A：越權與不存在回**同一個** 9301 ⇒ 呼叫端整張卡不渲染，且不得顯示
	 * 任何「你沒有權限」字樣——那會把「不存在」與「不是你的」區分出來。
	 */
	notFound: boolean;
	/** 5xx／網路失敗。9301 不走這裡（它是狀態不是錯誤）。 */
	error: string | null;
	refresh: () => Promise<void>;
}

function isNotApplicable(error: unknown): boolean {
	return invoiceBizCode(error) === INVOICE_BIZ_CODE.DOCUMENT_NOT_FOUND;
}

/**
 * 一張訂單的發票現況（58A §B04，端點 8）。
 *
 * **零輪詢**（§0.4 ④）。三個刷新來源，全部事件驅動：
 *
 *   1. **通知**——root layout 的 `NotificationUnreadProvider` 已經開著唯一一條通知
 *      SSE，並把每一筆 `notification.new` 用
 *      `NOTIFICATION_NEW_EVENT` 廣播到 window 上。這裡聽那個 window 事件，
 *      **不另開第二條 `useNotificationStream`**：多開一條會多佔一個同源連線額度，
 *      而且兩個訂閱者的 Last-Event-ID resume cursor 會互相覆蓋。
 *      買家可見的發票通知每一列的 `refType` 都是 `ORDER`、`refId` 都是 `order_no`。
 *   2. **使用者主動重整**——呼叫端在 `PROCESSING` 時渲染一顆「重新整理」文字鍵。
 *   3. **頁面重新可見**——`refetchOnWindowFocus` 明寫為 `true`：本 repo 的
 *      QueryClient 預設把它關掉了（`lib/query/client.ts`），不寫就沒有這個行為。
 */
export function useOrderInvoice(orderNo: string): UseOrderInvoiceResult {
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.orderInvoice(identity, orderNo);

	const invoiceQuery = useQuery({
		queryKey,
		queryFn: () => fetchOrderInvoice(orderNo),
		enabled: hydrated && isLoggedIn && orderNo.length > 0,
		refetchOnWindowFocus: true,
		// 9301 是「這筆訂單沒有發票」——零元訂單天天都有，上報它等於把正常狀態
		// 灌進 Sentry。
		meta: { isExpectedError: isNotApplicable },
	});

	React.useEffect(() => {
		if (!hydrated || !isLoggedIn || !orderNo) return;
		const onNotification = (event: Event) => {
			const detail = (event as CustomEvent<NotificationNewEventDetail>).detail;
			const item = detail?.item;
			if (!item) return;
			if (item.refType !== "ORDER") return;
			if (String(item.refId ?? "") !== orderNo) return;
			void queryClient.invalidateQueries({ queryKey });
		};
		window.addEventListener(NOTIFICATION_NEW_EVENT, onNotification);
		return () => {
			window.removeEventListener(NOTIFICATION_NEW_EVENT, onNotification);
		};
	}, [hydrated, isLoggedIn, orderNo, queryClient, queryKey]);

	const error = invoiceQuery.error;
	const notFound = error != null && isNotApplicable(error);

	return {
		invoice: invoiceQuery.data ?? null,
		loading: invoiceQuery.isLoading,
		notFound,
		error:
			error instanceof Error && !notFound
				? "發票資料載入失敗，請稍後再試。"
				: null,
		refresh: async () => {
			await invoiceQuery.refetch();
		},
	};
}
