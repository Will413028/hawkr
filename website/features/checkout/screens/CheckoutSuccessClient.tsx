"use client";

import { useSearchParams } from "next/navigation";
import * as React from "react";
import PurchaseTracker from "@/components/analytics/PurchaseTracker";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { CheckoutInvoiceRows } from "@/features/invoices/checkout/CheckoutInvoiceRows";
import { useOrder } from "@/lib/hooks/useOrders";

export default function CheckoutSuccessClient() {
	return (
		<React.Suspense fallback={null}>
			<Inner />
		</React.Suspense>
	);
}

function Inner() {
	const params = useSearchParams();
	const orderNo = params?.get("order") || "";
	const { order } = useOrder(orderNo || undefined);
	const [copied, setCopied] = React.useState(false);
	const copyOrderNo = async () => {
		if (!orderNo) return;
		try {
			await navigator.clipboard.writeText(orderNo);
			setCopied(true);
			window.setTimeout(() => setCopied(false), 1600);
		} catch {
			// clipboard blocked — silently ignore, user can long-press to copy
		}
	};

	return (
		<main className="min-h-[calc(100vh-200px)] max-w-[580px] mx-auto py-10 px-5 pb-12 flex items-center justify-center">
			<PurchaseTracker fallbackOrderNo={orderNo || undefined} />
			<div className="relative w-full bg-content1/90 border border-divider rounded-3xl p-10 px-7 pb-8 text-center flex flex-col items-center gap-3 shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_18px_40px_-22px_rgb(var(--color-ink-700-rgb)/0.28),0_6px_18px_-12px_rgb(var(--color-ink-950-rgb)/0.18)] backdrop-blur-md overflow-hidden dark:bg-content1/86 animate-checkout-pop">
				<div
					className="w-[148px] h-[148px] -mt-1.5 mb-[-4px] flex items-center justify-center relative z-1"
					aria-hidden="true"
				>
					<AppIcon
						name="check"
						size={148}
						className="text-[var(--hkui-success)]"
					/>
				</div>
				<h1 className="relative z-1 font-display text-xl md:text-2xl m-0 text-foreground">
					訂單已成立
				</h1>
				<p className="relative z-1 m-0 text-foreground-soft text-sm max-w-[38ch]">
					感謝你的購買！我們已通知商家準備出貨，後續資訊於訂單頁面更新通知你。
				</p>
				{(orderNo || order) && (
					<div className="relative z-1 w-full max-w-[360px] mt-3 p-4 rounded-2xl bg-content2 border border-divider text-left flex flex-col gap-3">
						{orderNo && (
							<div className="flex justify-between items-center gap-3">
								<span className="text-sm font-medium text-foreground-muted">
									訂單編號
								</span>
								<span className="text-sm font-bold text-foreground flex items-center gap-2">
									{orderNo}
									<button
										type="button"
										className="text-xs font-bold px-2 py-0.5 rounded-full border border-divider bg-content1 text-hk-primary-strong cursor-pointer transition-colors duration-150 hover:border-hk-primary-strong hover:bg-hk-primary-strong hover:text-hk-cream"
										onClick={copyOrderNo}
										data-copied={copied ? "true" : undefined}
										aria-label="複製訂單編號"
									>
										{copied ? "已複製" : "複製"}
									</button>
								</span>
							</div>
						)}
						{order && (
							<div
								className={`flex justify-between items-baseline gap-3${orderNo ? " border-t border-divider pt-3" : ""}`}
							>
								<span className="text-sm font-bold text-foreground">
									應付金額
								</span>
								<span className="text-xl font-black text-hk-accent">
									NT$ {Number(order.totalActualAmount || 0).toLocaleString()}
								</span>
							</div>
						)}
						{orderNo && (
							<CheckoutInvoiceRows orderNo={orderNo} variant="paid" />
						)}
					</div>
				)}
				<div className="relative z-1 flex gap-2 flex-wrap justify-center mt-4 w-full">
					{orderNo && (
						<Button href={`/account/orders/${orderNo}`}>查看訂單</Button>
					)}
					<Button href="/market/products" variant="bordered" color="default">
						繼續購物
					</Button>
				</div>
			</div>
		</main>
	);
}
