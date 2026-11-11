import { timestampDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError, type Transport } from "@connectrpc/connect";
import { createQueryOptions } from "@connectrpc/connect-query";
import {
	type PaymentAccount,
	PaymentAdminService,
} from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import { skipToken } from "@tanstack/react-query";

// 商家詳情收款帳戶卡的純邏輯（讀取狀態、快取 key、重新整理後的重讀判斷）；元件在 PaymentAccountCard.tsx，查詢 hook 在 use-store-detail.ts。

export type PaymentAccountState =
	| { state: "loading" }
	| { state: "error"; error?: unknown }
	| { state: "notFound" }
	| { state: "ready"; account: PaymentAccount };

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

/** 排入重新讀取後再讀的間隔：RefreshPaymentAccount 不等結果。 */
export const REFRESH_REREAD_MS = 5000;
/** 最多再讀幾次（間隔 × 次數是上限，約 30 秒）：背景工作沒在這段時間完成就停，操作者可以再按一次。 */
export const REFRESH_REREAD_MAX_ATTEMPTS = 6;

/** 帳戶的 fetch_time（毫秒）；沒有時是 undefined。 */
export function fetchTimeMs(account: PaymentAccount | undefined) {
	return account?.fetchTime
		? timestampDate(account.fetchTime).getTime()
		: undefined;
}

/** 重新整理的重讀是否可以停：fetch_time 與按下時不同（背景工作寫回了）。 */
export function rereadFinished(
	baselineMs: number | undefined,
	account: PaymentAccount | undefined,
): boolean {
	const now = fetchTimeMs(account);
	return now !== undefined && now !== baselineMs;
}
