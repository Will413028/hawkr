import type { Payment } from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import { paymentAppClient } from "@/lib/connect/payment";
import { type PollResult, pollPayment } from "./poll";
import { isSettled, paymentView } from "./state";

/** 對一張待付款訂單建立付款（冪等；訂單仍可付時，CANCELED／CREATE_FAILED 之後再呼叫會換新的嘗試）。 */
export async function createPayment(
	orderNo: string,
): Promise<Payment | undefined> {
	const res = await paymentAppClient().createPayment({ orderNo });
	return res.payment;
}

/** 讀買家自己的付款；結果以它為準。 */
export async function getPayment(
	orderNo: string,
): Promise<Payment | undefined> {
	const res = await paymentAppClient().getPayment({ orderNo });
	return res.payment;
}

/**
 * 進入付款步驟：CreatePayment，回 CREATING（背景補做）等還沒好的狀態時，以 GetPayment 輪詢到可以付款或終局。
 */
export async function openPayment(
	orderNo: string,
	signal?: AbortSignal,
): Promise<PollResult> {
	const payment = await createPayment(orderNo);
	const view = paymentView(payment);
	if (isSettled(view)) return { kind: "settled", view };
	return pollPayment({ read: () => getPayment(orderNo), signal });
}
