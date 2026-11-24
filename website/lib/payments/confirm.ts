import type { Payment } from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import type { Stripe, StripeElements } from "@stripe/stripe-js";
import type { AppLocale } from "@/lib/intl/locales";
import { localizeHref } from "@/lib/intl/navigation";
import { type PollResult, pollPayment } from "./poll";

/** 3DS／銀行導向完成後 Stripe 帶買家回來的網址；只帶訂單編號，結果一律以 GetPayment 為準。 */
export function paymentReturnUrl(
	origin: string,
	locale: AppLocale,
	orderNo: string,
): string {
	return `${origin}${localizeHref(`/payment/return?order=${encodeURIComponent(orderNo)}`, locale)}`;
}

export type ConfirmOutcome =
	| { kind: "declined"; message: string | undefined }
	| PollResult;

/**
 * 送出付款：`stripe.confirmPayment` 只是把資料交給 Stripe，它回報的成功不算結果。
 * 沒有被即時拒絕（`error`）時，一律以 `read`（GetPayment）輪詢到伺服器說了算。
 * `declined.message` 是 Stripe 依 Elements 的 locale 給買家的拒絕說明。
 */
export async function confirmAndSettle(options: {
	stripe: Pick<Stripe, "confirmPayment">;
	elements: StripeElements;
	returnUrl: string;
	read: () => Promise<Payment | undefined>;
	intervalMs?: number;
	maxAttempts?: number;
	signal?: AbortSignal;
}): Promise<ConfirmOutcome> {
	const { stripe, elements, returnUrl, read, ...poll } = options;
	const result = await stripe.confirmPayment({
		elements,
		confirmParams: { return_url: returnUrl },
		redirect: "if_required",
	});
	if (result.error) return { kind: "declined", message: result.error.message };
	return pollPayment({ read, ...poll });
}
