"use client";

import { useSearchParams } from "next/navigation";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import {
	readPaymentStatusHint,
	readPaymentTradeNo,
} from "@/lib/payments/query";
import { usePaymentResolution } from "@/lib/payments/usePaymentResolution";

interface PaymentLandingClientProps {
	orderNo: string;
}

/**
 * PayUni Web redirect target per issue #760. PayUni sends buyers back
 * to `/orders/{orderNo}?status=...&trade_no=...&code=...` after the
 * payment flow; this client resolves the real status (the `status` /
 * `code` query params are UI hints only and can be forged) and routes
 * to the existing checkout success / fail surfaces.
 *
 * Direct visits (no `trade_no` query) — buyer typed the URL or hit a
 * bookmark — bounce to `/account/orders/{orderNo}`, the canonical
 * order detail page.
 */
export default function PaymentLandingClient({
	orderNo,
}: PaymentLandingClientProps) {
	return (
		<React.Suspense fallback={null}>
			<Inner orderNo={orderNo} />
		</React.Suspense>
	);
}

function Inner({ orderNo }: PaymentLandingClientProps) {
	const router = useObservedRouter("payment_landing");
	const params = useSearchParams();
	const tradeNo = readPaymentTradeNo(params);
	const initialStatus = readPaymentStatusHint(params);

	// No trade_no → not a PayUni redirect. Bounce to the canonical
	// order detail page so this surface stays purely for the redirect
	// flow. Guards against typed URLs / bookmarks.
	React.useEffect(() => {
		if (!tradeNo) {
			router.replace(`/account/orders/${encodeURIComponent(orderNo)}`);
		}
	}, [tradeNo, orderNo, router]);

	const { outcome, reason, isPolling } = usePaymentResolution({
		tradeNo,
		initialStatus,
	});

	React.useEffect(() => {
		if (!tradeNo || outcome === "pending") return;
		const url =
			outcome === "paid"
				? `/checkout/success?order=${encodeURIComponent(orderNo)}`
				: `/checkout/fail?order=${encodeURIComponent(orderNo)}${
						reason ? `&reason=${encodeURIComponent(reason)}` : ""
					}`;
		router.replace(url);
	}, [tradeNo, outcome, reason, orderNo, router]);

	if (!tradeNo) {
		// useEffect above is mid-flight redirecting; render nothing to
		// avoid a flash of the "確認付款中" card during the bounce.
		return null;
	}

	return (
		<main className="min-h-[calc(100vh-200px)] max-w-[580px] mx-auto py-10 px-5 pb-12 flex items-center justify-center">
			<div className="relative w-full bg-content1/90 border border-divider rounded-3xl p-10 px-7 pb-8 text-center flex flex-col items-center gap-3 shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_18px_40px_-22px_rgb(var(--color-ink-950-rgb)/0.12),0_6px_18px_-12px_rgb(var(--color-ink-950-rgb)/0.08)] backdrop-blur-md overflow-hidden dark:bg-content1/86 animate-checkout-pop">
				<span
					className="m-0 my-4 inline-block text-hk-primary"
					aria-hidden="true"
				>
					{isPolling ? (
						<AppIcon name="spinner" size={52} className="animate-spin" />
					) : (
						<AppIcon name="check" size={52} />
					)}
				</span>
				<h1 className="relative z-1 font-display text-xl md:text-2xl m-0 text-foreground">
					{isPolling ? "確認付款中…" : "處理完成"}
				</h1>
				<p className="relative z-1 m-0 text-foreground-soft text-sm max-w-[38ch]">
					{isPolling
						? "我們正在向銀行確認結果，請不要關閉這個頁面。"
						: outcome === "paid"
							? "付款成功，導向訂單頁。"
							: reason || "未付款成功，導向重試頁。"}
				</p>
			</div>
		</main>
	);
}
