/**
 * 免用統一發票與收據代製授權（58 §6.1 端點 28／29，58A §S05）。
 *
 * 🔴 capability 是 `CapInvoiceManage`／**停權 BLOCK**，且**不在停權白名單內**：
 * 授權平台代製收據 ＝ 新增一種對外出具憑證的能力，屬**擴張型**操作。
 * 白名單放的是收縮型與救濟型（補稅務件、申訴），兩者不可混為一談。
 */
"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { isInvoiceBizCode } from "@/lib/invoices/errorCopy";
import {
	confirmSellerReceiptAuthorization,
	fetchSellerReceiptAuthorization,
	SELLER_INVOICE_BIZ_CODE,
} from "@/lib/invoices/seller-repo";
import type { SellerReceiptAuthorization } from "@/lib/invoices/seller-types";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

export interface UseSellerReceiptAuthorizationResult {
	authorization: SellerReceiptAuthorization | null;
	loading: boolean;
	error: unknown;
	refresh: () => Promise<void>;
}

/**
 * 端點 28。
 *
 * 🔴 **還沒建立稅務身分的賣家會拿到 404 ＋ `9320`，那不是錯誤態。**
 * （2026-09-01 dev 實測發現：原本直接把它當 error，畫面渲染成「讀取授權資料時發生
 * 問題」＋重試鍵——而那是**每個新賣家**進這一頁的樣子，重試一百次也不會變。）
 * 語意上「沒有稅務身分」就等於「沒有取得免用統一發票核定」⇒ 折成 `NOT_APPROVED`
 * 的 gate，與後端對已建檔但未核定的賣家給的答案一致。
 */
export function useSellerReceiptAuthorization(): UseSellerReceiptAuthorizationResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const enabled = isLoggedIn && activeRole === "seller";

	const query = useQuery({
		queryKey: queryKeys.seller.invoiceReceiptAuth(identity),
		queryFn: fetchSellerReceiptAuthorization,
		enabled,
		meta: { isExpectedError: isTaxProfileMissing },
	});

	const taxProfileMissing =
		query.error != null && isTaxProfileMissing(query.error);

	return {
		authorization: enabled
			? (query.data ?? (taxProfileMissing ? NOT_APPROVED_FALLBACK : null))
			: null,
		loading: enabled && query.isLoading,
		error: taxProfileMissing ? null : query.error,
		refresh: async () => {
			await query.refetch();
		},
	};
}

function isTaxProfileMissing(error: unknown): boolean {
	return isInvoiceBizCode(error, SELLER_INVOICE_BIZ_CODE.TAX_PROFILE_NOT_FOUND);
}

/**
 * `9320` 的投影。**每一欄都是「沒有」的正確答案**，不是佔位值：
 * 沒有稅務身分 ⇒ 沒核定過、沒授權過、沒有依據文件。
 */
const NOT_APPROVED_FALLBACK: SellerReceiptAuthorization = {
	eligible: false,
	ineligibleReason: "NOT_APPROVED",
	approvalBasisKey: "",
	approvedAt: "",
	approvalValidUntil: "",
	authorizedAt: "",
	consentVersion: "",
	receiptTemplateKey: "",
	version: "0",
	approvalBasisAssetId: "",
	approvalBasisAvailable: false,
	approvalBasisUnavailableReason: "NOT_DESIGNATED",
};

export interface UseConfirmReceiptAuthorizationResult {
	confirm: () => void;
	confirming: boolean;
	error: unknown;
}

/** 端點 29。 */
export function useConfirmReceiptAuthorization(
	expectedVersion: string,
): UseConfirmReceiptAuthorizationResult {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();

	const mutation = useMutation({
		mutationFn: () => confirmSellerReceiptAuthorization({ expectedVersion }),
		onSuccess: async () => {
			await Promise.all([
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceReceiptAuth(identity),
				}),
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceOverview(identity),
				}),
			]);
		},
	});

	return {
		confirm: () => mutation.mutate(),
		confirming: mutation.isPending,
		error: mutation.error,
	};
}
