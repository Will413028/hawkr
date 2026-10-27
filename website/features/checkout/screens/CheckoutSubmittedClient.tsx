"use client";

import { useSearchParams } from "next/navigation";
import * as React from "react";
import Button from "@/components/ui/Button";
import { CheckoutInvoiceRows } from "@/features/invoices/checkout/CheckoutInvoiceRows";
import { useOrder } from "@/lib/hooks/useOrders";

export default function CheckoutSubmittedClient() {
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

	return (
		<main className="min-h-[calc(100vh-200px)] max-w-[580px] mx-auto py-10 px-5 pb-12 flex items-center justify-center">
			<div className="relative w-full bg-content1/90 border border-divider rounded-3xl p-10 px-7 pb-8 text-center flex flex-col items-center gap-3 shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_18px_40px_-22px_rgba(46,70,94,0.24),0_6px_18px_-12px_rgba(15,24,33,0.16)] backdrop-blur-md overflow-hidden dark:bg-content1/86 animate-checkout-pop">
				<span
					className="relative z-1 inline-flex items-center justify-center w-[148px] h-[148px] -mt-1.5 mb-[-4px] animate-checkout-icon-pop [&_svg]:w-16 [&_svg]:h-16 [&_svg]:stroke-warning [&_svg]:stroke-[2] [&_svg]:stroke-linecap-round [&_svg]:stroke-linejoin-round [&_svg]:fill-none"
					aria-hidden="true"
				>
					<svg viewBox="0 0 24 24" aria-hidden="true">
						<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
						<polyline points="14 2 14 8 20 8" />
						<line x1="9" y1="13" x2="15" y2="13" />
						<line x1="9" y1="17" x2="13" y2="17" />
					</svg>
				</span>
				<h1 className="relative z-1 font-display text-xl md:text-2xl m-0 text-foreground">
					訂單已送出
				</h1>
				<p className="relative z-1 m-0 text-foreground-soft text-sm max-w-[38ch]">
					你選擇 ATM
					轉帳，請至訂單頁查看虛擬帳號、銀行代碼與繳費期限，於期限內完成轉帳即可。
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
							<CheckoutInvoiceRows
								orderNo={orderNo}
								variant="awaiting-payment"
							/>
						)}
					</div>
				)}
				<div className="relative z-1 flex gap-2 flex-wrap justify-center mt-4 w-full">
					{orderNo && (
						<Button href={`/account/orders/${orderNo}`}>
							前往訂單頁查看付款資訊
						</Button>
					)}
					<Button href="/account/orders" variant="bordered" color="default">
						我的訂單
					</Button>
				</div>
			</div>
		</main>
	);
}
