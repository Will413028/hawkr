"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import {
	type BuyerInvoiceDocumentsQuery,
	listBuyerInvoiceDocuments,
} from "@/lib/invoices/buyer-repo";
import type { BuyerInvoiceDocumentListResult } from "@/lib/invoices/types";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

export interface UseBuyerInvoiceDocumentsResult {
	/**
	 * 最後一份成功回來的結果。背景 refetch 失敗時**保留**它（TanStack Query 不會
	 * 因為 refetch 失敗而清掉 `data`）⇒ 呼叫端的致命錯誤態判 `!page` 而不是
	 * `error || !page`，否則一次網路抖動就把整頁換成錯誤畫面。
	 */
	page: BuyerInvoiceDocumentListResult | null;
	loading: boolean;
	error: unknown;
	refresh: () => Promise<void>;
}

/**
 * 買家發票中心的列表（58A §B09，端點 68）。
 *
 * `enabled` 三段與 `useInvoiceDocument` 一致：`hydrated` 之前 auth 還沒從 storage
 * 還原，此時發請求會帶著空身分打出去、拿回 401，然後在畫面上留下一個假的錯誤態。
 *
 * `options.enabled` 給呼叫端做**輸入驗證**用：日期格式不合法時整支不發，因為端點 68
 * 對壞掉的 `date_from` 回 400，而使用者每打一個字都會觸發一次。
 */
export function useBuyerInvoiceDocuments(
	params: BuyerInvoiceDocumentsQuery,
	options?: { enabled?: boolean },
): UseBuyerInvoiceDocumentsResult {
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const enabled = hydrated && isLoggedIn && options?.enabled !== false;

	const query = useQuery({
		queryKey: queryKeys.buyer.invoiceDocuments(
			identity,
			params as Record<string, unknown>,
		),
		queryFn: () => listBuyerInvoiceDocuments(params),
		enabled,
	});

	return {
		page: enabled ? (query.data ?? null) : null,
		loading: enabled && query.isLoading,
		error: query.error,
		refresh: async () => {
			await query.refetch();
		},
	};
}
