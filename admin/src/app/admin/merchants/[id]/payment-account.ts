import { Code, ConnectError, type Transport } from "@connectrpc/connect";
import { createQueryOptions } from "@connectrpc/connect-query";
import {
	type PaymentAccount,
	PaymentAdminService,
} from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import { PaymentAccountStatus } from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import { skipToken } from "@tanstack/react-query";

// 商家詳情收款帳戶卡的純邏輯（讀取狀態、快取 key）；元件在 PaymentAccountCard.tsx，查詢 hook 在 use-store-detail.ts。

export type PaymentAccountState =
	| { state: "loading" }
	| { state: "error"; error?: unknown }
	| { state: "notFound" }
	| { state: "ready"; account: PaymentAccount };

/**
 * 外部帳戶已建立（建立的兩步都成功）：看契約的狀態，不看外部帳戶 ID。第一步成功後列上就有外部 ID，第二步還沒成功的半建立列對外仍是 NOT_CREATED，
 * 這時 RefreshPaymentAccount 回 failed_precondition。
 */
export function paymentAccountCreated(account: PaymentAccount): boolean {
	return account.status !== PaymentAccountStatus.NOT_CREATED;
}

/** 讀取狀態的規則同 accountEmailOf：有錯誤就是錯誤（not_found 是尚未建立），成功卻沒有帳戶是讀取錯誤。 */
export function paymentAccountOf(result: {
	error: unknown;
	data: { account?: PaymentAccount } | undefined;
}): PaymentAccountState {
	if (result.error != null) {
		return result.error instanceof ConnectError &&
			result.error.code === Code.NotFound
			? { state: "notFound" }
			: { state: "error", error: result.error };
	}
	if (result.data === undefined) return { state: "loading" };
	if (!result.data.account) return { state: "error" };
	return { state: "ready", account: result.data.account };
}

/** GetPaymentAccount 的 query options；讀（usePaymentAccount）與寫（重試建立的回應寫進快取）用同一份 key。 */
export function paymentAccountQueryOptions(
	transport: Transport,
	storeId: string | undefined,
) {
	return createQueryOptions(
		PaymentAdminService.method.getPaymentAccount,
		storeId ? { storeId } : skipToken,
		{ transport },
	);
}
