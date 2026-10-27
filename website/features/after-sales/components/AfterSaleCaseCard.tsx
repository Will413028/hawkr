import type * as React from "react";
import Link from "@/components/shared/Link";
import { AfterSaleStatusBadge } from "@/features/after-sales/components/AfterSaleStatusBadge";
import type { AfterSaleCase } from "@/lib/after-sales/types";
import { AFTER_SALE_STATUS } from "@/lib/after-sales/types";
import type { RefundReasonValue } from "@/lib/codemeta/enums";
import { REFUND_REASON_LABEL } from "@/lib/codemeta/enums";
import { formatTaipeiDate, formatTaipeiDateTime } from "@/lib/date/taipei";

function reasonLabel(reason: string): string {
	return REFUND_REASON_LABEL[reason as RefundReasonValue] ?? reason ?? "—";
}

function formatDate(raw: string): string {
	// 解不出來就把原字串照登（沿用原行為）。
	return formatTaipeiDate(raw) || raw;
}

function formatDateTime(raw: string): string {
	return formatTaipeiDateTime(raw) || raw;
}

interface Props {
	aftersale: AfterSaleCase;
	/** Wraps the whole card in a link (buyer list). */
	href?: string;
	/** Footer actions (seller mobile cards put CTAs here). */
	footer?: React.ReactNode;
	className?: string;
}

/** Shared after-sale case card — buyer history list + seller mobile cards. */
export function AfterSaleCaseCard({
	aftersale,
	href,
	footer,
	className = "",
}: Props) {
	const refundText = Number(
		aftersale.refundAmount || aftersale.refundableAmount || 0,
	).toLocaleString();

	const body = (
		<>
			<div className="flex items-start justify-between gap-3">
				<div className="min-w-0">
					<AfterSaleStatusBadge
						status={aftersale.status}
						appealState={aftersale.appealState}
						size="sm"
					/>
					<p className="mt-1.5 text-sm font-semibold text-foreground break-all">
						{aftersale.caseNo}
					</p>
				</div>
				<div className="text-right shrink-0">
					<p className="text-base font-bold text-hk-primary tabular-nums">
						NT$ {refundText}
					</p>
					{aftersale.createdAt ? (
						<p className="text-xs text-foreground-soft mt-0.5">
							{formatDate(aftersale.createdAt)}
						</p>
					) : null}
				</div>
			</div>
			<dl className="mt-2 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-xs">
				<dt className="text-foreground-soft">原因</dt>
				<dd className="m-0 text-hk-body truncate">
					{reasonLabel(aftersale.reason)}
				</dd>
				<dt className="text-foreground-soft">訂單</dt>
				<dd className="m-0 text-hk-body break-all">#{aftersale.orderNo}</dd>
				{aftersale.buyerName ? (
					<>
						<dt className="text-foreground-soft">買家</dt>
						<dd className="m-0 text-hk-body truncate">{aftersale.buyerName}</dd>
					</>
				) : aftersale.sellerName ? (
					<>
						<dt className="text-foreground-soft">賣家</dt>
						<dd className="m-0 text-hk-body truncate">
							{aftersale.sellerName}
						</dd>
					</>
				) : null}
				{aftersale.status === AFTER_SALE_STATUS.RETURN_PENDING &&
				aftersale.shipBackDeadlineAt ? (
					<>
						<dt className="text-foreground-soft">寄回期限</dt>
						<dd className="m-0 text-hk-body">
							{formatDateTime(aftersale.shipBackDeadlineAt)}
						</dd>
					</>
				) : null}
			</dl>
		</>
	);

	const base = `bg-content1 border border-divider rounded-hkui-lg p-4 ${className}`;

	if (href) {
		return (
			<Link
				href={href}
				className={`block text-inherit no-underline transition-colors duration-200 hover:border-hkui-primary-200 ${base}`}
			>
				{body}
				{footer}
			</Link>
		);
	}

	return (
		<div className={base}>
			{body}
			{footer ? <div className="mt-3">{footer}</div> : null}
		</div>
	);
}

export default AfterSaleCaseCard;
