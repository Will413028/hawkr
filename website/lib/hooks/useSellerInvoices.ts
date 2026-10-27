"use client";

/**
 * 賣家面發票的 **BLOCK 類** 查詢（58A §B-2.2 (4)(c)）。
 *
 * 🔴 本檔每一支 hook 的 `enabled` 都必須含 `&& !blocked`。
 * 這些端點（16／30／31／33／72）的 capability 是 `CapInvoiceManage`，停權賣家打了必回
 * `9240`，而 `lib/api/client.ts` 會把 9240 廣播成全域停權事件 ⇒ S07 掛在停權白名單
 * 頁（`/seller-center/orders`）底下，不先 gate 就會在每次載入訂單詳情時觸發一次
 * 全域 invalidate 風暴（§B-2.3 (3) 查詢層）。
 *
 * 🔴 **放行類的 hook 不在本檔**，在 `./useSellerInvoiceRemedy.ts`——那邊加上
 * `!blocked` 就是在前端把後端的放行抵銷掉，而兩支查詢都回 200、沒有任何測試
 * 會叫（§B-2.0 T04）。兩類分檔就是為了讓這個差別在檔案層級看得見。
 *
 * 🔴 **零輪詢**（§0.4 ④）：發票端點走 `tierDefault` 限流桶、與該使用者其他請求
 * 共用額度。更新只有三個來源——mutation 後 `invalidateQueries`、頁面「重新整理」
 * 鍵、既有通知中心。Phase 1 gateway 沒有任何發票 SSE 事件，不要為它開第二條。
 */

import { SellerCapability } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import {
	fetchSellerInvoiceDocument,
	fetchSellerInvoiceJob,
	fetchSellerInvoiceOverview,
	listSellerInvoiceDocuments,
	listSellerReceivedInvoiceDocuments,
	type SellerInvoiceDocumentsQuery,
	type SellerReceivedInvoicesQuery,
} from "@/lib/invoices/seller-repo";
import type {
	InvoiceJobInfo,
	SellerInvoiceDocumentDetail,
	SellerInvoiceDocumentsResult,
	SellerInvoiceOverview,
	SellerPagedResult,
	SellerReceivedInvoiceItem,
} from "@/lib/invoices/seller-types";
import { queryKeys } from "@/lib/query/keys";
import { hasCapability, type MyStore } from "@/lib/seller/store";
import { useQueryIdentity } from "./useQueryIdentity";
import { useSellerSelf } from "./useSellerSelf";

interface BlockedQueryBase {
	loading: boolean;
	error: unknown;
	/** 停權 ⇒ 查詢從未發出。呼叫端據此渲染停權說明而不是「載入失敗」。 */
	blockedBySuspension: boolean;
	refresh: () => Promise<void>;
}

export interface UseSellerInvoiceOverviewResult extends BlockedQueryBase {
	overview: SellerInvoiceOverview | null;
}

/** 店家已讀到、而且沒有發票管理的能力（限制中）。 */
function invoiceManageBlocked(self: MyStore | null): boolean {
	return self !== null && !hasCapability(self, SellerCapability.INVOICE_MANAGE);
}

/** 端點 16（S01 唯一資料源）。 */
export function useSellerInvoiceOverview(): UseSellerInvoiceOverviewResult {
	const { isLoggedIn, activeRole } = useAuth();
	const { data: self } = useSellerSelf();
	const blocked = invoiceManageBlocked(self);
	const identity = useQueryIdentity();
	const enabled = isLoggedIn && activeRole === "seller" && !blocked;

	const query = useQuery({
		queryKey: queryKeys.seller.invoiceOverview(identity),
		queryFn: fetchSellerInvoiceOverview,
		enabled,
		// apiClient already retries GET failures once. This shared sidebar query
		// must not multiply an invoice outage into four requests (backend#3230).
		retry: false,
	});

	return {
		overview: enabled ? (query.data ?? null) : null,
		loading: enabled && query.isLoading,
		error: query.error,
		blockedBySuspension: blocked,
		refresh: async () => {
			await query.refetch();
		},
	};
}

export interface UseSellerInvoiceDocumentsResult extends BlockedQueryBase {
	// 端點 30 的分頁結果比標準分頁多一個 shipmentStatusDegraded（backend#3019）。
	// 用 SellerPagedResult 會把它在型別上抹掉，S06 就再也分不出「降級」與「還沒出貨」。
	page: SellerInvoiceDocumentsResult | null;
}

/**
 * 端點 30（S06 全頁、S07 段 1）。
 *
 * `params` 進 queryKey 的 object slot ⇒ 每個篩選組合各自一個快取槽，而
 * `invalidateQueries` 帶 `{}` 會部分比對整族。
 */
