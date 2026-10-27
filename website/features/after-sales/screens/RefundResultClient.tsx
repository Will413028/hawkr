"use client";

import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { AfterSaleRefundBanner } from "@/features/after-sales/components/AfterSaleRefundBanner";
import {
	fetchMyAfterSaleDetail,
	fetchMyAfterSales,
} from "@/lib/after-sales/buyer-repo";
import type { AfterSaleRefundProgress } from "@/lib/after-sales/types";
import { AFTER_SALE_STATUS, REFUND_TASK_STATUS } from "@/lib/after-sales/types";
import { useAuth } from "@/lib/auth/context";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { queryKeys } from "@/lib/query/keys";

/**
 * Web landing for PayUni's ATM-refund return. Visual shell mirrors
 * `CheckoutSuccessClient` (glass card + tone gradient + 148px hero + order box
 * + button row) so the post-PayUni experience is consistent across flows.
 *
 * The gateway 302s the buyer here (`{WEB_FRONTEND_URL}/orders/{orderNo}/refund`)
 * only when refund_page was opened with `clientType: "WEB_BROWSER"`. The query
 * carries `status` / `trade_no` / `estimated_refund_date` / `code`, but per
 * design doc 09 §5.4.2 those are UI hints ONLY — we re-confirm the real refund
 * state from the after-sale case detail (the SoT) before showing anything.
 */

type Outcome =
	| "settled"
	| "processing"
	| "action"
	| "manual"
	| "failed"
	| "unknown";

/** True outcome from the real refundProgress; the PayUni query is a fallback. */
function classify(
	progress: AfterSaleRefundProgress | undefined,
	hint: string,
): Outcome {
	if (progress) {
		const task = progress.refundTaskStatus;
		if (progress.bankDate || task === REFUND_TASK_STATUS.DONE) return "settled";
		// MANUAL_INTERVENTION ("40"): 對帳無法判定，系統正人工處理中。買家不應被
		// 引導重填退款資訊，因此獨立於 failed（即使 refundFailureMessage 一併帶回）。
		if (task === REFUND_TASK_STATUS.MANUAL_INTERVENTION) return "manual";
		if (progress.refundFailureMessage || task === REFUND_TASK_STATUS.FAILED)
			return "failed";
		if (
			progress.needRefundPage ||
			task === REFUND_TASK_STATUS.AWAITING_BUYER_REFUND_PAGE
		)
			return "action";
		if (progress.estimatedRefundDate || task) return "processing";
	}
	const h = hint.toLowerCase();
	if (h === "success") return "processing"; // submitted; real settle lags
	if (h === "failed" || h === "failure") return "failed";
	return "unknown";
}

interface Resolved {
	caseNo: string;
	progress?: AfterSaleRefundProgress;
}

/** Find the after-sale case for this order (prefer the one in refund), then
 * read its detail for the authoritative refundProgress. */
async function resolveForOrder(orderNo: string): Promise<Resolved | null> {
	const { records } = await fetchMyAfterSales({ pageSize: 50 });
	const mine = records.filter((c) => c.orderNo === orderNo);
	if (mine.length === 0) return null;
	const picked =
		mine.find((c) => c.status === AFTER_SALE_STATUS.REFUNDING) ??
		[...mine].sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1))[0];
	const detail = await fetchMyAfterSaleDetail(picked.caseNo);
	return {
		caseNo: detail.caseNo || picked.caseNo,
		progress: detail.refundProgress,
	};
}

/** Per-outcome glass-card chrome. Class strings are literal so Tailwind v4
 * statically picks up the shadow. */
const CHROME: Record<
	Outcome,
	{ shadow: string; iconBg: string; title: string }
> = {
	settled: {
		shadow:
			"shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_18px_40px_-22px_rgb(var(--color-ink-700-rgb)/0.28),0_6px_18px_-12px_rgb(var(--color-ink-950-rgb)/0.18)]",
		iconBg: "text-success-700",
		title: "退款已完成",
	},
	processing: {
		shadow:
			"shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_18px_40px_-22px_rgba(46,70,94,0.24),0_6px_18px_-12px_rgba(15,24,33,0.16)]",
		iconBg: "text-info-700",
		title: "退款資訊已送出",
	},
	action: {
		shadow:
			"shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_18px_40px_-22px_rgba(94,78,46,0.26),0_6px_18px_-12px_rgba(33,27,15,0.16)]",
		iconBg: "text-warning-700",
		title: "尚需補填退款資訊",
	},
	manual: {
		shadow:
			"shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_18px_40px_-22px_rgba(46,70,94,0.24),0_6px_18px_-12px_rgba(15,24,33,0.16)]",
		iconBg: "text-info-700",
		title: "退款人工處理中",
	},
	failed: {
		shadow:
			"shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_18px_40px_-22px_rgba(94,46,46,0.26),0_6px_18px_-12px_rgba(33,15,15,0.16)]",
		iconBg: "text-danger-700",
		title: "退款未完成",
	},
	unknown: {
		shadow:
			"shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_18px_40px_-22px_rgba(46,70,94,0.24),0_6px_18px_-12px_rgba(15,24,33,0.16)]",
		iconBg: "text-foreground",
		title: "退款處理中",
	},
};

export default function RefundResultClient({ orderNo }: { orderNo: string }) {
	return (
		<React.Suspense fallback={null}>
			<Inner orderNo={orderNo} />
		</React.Suspense>
	);
}

