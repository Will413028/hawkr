"use client";

import * as React from "react";
import { fetchPayment, syncPayment } from "@/lib/orders/repo";
import { classifyPaymentStatus, type PaymentOutcome } from "./status";

export interface UsePaymentResolutionOptions {
	/**
	 * PayUni trade_no carried back on the redirect URL. Required for
	 * truth — without it we can only act on the (untrusted) query
	 * `initialStatus`.
	 */
	tradeNo: string;
	/**
	 * Status string copied from the PayUni redirect URL. Used ONLY as
	 * a fallback signal when `tradeNo` is missing — never trusted on
	 * its own, because the buyer can edit the URL.
	 */
	initialStatus?: string;
	/**
	 * Polling cap. Default 12 attempts × 2000ms = ~24s. After the cap
	 * we issue one final `syncPayment(tradeNo)` (forces a gateway push)
	 * and resolve with whatever comes back.
	 */
	maxAttempts?: number;
	intervalMs?: number;
}

export interface UsePaymentResolutionResult {
	/** Terminal status, or "pending" while we're still polling. */
	outcome: PaymentOutcome;
	/** Human-readable failure reason, populated when `outcome === "failed"`. */
	reason?: string;
	/** True while polling is in flight. Flips false once `outcome` is terminal. */
	isPolling: boolean;
}

function httpStatus(error: unknown): number | undefined {
	if (!error || typeof error !== "object" || !("status" in error)) {
		return undefined;
	}
	const status = (error as { status?: unknown }).status;
	return typeof status === "number" ? status : undefined;
}

/**
 * Shared payment-status resolution for any PayUni redirect landing
 * page. Polls `/app/payments/{trade_no}` on an interval, falls back to
 * `/app/payments/{trade_no}/status_syncs` once the polling cap is hit,
 * and resolves with a `PaymentOutcome` the caller can route on.
 *
 * Used by:
 *  - /payment/return (legacy redirect landing, kept for backward compat)
 *  - /orders/{orderNo} (new PayUni Web redirect target — pending route)
 *
 * Important: this hook does NOT navigate. The caller decides whether
 * to redirect, render inline success, or pop a modal. Keeps the hook
 * reusable across surfaces with different UX shells.
 */
export function usePaymentResolution(
	options: UsePaymentResolutionOptions,
): UsePaymentResolutionResult {
	const {
		tradeNo,
		initialStatus,
		maxAttempts = 12,
		intervalMs = 2000,
	} = options;

	const [outcome, setOutcome] = React.useState<PaymentOutcome>("pending");
	const [reason, setReason] = React.useState<string | undefined>();

	React.useEffect(() => {
		let cancelled = false;
		let attempts = 0;
		let timer: ReturnType<typeof setTimeout> | null = null;

		const finish = (next: PaymentOutcome, because?: string) => {
			if (cancelled) return;
			setOutcome(next);
			setReason(because);
		};

		const poll = async (): Promise<void> => {
			// No trade_no = we can only act on the redirect's query string.
			// Treat a positive query hint as terminal-success ONLY if we
			// genuinely have no way to verify; this matches the previous
			// /payment/return behaviour but should be considered a soft
			// signal — pages that have a trade_no should always prefer it.
			if (!tradeNo) {
				const fromQuery = classifyPaymentStatus(initialStatus);
				if (fromQuery === "paid") {
					finish("paid");
				} else {
					finish("failed", "缺少交易編號");
				}
				return;
			}

			try {
				const response = await fetchPayment(tradeNo);
				if (cancelled) return;
				const next = classifyPaymentStatus(response.status);
				if (next === "paid") {
					finish("paid");
					return;
				}
				if (next === "failed") {
					finish("failed", response.status);
					return;
				}
			} catch (error) {
				const status = httpStatus(error);
				if (status === 401 || status === 403) {
					finish(
						"failed",
						"無法以目前登入身分確認這筆付款，請重新登入下單的買家帳號後到訂單頁確認。",
					);
					return;
				}
				if (status === 404) {
					finish("failed", "找不到付款交易，請稍後在訂單中確認。");
					return;
				}
				if (status && status >= 400 && status < 500) {
					finish("failed", "付款查詢失敗，請稍後在訂單中確認。");
					return;
				}
				// Transient network failure: swallow this attempt; server
				// may still settle on the next tick.
			}

			attempts += 1;
			if (attempts >= maxAttempts) {
				// One last forced gateway sync before we give up.
				try {
					const synced = await syncPayment(tradeNo);
					if (!cancelled) {
						const next = classifyPaymentStatus(synced.status);
						if (next === "paid") {
							finish("paid");
							return;
						}
						if (next === "failed") {
							finish("failed", synced.status);
							return;
						}
					}
				} catch {
					/* fall through */
				}
				finish("failed", "處理逾時，請稍後在訂單中確認");
				return;
			}

			timer = setTimeout(poll, intervalMs);
		};

		void poll();
		return () => {
			cancelled = true;
			if (timer) clearTimeout(timer);
		};
	}, [tradeNo, initialStatus, maxAttempts, intervalMs]);

	return {
		outcome,
		reason,
		isPolling: outcome === "pending",
	};
}
