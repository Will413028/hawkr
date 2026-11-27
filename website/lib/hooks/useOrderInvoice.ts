"use client";

import { StateRefreshHint } from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { accountQueryFn, accountQueryPending } from "@/lib/auth/account-query";
import { useAccountStatus } from "@/lib/auth/use-account-status";
import {
	fetchOrderInvoice,
	INVOICE_BIZ_CODE,
	invoiceBizCode,
} from "@/lib/invoices/buyer-repo";
import type { OrderInvoice } from "@/lib/invoices/types";
import {
	NOTIFICATION_SIGNAL_EVENT,
	type NotificationSignalEventDetail,
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
 *   1. **通知**——root layout 的 `NotificationUnreadProvider` 已經開著唯一一條通知串流，
 *      並把每個收件匣訊號用 `NOTIFICATION_SIGNAL_EVENT` 廣播到 window 上。這裡聽那個 window 事件，
 *      **不另開第二條串流**：多開一條會多佔一個同源連線額度。訊號不帶通知內容，
 *      只帶狀態重讀提示：提示含 `ORDER`（訂單狀態有變）就重讀這張訂單的發票。
 *   2. **使用者主動重整**——呼叫端在 `PROCESSING` 時渲染一顆「重新整理」文字鍵。
 *   3. **頁面重新可見**——`refetchOnWindowFocus` 明寫為 `true`：本 repo 的
 *      QueryClient 預設把它關掉了（`lib/query/client.ts`），不寫就沒有這個行為。
 */
export function useOrderInvoice(orderNo: string): UseOrderInvoiceResult {
	const status = useAccountStatus();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.orderInvoice(identity, orderNo);

	const invoiceQuery = useQuery({
		queryKey,
		queryFn: accountQueryFn(status, () => fetchOrderInvoice(orderNo)),
		enabled: orderNo.length > 0,
		refetchOnWindowFocus: true,
		meta: { isExpectedError: isNotApplicable },
	});

	React.useEffect(() => {
		if (status !== "signedIn" || !orderNo) return;
		const onNotification = (event: Event) => {
			const detail = (event as CustomEvent<NotificationSignalEventDetail>)
				.detail;
			if (!detail?.hints.includes(StateRefreshHint.ORDER)) return;
			void queryClient.invalidateQueries({ queryKey });
		};
		window.addEventListener(NOTIFICATION_SIGNAL_EVENT, onNotification);
		return () => {
			window.removeEventListener(NOTIFICATION_SIGNAL_EVENT, onNotification);
		};
	}, [status, orderNo, queryClient, queryKey]);

	const error = invoiceQuery.error;
	const notFound = error != null && isNotApplicable(error);

	return {
		invoice: invoiceQuery.data ?? null,
		loading: accountQueryPending(status, invoiceQuery.isLoading),
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