export function useSellerInvoiceDocuments(
	params: SellerInvoiceDocumentsQuery,
	options?: { enabled?: boolean },
): UseSellerInvoiceDocumentsResult {
	const { isLoggedIn, activeRole } = useAuth();
	const { data: self } = useSellerSelf();
	const blocked = invoiceManageBlocked(self);
	const identity = useQueryIdentity();
	const enabled =
		isLoggedIn &&
		activeRole === "seller" &&
		!blocked &&
		options?.enabled !== false;

	const query = useQuery({
		queryKey: queryKeys.seller.invoiceDocuments(
			identity,
			params as Record<string, unknown>,
		),
		queryFn: () => listSellerInvoiceDocuments(params),
		enabled,
	});

	return {
		page: enabled ? (query.data ?? null) : null,
		loading: enabled && query.isLoading,
		error: query.error,
		blockedBySuspension: blocked,
		refresh: async () => {
			await query.refetch();
		},
	};
}

export interface UseSellerInvoiceDocumentResult extends BlockedQueryBase {
	detail: SellerInvoiceDocumentDetail | null;
}

/** 端點 31（S07 段 2）。`documentId` 為空時不發（段 1 還沒回來）。 */
export function useSellerInvoiceDocument(
	documentId: string,
): UseSellerInvoiceDocumentResult {
	const { isLoggedIn, activeRole } = useAuth();
	const { data: self } = useSellerSelf();
	const blocked = invoiceManageBlocked(self);
	const identity = useQueryIdentity();
	const enabled =
		isLoggedIn && activeRole === "seller" && !blocked && documentId !== "";

	const query = useQuery({
		queryKey: queryKeys.seller.invoiceDocument(identity, documentId),
		queryFn: () => fetchSellerInvoiceDocument(documentId),
		enabled,
	});

	return {
		detail: enabled ? (query.data ?? null) : null,
		loading: enabled && query.isLoading,
		error: query.error,
		blockedBySuspension: blocked,
		refresh: async () => {
			await query.refetch();
		},
	};
}

export interface UseSellerReceivedInvoicesResult extends BlockedQueryBase {
	page: SellerPagedResult<SellerReceivedInvoiceItem> | null;
}

/**
 * 端點 72（S12 進項分頁）。
 *
 * 🔴 **停權時 BLOCK，與銷項同一條線**。58A §S12 寫「進項 tab 停權時仍可看（進項憑證
 * 是賣家的財產，與能否營業無關）」，但它自己指定的 capability 是 `CapInvoiceManage`，
 * 而 `gateway/internal/authz/policy.go` 的 `suspendedWhitelist` **刻意不含**它
 * （該檔逐字：「後者是擴張型／唯讀查看」）⇒ 停權賣家打端點 72 必回 `9240`。
 * 前端若照 58A 放行，做出來的是一頁必定 9240 的畫面 ＋ 一次全域停權廣播，
 * 正是 §B-2.0 T03 的「加太多」那一半。已回報衝突，未自行改規格。
 */
export function useSellerReceivedInvoices(
	params: SellerReceivedInvoicesQuery,
	options?: { enabled?: boolean },
): UseSellerReceivedInvoicesResult {
	const { isLoggedIn, activeRole } = useAuth();
	const { data: self } = useSellerSelf();
	const blocked = invoiceManageBlocked(self);
	const identity = useQueryIdentity();
	const enabled =
		isLoggedIn &&
		activeRole === "seller" &&
		!blocked &&
		options?.enabled !== false;

	const query = useQuery({
		queryKey: queryKeys.seller.invoiceReceivedDocuments(
			identity,
			params as Record<string, unknown>,
		),
		queryFn: () => listSellerReceivedInvoiceDocuments(params),
		enabled,
	});

	return {
		page: enabled ? (query.data ?? null) : null,
		loading: enabled && query.isLoading,
		error: query.error,
		blockedBySuspension: blocked,
		refresh: async () => {
			await query.refetch();
		},
	};
}

export interface UseSellerInvoiceJobResult extends BlockedQueryBase {
	job: InvoiceJobInfo | null;
}

/**
 * 端點 33（S08 唯一資料源）。
 *
 * 🔴 停權 **BLOCK**，S08 **不在**白名單——停權賣家的回報落點是 S07 的 ⑦（走端點
 * 62R，`CapInvoiceTaxRemedy`），不是這一頁（58A §S08 權限列）。
 */
export function useSellerInvoiceJob(jobId: string): UseSellerInvoiceJobResult {
	const { isLoggedIn, activeRole } = useAuth();
	const { data: self } = useSellerSelf();
	const blocked = invoiceManageBlocked(self);
	const identity = useQueryIdentity();
	const enabled =
		isLoggedIn && activeRole === "seller" && !blocked && jobId !== "";

	const query = useQuery({
		queryKey: queryKeys.seller.invoiceJob(identity, jobId),
		queryFn: () => fetchSellerInvoiceJob(jobId),
		enabled,
	});

	return {
		job: enabled ? (query.data ?? null) : null,
		loading: enabled && query.isLoading,
		error: query.error,
		blockedBySuspension: blocked,
		refresh: async () => {
			await query.refetch();
		},
	};
}
