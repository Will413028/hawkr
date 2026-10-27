"use client";

import { useSearchParams } from "next/navigation";
import * as React from "react";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import {
	readPaymentOrderNo,
	readPaymentStatusHint,
	readPaymentTradeNo,
} from "@/lib/payments/query";
import { usePaymentResolution } from "@/lib/payments/usePaymentResolution";

export default function PaymentReturnClient() {
	return (
		<React.Suspense fallback={null}>
			<Inner />
		</React.Suspense>
	);
}

/**
 * PayUni redirects buyers back here with `trade_no` (and possibly
 * `order` / `status`). Polling + status classification lives in
 * `usePaymentResolution`; this component is just the UX shell + the
 * route decision (push to /checkout/success or /checkout/fail).
 *
 * The new PayUni Web flow (issue #760) introduces a second landing
 * route at `/orders/{orderNo}`. That page will consume the same hook
 * but render the result inline rather than redirecting.
 */
function Inner() {
	const router = useObservedRouter("payment_return");
	const params = useSearchParams();
	const tradeNo = readPaymentTradeNo(params);
	const orderNo = readPaymentOrderNo(params);
	const initialStatus = readPaymentStatusHint(params);

	const { outcome, reason, isPolling } = usePaymentResolution({
		tradeNo,
		initialStatus,
	});

	// Friendlier "we're still trying" hint after ~6s — silences the worry
	// that the page is frozen when the bank is slow to respond.
	const [longWait, setLongWait] = React.useState(false);
	React.useEffect(() => {
		if (!isPolling) return;
		const t = window.setTimeout(() => setLongWait(true), 6000);
		return () => window.clearTimeout(t);
	}, [isPolling]);

	React.useEffect(() => {
		if (outcome === "pending") return;
		const url =
			outcome === "paid"
				? `/checkout/success?order=${encodeURIComponent(orderNo)}`
				: `/checkout/fail${
						orderNo ? `?order=${encodeURIComponent(orderNo)}` : ""
					}${
						reason
							? `${orderNo ? "&" : "?"}reason=${encodeURIComponent(reason)}`
							: ""
					}`;
		router.replace(url);
	}, [outcome, reason, orderNo, router]);

	return (
		<main className="min-h-[calc(100vh-200px)] max-w-[580px] mx-auto py-10 px-5 pb-12 flex items-center justify-center">
			<div className="relative w-full bg-content1/90 border border-divider rounded-3xl p-10 px-7 pb-8 text-center flex flex-col items-center gap-3 shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_18px_40px_-22px_rgba(46,70,94,0.24),0_6px_18px_-12px_rgba(15,24,33,0.16)] backdrop-blur-md overflow-hidden dark:bg-content1/86 animate-checkout-pop before:absolute before:inset-x-0 before:top-0 before:h-[140px] before:pointer-events-none before:z-0 before:bg-gradient-to-b before:from-warning-50 before:to-transparent">
				<span
					className="relative z-1 inline-flex items-center justify-center w-[148px] h-[148px] -mt-1.5 mb-[-4px] animate-checkout-icon-pop"
					aria-hidden="true"
				>
					<span className="inline-flex gap-2">
						<span className="inline-block w-2.5 h-2.5 rounded-full bg-warning animate-checkout-bounce" />
						<span className="inline-block w-2.5 h-2.5 rounded-full bg-warning animate-checkout-bounce [animation-delay:150ms]" />
						<span className="inline-block w-2.5 h-2.5 rounded-full bg-warning animate-checkout-bounce [animation-delay:300ms]" />
					</span>
				</span>
				<h1 className="relative z-1 font-display text-xl md:text-2xl m-0 text-foreground">
					{isPolling ? "確認付款中" : "處理完成"}
				</h1>
				<p className="relative z-1 m-0 text-foreground-soft text-sm max-w-[38ch]">
					{isPolling
						? "我們正在向銀行確認結果，請不要關閉這個頁面。"
						: outcome === "paid"
							? "付款成功，正在導向訂單頁。"
							: reason || "未付款成功，正在導向重試頁。"}
				</p>
				{isPolling && longWait && (
					<span
						className="relative z-1 inline-flex items-center gap-1.5 text-xs font-bold text-foreground-muted mt-2"
						role="status"
					>
						<svg
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
							className="animate-spin w-3.5 h-3.5"
							aria-hidden="true"
						>
							<circle cx="12" cy="12" r="10" />
							<path d="M12 6v6l4 2" strokeLinecap="round" />
						</svg>
						有點久了，再給銀行 10 秒…
					</span>
				)}
				{orderNo && (
					<div className="relative z-1 w-full max-w-[360px] mt-3 p-4 rounded-2xl bg-content2 border border-divider text-left flex flex-col gap-2">
						<div className="flex justify-between items-center gap-3">
							<span className="text-xs font-bold text-foreground-muted">
								訂單編號
							</span>
							<span className="text-xs font-extrabold text-foreground flex items-center gap-2">
								{orderNo}
							</span>
						</div>
					</div>
				)}
			</div>
		</main>
	);
}