function Inner({ orderNo }: { orderNo: string }) {
	const params = useSearchParams();
	const hint = params?.get("status") ?? "";

	const { hydrated } = useAuth();
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.buyer.refundResolve(identity, orderNo),
		queryFn: () => resolveForOrder(orderNo),
		enabled: hydrated && Boolean(orderNo),
	});
	const resolved = query.data ?? null;
	// Stay in the loading shell until auth hydrates and the first resolve
	// settles, so we never flash a resolved outcome on an unauthenticated read.
	const loading = !hydrated || query.isLoading;
	const [copied, setCopied] = React.useState(false);

	const copyOrderNo = async () => {
		try {
			await navigator.clipboard.writeText(orderNo);
			setCopied(true);
			window.setTimeout(() => setCopied(false), 1600);
		} catch {
			// clipboard blocked — user can long-press to copy
		}
	};

	const progress = resolved?.progress;
	const outcome: Outcome = classify(progress, hint);
	const chrome = CHROME[outcome];
	const caseHref = resolved?.caseNo
		? `/account/after-sales/${encodeURIComponent(resolved.caseNo)}`
		: "/account/after-sales";

	const desc = (() => {
		switch (outcome) {
			case "settled":
				return progress?.bankDate
					? `退款已於 ${progress.bankDate} 撥出，將匯入你提供的帳戶。`
					: "款項已撥出，將匯入你提供的帳戶。";
			case "processing":
				return progress?.estimatedRefundDate
					? `我們已收到你的退款資訊，預計 ${progress.estimatedRefundDate} 入帳。`
					: "我們已收到你的退款資訊，款項將於審核後撥入。";
			case "action":
				return "你的付款方式需要提供銀行帳號才能完成退款，請於下方補填。";
			case "manual":
				return "系統正在為你的退款進行人工處理，無需重新填寫退款資訊。如有疑問，請聯絡客服協助。";
			case "failed":
				return "退款尚未完成，請依下方提示重新填寫退款資訊，或聯絡客服協助。";
			default:
				return "我們正在確認退款狀態，可至售後進度查看最新結果。";
		}
	})();

	// Loading shares the exact shell with a neutral info gradient.
	const c = loading ? CHROME.unknown : chrome;

	return (
		<main className="min-h-[calc(100vh-200px)] max-w-[580px] mx-auto py-10 px-5 pb-12 flex items-center justify-center">
			<div
				className={`relative w-full bg-content1/90 border border-divider rounded-3xl p-10 px-7 pb-8 text-center flex flex-col items-center gap-3 backdrop-blur-md overflow-hidden dark:bg-content1/86 animate-checkout-pop ${c.shadow}`}
			>
				<div
					className="relative z-1 w-[148px] h-[148px] -mt-1.5 mb-[-4px] flex items-center justify-center"
					aria-hidden="true"
				>
					{loading ? (
						<span className="inline-flex gap-2" role="status">
							<span className="inline-block w-2.5 h-2.5 rounded-full bg-info animate-checkout-bounce" />
							<span className="inline-block w-2.5 h-2.5 rounded-full bg-info animate-checkout-bounce [animation-delay:150ms]" />
							<span className="inline-block w-2.5 h-2.5 rounded-full bg-info animate-checkout-bounce [animation-delay:300ms]" />
						</span>
					) : outcome === "settled" ? (
						<AppIcon
							name="check"
							size={148}
							className="text-[var(--hkui-success)]"
						/>
					) : (
						<span
							className={`inline-flex items-center justify-center w-[88px] h-[88px] rounded-full bg-content2 ${chrome.iconBg} animate-checkout-icon-pop`}
						>
							<svg
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								strokeWidth="2"
								className="w-11 h-11"
								aria-hidden="true"
							>
								{outcome === "processing" || outcome === "manual" ? (
									<>
										<circle cx="12" cy="12" r="9" />
										<path
											d="M12 7v5l3 2"
											strokeLinecap="round"
											strokeLinejoin="round"
										/>
									</>
								) : (
									<>
										<circle cx="12" cy="12" r="9" />
										<path d="M12 8v5" strokeLinecap="round" />
										<path d="M12 16h.01" strokeLinecap="round" />
									</>
								)}
							</svg>
						</span>
					)}
				</div>

				<h1 className="relative z-1 font-display text-xl md:text-2xl m-0 text-foreground">
					{loading ? "確認退款結果中" : c.title}
				</h1>
				<p className="relative z-1 m-0 text-foreground-soft text-sm max-w-[38ch]">
					{loading ? "正在向系統確認最新退款狀態，請稍候。" : desc}
				</p>

				{!loading && (
					<div className="relative z-1 w-full max-w-[360px] mt-3 p-4 rounded-2xl bg-content2 border border-divider text-left flex flex-col gap-3">
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
					</div>
				)}

				{!loading &&
				(outcome === "action" || outcome === "failed") &&
				progress ? (
					<div className="relative z-1 w-full max-w-[360px] mt-1 text-left">
						<AfterSaleRefundBanner orderNo={orderNo} progress={progress} />
					</div>
				) : null}

				{!loading && (
					<div className="relative z-1 flex gap-2 flex-wrap justify-center mt-4 w-full">
						<Button href={caseHref}>查看售後進度</Button>
						<Button
							href={`/account/orders/${encodeURIComponent(orderNo)}`}
							variant="bordered"
							color="default"
						>
							查看訂單
						</Button>
					</div>
				)}
			</div>
		</main>
	);
}
