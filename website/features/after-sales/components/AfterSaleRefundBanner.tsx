import {
	type AfterSaleRefundProgress,
	REFUND_TASK_STATUS,
} from "@/lib/after-sales/types";

interface Props {
	progress?: AfterSaleRefundProgress;
}

/**
 * Buyer-facing refund-state banner:
 *   - refundTaskStatus MANUAL_INTERVENTION → info (staff handling)
 *   - refundFailureMessage → danger
 * Renders nothing when there's no refund progress to surface.
 */
export function AfterSaleRefundBanner({ progress }: Props) {
	if (!progress) return null;

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
				<p className="m-0 mt-1 leading-relaxed">
					{progress.refundFailureMessage}
				</p>
			</section>
		);
	}

	return null;
}

export default AfterSaleRefundBanner;
