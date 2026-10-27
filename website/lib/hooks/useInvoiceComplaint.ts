"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import {
	fetchInvoiceComplaint,
	INVOICE_BIZ_CODE,
	invoiceBizCode,
} from "@/lib/invoices/buyer-repo";
import type { InvoiceComplaint } from "@/lib/invoices/types";
import {
	NOTIFICATION_NEW_EVENT,
	type NotificationNewEventDetail,
} from "@/lib/notifications/events";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

export interface UseInvoiceComplaintResult {
	complaint: InvoiceComplaint | null;
	loading: boolean;
	/** 9309（Tier A：越權與不存在同碼）⇒ 呼叫端不得顯示「無權操作」。 */
	notFound: boolean;
	error: string | null;
	refresh: () => Promise<void>;
	/**
	 * 三支動作端點回的是**轉移後的完整單筆** ⇒ 直接覆蓋這一格快取，不再打一次
	 * 端點 15（§B07 三支動作端點共通規則第 2 條）。
	 */
	replace: (next: InvoiceComplaint) => void;
}

/**
 * 一件發票申訴案件（58A §B07，端點 15）。
 *
 * **零輪詢**（§0.4 ④）。刷新來源與 `useOrderInvoice` 相同：既有那條唯一的通知
 * SSE 廣播到 window 上的 `notification.new`（P9 `invoice.complaint.responded`
 * 會產生買家可見通知，`refType === "ORDER"`、`refId === order_no`）＋ 使用者
 * 主動重整 ＋ `refetchOnWindowFocus`。
 */
export function useInvoiceComplaint(
	orderNo: string,
	complaintId: string,
): UseInvoiceComplaintResult {
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const queryKey = queryKeys.buyer.orderInvoiceComplaint(
		identity,
		orderNo,
		complaintId,
	);

	const enabled =
		hydrated && isLoggedIn && orderNo.length > 0 && complaintId.length > 0;

	const complaintQuery = useQuery({
		queryKey,
		queryFn: () => fetchInvoiceComplaint(orderNo, complaintId),
		enabled,
		refetchOnWindowFocus: true,
		meta: {
			isExpectedError: (error: unknown) =>
				invoiceBizCode(error) === INVOICE_BIZ_CODE.COMPLAINT_NOT_FOUND,
		},
	});

	React.useEffect(() => {
		if (!enabled) return;
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
	}, [enabled, orderNo, queryClient, queryKey]);

	const error = complaintQuery.error;
	const notFound =
		error != null &&
		invoiceBizCode(error) === INVOICE_BIZ_CODE.COMPLAINT_NOT_FOUND;

	return {
		complaint: complaintQuery.data ?? null,
		loading: complaintQuery.isLoading,
		notFound,
		error:
			error instanceof Error && !notFound
				? "申訴案件載入失敗，請稍後再試。"
				: null,
		refresh: async () => {
			await complaintQuery.refetch();
		},
		replace: (next) => {
			queryClient.setQueryData(queryKey, next);
		},
	};
}
