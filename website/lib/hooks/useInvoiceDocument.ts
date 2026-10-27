"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import {
	fetchInvoiceDocument,
	INVOICE_BIZ_CODE,
	type InvoiceNotificationResendResult,
	invoiceBizCode,
	newIdempotencyKey,
	resendBuyerInvoiceNotification,
} from "@/lib/invoices/buyer-repo";
import type { InvoiceDocumentDetail } from "@/lib/invoices/types";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

export interface UseInvoiceDocumentResult {
	detail: InvoiceDocumentDetail | null;
	loading: boolean;
	error: string | null;
}

/**
 * 單一發票／收據文件的詳情（58A §B05，端點 69）。
 *
 * document-scoped：授權述詞是「這份文件的買受人是不是你」，路徑上沒有 `orderNo`
 * ⇒ 發票通知的深連結（`/account/invoices/#{documentId}`）不必先查任何東西就能落地。
 *
 * 🔴 越權與不存在都是 9301（Tier A）⇒ 呼叫端的致命錯誤態文案必須與「不存在」
 * 一致，不得出現「你沒有權限查看」。
 */
export function useInvoiceDocument(
	documentId: string,
): UseInvoiceDocumentResult {
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();

	const documentQuery = useQuery({
		queryKey: queryKeys.buyer.invoiceDocument(identity, documentId),
		queryFn: () => fetchInvoiceDocument(documentId),
		enabled: hydrated && isLoggedIn && documentId.length > 0,
		meta: {
			isExpectedError: (error: unknown) =>
				invoiceBizCode(error) === INVOICE_BIZ_CODE.DOCUMENT_NOT_FOUND,
		},
	});

	return {
		detail: documentQuery.data ?? null,
		loading: documentQuery.isLoading,
		error:
			documentQuery.error instanceof Error
				? "這份文件可能已被移除，或連結不正確。"
				: null,
	};
}

export interface UseResendInvoiceNotificationResult {
	resend: () => void;
	sending: boolean;
	/** 成功寄出時的結果（含遮罩收件信箱）；沒寄過為 `null`。 */
	result: InvoiceNotificationResendResult | null;
	/** 拿到 `9352`：剛剛才寄過，10 分鐘內不重複寄送。**不是失敗**。 */
	throttled: boolean;
	/** 真正的失敗（含 `9301`）。`9352` 不走這裡。 */
	error: unknown;
}

/**
 * 端點 71｜重寄發票／收據通知（B05）。
 *
 * 🔴 `9352` 從 `error` 裡分出來單獨回報：它是「剛剛才寄過」而不是「寄不出去」，
 * 混在一起的話買家會以為系統壞了，然後去開一張申訴——而那正是重寄鍵要避免的事。
 *
 * 🔴 成功後 **invalidate 文件詳情**：重寄會在 `timeline` 追加一筆，不失效的話
 * 買家按完之後看到的歷程還是舊的，等於沒有任何證據說明剛剛發生了什麼。
 *
 * ⚠️ 冪等鍵**每次按下都換新的一把**（理由見 `resendBuyerInvoiceNotification`）。
 */
export function useResendInvoiceNotification(
	documentId: string,
): UseResendInvoiceNotificationResult {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();

	const mutation = useMutation({
		mutationFn: () =>
			resendBuyerInvoiceNotification(documentId, newIdempotencyKey()),
		onSuccess: async () => {
			await queryClient.invalidateQueries({
				queryKey: queryKeys.buyer.invoiceDocument(identity, documentId),
			});
		},
	});

	const throttled =
		mutation.error != null &&
		invoiceBizCode(mutation.error) ===
			INVOICE_BIZ_CODE.NOTIFICATION_RESEND_THROTTLED;

	return {
		resend: () => mutation.mutate(),
		sending: mutation.isPending,
		result: mutation.data ?? null,
		throttled,
		error: throttled ? null : mutation.error,
	};
}
