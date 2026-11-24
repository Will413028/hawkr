import {
	type Payment,
	PaymentStatus,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";

/**
 * 付款狀態到畫面狀態的唯一決定處。結果只認 `GetPayment`／`CreatePayment` 回的 `Payment`（webhook 驅動）；
 * client 端 `confirmPayment` 的回報、網址上的任何 status 都不是結果。
 *
 * - `pay`：狀態 AWAITING_PAYMENT 且帶 Stripe session，買家可以付款。
 * - `wait`：CREATING、PROCESSING，或 AWAITING_PAYMENT 但沒有 session（金流商那邊已往前），稍後以 GetPayment 重讀。
 * - `succeeded`：只有 SUCCEEDED。
 * - `retry`：CANCELED、CREATE_FAILED；訂單仍可付時再呼叫 CreatePayment 會換新的嘗試。
 * - `anomaly`：買家不能重試，等營運處理。
 * - `unknown`：UNSPECIFIED 或不認得的值；不是成功，也不是終局。
 */
export type PaymentView =
	| { kind: "pay"; clientSecret: string; publishableKey: string }
	| { kind: "wait" }
	| { kind: "succeeded" }
	| { kind: "retry" }
	| { kind: "anomaly" }
	| { kind: "unknown" };

export function paymentView(payment: Payment | undefined): PaymentView {
	if (!payment) return { kind: "unknown" };
	switch (payment.status) {
		case PaymentStatus.AWAITING_PAYMENT: {
			const provider = payment.session?.provider;
			if (provider?.case === "stripe" && provider.value.clientSecret) {
				return {
					kind: "pay",
					clientSecret: provider.value.clientSecret,
					publishableKey: provider.value.publishableKey,
				};
			}
			return { kind: "wait" };
		}
		case PaymentStatus.CREATING:
		case PaymentStatus.PROCESSING:
			return { kind: "wait" };
		case PaymentStatus.SUCCEEDED:
			return { kind: "succeeded" };
		case PaymentStatus.CANCELED:
		case PaymentStatus.CREATE_FAILED:
			return { kind: "retry" };
		case PaymentStatus.ANOMALY:
			return { kind: "anomaly" };
		default:
			return { kind: "unknown" };
	}
}

/** 輪詢可以停下的畫面狀態：除了 `wait`、`unknown`，其餘都是要買家動作或終局。 */
export type SettledView = Exclude<PaymentView, { kind: "wait" | "unknown" }>;

export function isSettled(view: PaymentView): view is SettledView {
	return view.kind !== "wait" && view.kind !== "unknown";
}
