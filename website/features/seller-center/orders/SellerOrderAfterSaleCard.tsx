"use client";

import Link from "@/components/shared/Link";
import { AfterSaleStatusBadge } from "@/features/after-sales/components/AfterSaleStatusBadge";
import {
	AFTER_SALE_APPEAL_NEXT_STEP,
	afterSaleIsUnderAppeal,
	afterSaleNeedsSeller,
} from "@/lib/after-sales/status";
import type { AfterSaleCase } from "@/lib/after-sales/types";
import { AFTER_SALE_STATUS } from "@/lib/after-sales/types";
import type { RefundReasonValue } from "@/lib/codemeta/enums";
import { REFUND_REASON_LABEL } from "@/lib/codemeta/enums";

function reasonLabel(raw: string): string {
	return REFUND_REASON_LABEL[raw as RefundReasonValue] ?? raw ?? "—";
}

function nextStep(status: string, appealState?: string): string {
	if (afterSaleIsUnderAppeal(appealState)) return AFTER_SALE_APPEAL_NEXT_STEP;
	switch (status) {
		case AFTER_SALE_STATUS.PENDING:
			return "待你處理售後申請";
		case AFTER_SALE_STATUS.RETURN_PENDING:
			return "等買家寄回商品";
		case AFTER_SALE_STATUS.RETURN_SHIPPING:
			return "退貨運送中";
		case AFTER_SALE_STATUS.RETURN_AT_STORE:
			return "退貨已到超商，等待物流取件";
		case AFTER_SALE_STATUS.RETURN_DELIVERED:
			return "退貨已送達，待你確認收貨";
		case AFTER_SALE_STATUS.RETURN_FAILED:
			return "退貨物流失敗，需客服協助重新安排";
		case AFTER_SALE_STATUS.RETURN_RECEIVED:
			return "待你確認收貨退款";
		case AFTER_SALE_STATUS.REFUNDING:
			return "退款處理中";
		case AFTER_SALE_STATUS.CLOSED:
			return "售後已結案";
		default:
			return "—";
	}
}

/**
 * Seller order-detail 售後案件 card. The actual processing happens on the
 * 售後處理 tab; this card surfaces the latest case + a deep link that opens
 * the case drawer there (?tab=aftersale&caseNo=…).
 */
export function SellerOrderAfterSaleCard({
	aftersale,
}: {
	aftersale: AfterSaleCase;
}) {
	const needsAction = afterSaleNeedsSeller(aftersale);
	const href = `/seller-center/orders?tab=aftersale&caseNo=${encodeURIComponent(
		aftersale.caseNo,
	)}`;
	return (
		<section className="seller-card">
			<header className="seller-card__head">
				<h2>售後案件</h2>
				<AfterSaleStatusBadge
					status={aftersale.status}
					appealState={aftersale.appealState}
					size="sm"
				/>
			</header>
			<dl className="seller-order-detail__info">
				<div>
					<dt>案件編號</dt>
					<dd>{aftersale.caseNo}</dd>
				</div>
				<div>
					<dt>原因</dt>
					<dd>{reasonLabel(aftersale.reason)}</dd>
				</div>
				<div>
					<dt>可退金額</dt>
					<dd>
						NT${" "}
						{Number(
							aftersale.refundAmount || aftersale.refundableAmount || 0,
						).toLocaleString()}
					</dd>
				</div>
				<div>
					<dt>下一步</dt>
					<dd>{nextStep(aftersale.status, aftersale.appealState)}</dd>
				</div>
			</dl>
			<Link
				href={href}
				// `.seller-orders__action` 的白字（0-1-0）會被 dark 的
				// `body a:not([data-hkui-button])`（0-2-2）換成 `--color-ink-200`。
				data-hkui-button=""
				className="seller-orders__action seller-orders__action--primary mt-2"
			>
				{needsAction ? "前往處理售後" : "前往售後處理"}
			</Link>
		</section>
	);
}

export default SellerOrderAfterSaleCard;
