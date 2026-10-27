"use client";

import { useSearchParams } from "next/navigation";
import * as React from "react";
import Button from "@/components/ui/Button";
import { clearPendingPurchase } from "@/lib/analytics/purchase-store";

export default function CheckoutFailClient() {
	return (
		<React.Suspense fallback={null}>
			<Inner />
		</React.Suspense>
	);
}

const COMMON_REASONS = [
	"信用卡號、有效期或安全碼有誤",
	"信用卡額度不足或交易被銀行拒絕",
	"PayUni 付款頁逾時關閉",
	"網路中斷導致回傳失敗",
];

function Inner() {
	const params = useSearchParams();
	const orderNo = params?.get("order") || "";
	const reason = params?.get("reason") || "";

	// 這一輪結帳失敗，confirm 當下寫下的 pending orderNos 不會有人消費了——
	// 只有 /checkout/success 的 PurchaseTracker 讀它。留著的話它會活到整個
	// tab session 結束，之後手動開一個舊的 success URL 就會補送一筆從未付款
	// 成功的 GA4 purchase。
	React.useEffect(() => {
		clearPendingPurchase();
	}, []);

	return (
		<main className="min-h-[calc(100vh-200px)] max-w-[580px] mx-auto py-10 px-5 pb-12 flex items-center justify-center">
			<div className="relative w-full bg-content1/90 border border-divider rounded-3xl p-10 px-7 pb-8 text-center flex flex-col items-center gap-3 shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_18px_40px_-22px_rgba(94,46,46,0.28),0_6px_18px_-12px_rgba(33,15,15,0.18)] backdrop-blur-md overflow-hidden dark:bg-content1/86 animate-checkout-pop">
				<span
					className="relative z-1 inline-flex items-center justify-center w-[148px] h-[148px] -mt-1.5 mb-[-4px] animate-checkout-icon-shake [&_svg]:w-16 [&_svg]:h-16 [&_svg]:stroke-danger [&_svg]:stroke-[3.5] [&_svg]:stroke-linecap-round [&_svg]:fill-none"
					aria-hidden="true"
				>
					<svg viewBox="0 0 24 24" aria-hidden="true">
						<line x1="7" y1="7" x2="17" y2="17" />
						<line x1="17" y1="7" x2="7" y2="17" />
					</svg>
				</span>
				<h1 className="relative z-1 font-display text-xl md:text-2xl m-0 text-foreground">
					付款未完成
				</h1>
				<p className="relative z-1 m-0 text-foreground-soft text-sm max-w-[38ch]">
					你的訂單還沒成立，付款流程被中斷或銀行端拒絕。你可以回到結帳頁重新嘗試。
				</p>
				{(orderNo || reason) && (
					<div className="relative z-1 w-full max-w-[360px] mt-3 p-4 rounded-2xl bg-content2 border border-divider text-left flex flex-col gap-3">
						{reason && (
							<div className="flex justify-between items-center gap-3">
								<span className="text-sm font-medium text-foreground-muted">
									系統訊息
								</span>
								<span className="text-sm font-bold text-foreground flex items-center gap-2">
									{reason}
								</span>
							</div>
						)}
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
					</div>
				)}
				<div className="relative z-1 w-full max-w-[360px] mt-3 text-left">
					<h2 className="text-xs font-extrabold text-foreground mb-2">
						常見原因
					</h2>
					<ul className="list-none m-0 p-0 flex flex-col gap-1">
						{COMMON_REASONS.map((r) => (
							<li
								className="text-xs font-bold text-foreground-soft flex items-center gap-2 before:content-[''] before:w-1 before:h-1 before:rounded-full before:bg-danger before:shrink-0"
								key={r}
							>
								{r}
							</li>
						))}
					</ul>
				</div>
				<div className="relative z-1 flex gap-2 flex-wrap justify-center mt-4 w-full">
					<Button href="/checkout">重新結帳</Button>
					<Button href="/cart" variant="bordered" color="default">
						回購物車
					</Button>
				</div>
			</div>
		</main>
	);
}
