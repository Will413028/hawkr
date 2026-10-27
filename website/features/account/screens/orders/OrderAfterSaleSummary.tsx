"use client";

import Button from "@/components/ui/Button";
import { AfterSaleProgress } from "@/features/after-sales/components/AfterSaleProgress";
import { AfterSaleStatusBadge } from "@/features/after-sales/components/AfterSaleStatusBadge";
import type { RefundReasonValue } from "@/lib/codemeta/enums";
import { REFUND_REASON_LABEL } from "@/lib/codemeta/enums";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useMyAfterSales } from "@/lib/hooks/useMyAfterSales";

function reasonLabel(reason: string): string {
	return REFUND_REASON_LABEL[reason as RefundReasonValue] ?? reason ?? "—";
}

function formatDate(raw: string): string {
	// 解不出來就把原字串照登（沿用原行為）。
	return formatTaipeiDate(raw) || raw;
}

/**
 * Buyer order-detail "售後案件" block. Owns the after-sale CTA:
 *   - case exists → summary + 查看售後案件
 *   - no case + canRefund → hint + 申請退換貨
 *   - otherwise → nothing
 *
 * When `hideApplyCta` is set (e.g. the 已送達 action card already shows
 * 申請退換貨), the "no case + canRefund" apply block is suppressed so the
 * button isn't rendered twice; the existing-case summary still shows.
 */
export function OrderAfterSaleSummary({
	orderNo,
	canRefund,
	hideApplyCta = false,
}: {
	orderNo: string;
	canRefund: boolean;
	hideApplyCta?: boolean;
}) {
	const { lookup, loading } = useMyAfterSales();
	const aftersale = lookup.get(orderNo);

	// Nothing to show and can't apply → render nothing (no empty noise).
	if (!aftersale && !canRefund) return null;
	// Still loading the list and there's no reason to apply yet — wait.
	if (!aftersale && loading) return null;

	if (!aftersale) {
		// Apply CTA owned elsewhere (delivered action card) → stay silent.
		if (hideApplyCta) return null;
		return (
			<section className="bg-content1 border border-divider rounded-hkui-lg p-5">
				<h2 className="text-base font-semibold mb-2">售後案件</h2>
				<p className="text-sm text-foreground-soft mb-4">
					收到的商品有瑕疵、寄錯或與描述不符？可在審核期限內申請退換貨。
				</p>
				<Button
					href={`/account/orders/${orderNo}/return`}
					variant="bordered"
					color="warning"
				>
					申請退換貨
				</Button>
			</section>
		);
	}

	const refundText = Number(
		aftersale.refundAmount || aftersale.refundableAmount || 0,
	).toLocaleString();

	return (
		<section className="bg-content1 border border-divider rounded-hkui-lg p-5">
			<header className="flex items-center justify-between gap-3 mb-3">
				<h2 className="text-base font-semibold">售後案件</h2>
				<AfterSaleStatusBadge
					status={aftersale.status}
					appealState={aftersale.appealState}
				/>
			</header>
			<dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-sm mb-4">
				<dt className="text-hk-muted">案件編號</dt>
				<dd className="m-0 text-hk-body font-medium break-all">
					{aftersale.caseNo}
				</dd>
				<dt className="text-hk-muted">申請原因</dt>
				<dd className="m-0 text-hk-body">{reasonLabel(aftersale.reason)}</dd>
				{aftersale.createdAt ? (
					<>
						<dt className="text-hk-muted">申請時間</dt>
						<dd className="m-0 text-hk-body">
							{formatDate(aftersale.createdAt)}
						</dd>
					</>
				) : null}
				<dt className="text-hk-muted">退款金額</dt>
				<dd className="m-0 text-hk-primary font-semibold">NT$ {refundText}</dd>
			</dl>
			<AfterSaleProgress
				status={aftersale.status}
				size="compact"
				className="mb-4"
			/>
			<div className="flex justify-end">
				<Button href={`/account/after-sales/${aftersale.caseNo}`}>
					查看售後案件
				</Button>
			</div>
		</section>
	);
}

export default OrderAfterSaleSummary;
