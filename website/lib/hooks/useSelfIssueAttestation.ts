/**
 * 自行開票責任條款（58 §6.1 端點 21／22，58A §S04）。
 *
 * 🔴 **本 hook 一定要搭端點 17 一起用**：CTA 的判準是 T04 的 allowed-from
 * （`taxReviewStatus === "SUBMITTED"`），**不是** `effective === false`。
 * 兩次踩過同一個坑、理由不同而結論一樣：
 *   ① 端點 19 在送審那一句就寫 `attested_at` ⇒ 若拿「簽過沒」當判準，
 *      主 CTA 對任何送過審的賣家永遠不渲染，`SELF_ISSUE_PENDING` 不可達。
 *   ② `effective` 現在錨在 `SELF_ISSUE_PENDING` ⇒ `effective === false`
 *      在 `DRAFT` 也成立，會對還沒送審的賣家渲染一顆必定回 9321 的鍵。
 * ⇒ **CTA 的判準是「這條邊現在走不走得通」，不是某個布林旗標。**
 *
 * 🔴 capability 是 `CapInvoiceTaxRemedy`，**在停權白名單內**：鎖住它，因違規停權的
 * 賣家補不了簽、也推不進待審——而那正是解除停權的路徑。所以 `enabled` 不得加
 * `!isSuspended`。
 */
"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { isInvoiceBizCode } from "@/lib/invoices/errorCopy";
import {
	acceptSelfIssueAttestation,
	fetchSelfIssueAttestation,
	SELLER_INVOICE_BIZ_CODE,
} from "@/lib/invoices/seller-repo";
import type { SelfIssueAttestation } from "@/lib/invoices/seller-types";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

export interface UseSelfIssueAttestationResult {
	attestation: SelfIssueAttestation | null;
	/** 還沒建立稅務身分（404 ＋ `9320`）。**不是錯誤態**。 */
	taxProfileMissing: boolean;
	loading: boolean;
	error: unknown;
	refresh: () => Promise<void>;
}

/**
 * 端點 21。
 *
 * 🔴 **「恆有回應」只對已建檔的賣家成立。** 還沒建立稅務身分的賣家拿到的是
 * 404 ＋ `9320`（2026-09-01 dev 實測）——那是每個新賣家的常態，當成錯誤會把
 * 整頁渲染成紅字，而重試一百次也不會變。
 */
export function useSelfIssueAttestation(): UseSelfIssueAttestationResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const enabled = isLoggedIn && activeRole === "seller";

	const query = useQuery({
		queryKey: queryKeys.seller.invoiceAttestation(identity),
		queryFn: fetchSelfIssueAttestation,
		enabled,
		meta: { isExpectedError: isTaxProfileMissing },
	});

	const missing = query.error != null && isTaxProfileMissing(query.error);

	return {
		attestation: enabled ? (query.data ?? null) : null,
		taxProfileMissing: missing,
		loading: enabled && query.isLoading,
		error: missing ? null : query.error,
		refresh: async () => {
			await query.refetch();
		},
	};
}

function isTaxProfileMissing(error: unknown): boolean {
	return isInvoiceBizCode(error, SELLER_INVOICE_BIZ_CODE.TAX_PROFILE_NOT_FOUND);
}

export interface UseAcceptSelfIssueAttestationResult {
	accept: () => void;
	accepting: boolean;
	error: unknown;
}

/**
 * 端點 22。
 *
 * @param currentTermsVersion 端點 21 的 `currentTermsVersion`（**當期版本**）。
 *   參數刻意這樣命名：拿賣家已簽的 `termsVersion` 送出，未簽過的人送的是空字串，
 *   後端必回 400，而那是一個看不出原因的失敗。
 */
export function useAcceptSelfIssueAttestation(
	currentTermsVersion: string,
): UseAcceptSelfIssueAttestationResult {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();

	const mutation = useMutation({
		mutationFn: () => acceptSelfIssueAttestation({ currentTermsVersion }),
		onSuccess: async () => {
			// 條款 ＋ 稅務身分 ＋ 總覽三者都會變：補簽會把賣家推進 SELF_ISSUE_PENDING，
			// 而 S01 卡 1 顯示的正是那個狀態。
			await Promise.all([
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceAttestation(identity),
				}),
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceTaxProfile(identity),
				}),
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceOverview(identity),
				}),
			]);
		},
	});

	return {
		accept: () => mutation.mutate(),
		accepting: mutation.isPending,
		error: mutation.error,
	};
}
