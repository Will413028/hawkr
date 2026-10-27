"use client";

import * as React from "react";
import Button from "@/components/ui/Button";
import { fetchRefundPage } from "@/lib/after-sales/buyer-repo";
import {
	type AfterSaleRefundProgress,
	REFUND_TASK_STATUS,
} from "@/lib/after-sales/types";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import { safeRedirectUrl } from "@/lib/navigation/safe-url";

interface Props {
	/** Refund page is keyed by orderNo (`/app/orders/{orderNo}/refund_page`). */
	orderNo: string;
	progress?: AfterSaleRefundProgress;
}

/**
 * Buyer-facing refund-state banner:
 *   - refundTaskStatus MANUAL_INTERVENTION → info, no refill CTA (staff handling)
 *   - needRefundPage → warning, CTA bounces to the PayUni ATM refund page
 *   - refundFailureMessage → danger
 *   - bankDate (refund settled) → success
 * Renders nothing when there's no refund progress to surface.
 */
export function AfterSaleRefundBanner({ orderNo, progress }: Props) {
	const [busy, setBusy] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);

	if (!progress) return null;

	const handleFill = async () => {
		setBusy(true);
		setError(null);
		try {
			const { redirectUrl } = await fetchRefundPage(orderNo);
			const target = safeRedirectUrl(redirectUrl);
			if (target) {
				commitDocumentNavigation(target);
				return;
			}
			setError("暫時無法開啟退款頁，請稍後再試或聯絡客服。");
		} catch {
			setError("暫時無法開啟退款頁，請稍後再試或聯絡客服。");
		} finally {
			setBusy(false);
		}
	};

	// 對帳無法判定時退款任務轉為人工處理（refundTaskStatus="40"）。系統會由客服／
	// 對帳流程接手，買家不應被引導重填退款資訊，因此獨立於「退款失敗」處置，且不出
	// 現「重新填寫退款資訊」按鈕。此判斷須先於 refundFailureMessage（狀態 40 可能一併帶回訊息）。
	if (progress.refundTaskStatus === REFUND_TASK_STATUS.MANUAL_INTERVENTION) {
		return (
			<section
				className="rounded-hkui-lg border border-info-50 bg-info-50 p-4 text-sm text-info-700"
				role="status"
			>
				<strong className="block text-base">退款人工處理中</strong>
				<p className="m-0 mt-1 leading-relaxed">
					系統正在為你的退款進行人工處理，無需重新填寫退款資訊。如有疑問，請聯絡客服協助。
				</p>
			</section>
		);
	}

	if (progress.refundFailureMessage) {
		return (
			<section
				className="rounded-hkui-lg border border-danger-200 bg-danger-50 p-4 text-sm text-danger-700"
				role="alert"
			>
				<strong className="block text-base">退款失敗</strong>
				<p className="m-0 mt-1 mb-3 leading-relaxed">
					{progress.refundFailureMessage}
				</p>
				<Button
					color="danger"
					disabled={busy}
					isLoading={busy}
					loadingLabel="開啟退款頁中"
					onClick={handleFill}
				>
					重新填寫退款資訊
				</Button>
				{error ? (
					<p className="mt-2 text-xs text-danger-700 font-semibold">{error}</p>
				) : null}
			</section>
		);
	}

	if (progress.needRefundPage) {
		return (
			<section
				className="rounded-hkui-lg border border-warning-200 bg-warning-50 p-4 text-warning-700"
				role="status"
			>
				<strong className="block text-base">請填寫退款資訊</strong>
				<p className="m-0 mt-1 mb-3 text-sm leading-relaxed">
					你的付款方式需要提供銀行帳號才能完成退款。請填寫退款資訊，款項將於審核後撥入。
				</p>
				<Button
					color="warning"
					disabled={busy}
					isLoading={busy}
					loadingLabel="開啟退款頁中"
					onClick={handleFill}
				>
					填寫退款資訊
				</Button>
				{error ? (
					<p className="mt-2 text-xs text-danger-700 font-semibold">{error}</p>
				) : null}
			</section>
		);
	}

	if (progress.bankDate || progress.estimatedRefundDate) {
		const settled = Boolean(progress.bankDate);
		return (
			<section
				className={`rounded-hkui-lg border p-4 text-sm ${
					settled
						? "border-success-200 bg-success-50 text-success-700"
						: "border-info-50 bg-info-50 text-info-700"
				}`}
				role="status"
			>
				<strong className="block text-base">
					{settled ? "退款已完成" : "退款處理中"}
				</strong>
				<p className="m-0 mt-1 leading-relaxed">
					{settled
						? `退款已於 ${progress.bankDate} 撥出。`
						: `預計退款日期：${progress.estimatedRefundDate}。`}
				</p>
			</section>
		);
	}

	return null;
}

export default AfterSaleRefundBanner;
