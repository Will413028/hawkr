"use client";

import { errorText } from "@hawkr/api/connect/errors";
import { useSearchParams } from "next/navigation";
import * as React from "react";
import PaymentStatusCard, {
	type PaymentStatusKind,
} from "@/features/payments/components/PaymentStatusCard";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { getPayment } from "@/lib/payments/api";
import { pollPayment } from "@/lib/payments/poll";
import { readPaymentOrderNo } from "@/lib/payments/query";

export default function PaymentReturnClient() {
	return (
		<React.Suspense fallback={null}>
			<Inner />
		</React.Suspense>
	);
}

type Outcome = { kind: PaymentStatusKind; message?: string };

/**
 * Stripe 在 3DS 或銀行導向之後帶買家回來的頁面。網址只用來知道是哪一張訂單（`order`）；
 * 網址上 Stripe 附的 `redirect_status` 等參數一律不看，結果以 GetPayment 輪詢為準：
 * 已入帳去成功頁，買家還要動作（驗證失敗、取消）回付款步驟，其餘留在這裡說明。
 */
function Inner() {
	const router = useObservedRouter("payment_return");
	const params = useSearchParams();
	const orderNo = readPaymentOrderNo(params);
	const defaultErrors = useDefaultErrorText();
	const defaultErrorsRef = React.useRef(defaultErrors);
	defaultErrorsRef.current = defaultErrors;
	const [outcome, setOutcome] = React.useState<Outcome>({ kind: "waiting" });

	React.useEffect(() => {
		if (!orderNo) {
			router.replace("/account/orders");
			return;
		}
		const abort = new AbortController();
		pollPayment({ read: () => getPayment(orderNo), signal: abort.signal })
			.then((result) => {
				if (abort.signal.aborted) return;
				if (result.kind === "timeout") return setOutcome({ kind: "timeout" });
				const { view } = result;
				if (view.kind === "succeeded") {
					router.replace(
						`/checkout/success?order=${encodeURIComponent(orderNo)}`,
					);
					return setOutcome({ kind: "succeeded" });
				}
				if (view.kind === "anomaly") return setOutcome({ kind: "anomaly" });
				router.replace(`/orders/${encodeURIComponent(orderNo)}`);
			})
			.catch((e) => {
				if (!abort.signal.aborted) {
					setOutcome({
						kind: "error",
						message: errorText(e, defaultErrorsRef.current),
					});
				}
			});
		return () => abort.abort();
	}, [orderNo, router]);

	return (
		<PaymentStatusCard
			kind={outcome.kind}
			orderNo={orderNo}
			message={outcome.message}
		/>
	);
}
