"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { AfterSaleDecisionDrawer } from "@/features/after-sales/components/AfterSaleDecisionDrawer";
import {
	AFTER_SALE_APPEAL_NEXT_STEP,
	AFTER_SALE_STATUS,
	type AfterSaleCase,
	type AfterSaleGroup,
	afterSaleDisplayLabel,
	afterSaleGroupOf,
	afterSaleIsTerminal,
	afterSaleIsUnderAppeal,
} from "@/lib/after-sales";
import {
	REFUND_REASON_LABEL,
	type RefundReasonValue,
} from "@/lib/codemeta/enums";
import { SellerEmptyState } from "../components/SellerEmptyState";
import { SellerTableSkeletonRows } from "../components/SellerSkeletons";

/**
 * 售後處理面板 — the 異常處理 tab's after-sale sub-view (sibling of
 * `AnomalyPanel`). Lists the seller's after-sale (退換貨) cases and lets the
 * seller process them via the case drawer (resolve / confirm-receipt).
 *
 * Data (`useSellerAfterSales`) lives in `SellerOrdersPage` and is passed in;
 * the page owns toolbar search while this panel owns the case drawer.
 * `onRefresh` re-pulls the list after a decision so statuses update in place.
 */

const GROUP_OPTIONS: ReadonlyArray<{ value: AfterSaleGroup; label: string }> = [
	{ value: "all", label: "全部" },
	{ value: "pending", label: "待處理" },
	{ value: "returning", label: "退貨中" },
	{ value: "refunding", label: "退款中" },
];

/** Map a case status to the shared order-status badge tone. Folds appeal
 * state: a case in platform arbitration shows the warning tone regardless of
 * its raw status (已拒絕 / 待處理). */
function statusBadgeClass(status: string, appealState?: string): string {
	if (afterSaleIsUnderAppeal(appealState)) {
		return "seller-orders__status seller-orders__status--warning";
	}
	switch (status) {
		case "0":
		case "30":
			return "seller-orders__status seller-orders__status--warning";
		case "20":
		case "25":
		case "26":
		case "27":
			return "seller-orders__status seller-orders__status--shipping";
		case "28":
			return "seller-orders__status seller-orders__status--alert";
		case "40":
			return "seller-orders__status seller-orders__status--pending";
		case "50":
			return "seller-orders__status seller-orders__status--complete";
		default:
			return "seller-orders__status seller-orders__status--muted";
	}
}

function formatMoney(raw: string): string {
	const n = Number(raw || 0);
	if (Number.isNaN(n)) return "—";
	return `NT$ ${n.toLocaleString()}`;
}

function formatReason(raw: string): string {
	if (!raw) return "—";
	return REFUND_REASON_LABEL[raw as RefundReasonValue] ?? raw;
}

/** Short "下一步" hint per status — answers "現在輪到誰做什麼". Under platform
 * arbitration the seller can't act, so it reads「等待平台裁決」. */
function nextStepText(status: string, appealState?: string): string {
	if (afterSaleIsUnderAppeal(appealState)) return AFTER_SALE_APPEAL_NEXT_STEP;
	switch (status) {
		case AFTER_SALE_STATUS.PENDING:
			return "待你處理";
		case AFTER_SALE_STATUS.RETURN_PENDING:
			return "等買家寄回";
		case AFTER_SALE_STATUS.RETURN_SHIPPING:
			return "退貨運送中";
		case AFTER_SALE_STATUS.RETURN_AT_STORE:
			return "超商待取";
		case AFTER_SALE_STATUS.RETURN_DELIVERED:
		case AFTER_SALE_STATUS.RETURN_RECEIVED:
			return "待你確認收貨";
		case AFTER_SALE_STATUS.RETURN_FAILED:
			return "退貨失敗";
		case AFTER_SALE_STATUS.REFUNDING:
			return "退款處理中";
		case AFTER_SALE_STATUS.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT:
			return "買家逾時未寄回，案件已關閉";
		default:
			return "—";
	}
}

/** Primary action label per status (plan §操作欄). Under platform arbitration
 * the seller can only review, so it reads「查看詳情」. */
function actionLabel(status: string, appealState?: string): string {
	if (afterSaleIsUnderAppeal(appealState)) return "查看詳情";
	switch (status) {
		case AFTER_SALE_STATUS.PENDING:
			return "處理售後";
		case AFTER_SALE_STATUS.RETURN_PENDING:
		case AFTER_SALE_STATUS.RETURN_SHIPPING:
		case AFTER_SALE_STATUS.RETURN_AT_STORE:
		case AFTER_SALE_STATUS.RETURN_FAILED:
			return "查看退貨";
		case AFTER_SALE_STATUS.RETURN_DELIVERED:
		case AFTER_SALE_STATUS.RETURN_RECEIVED:
			return "確認收貨";
		case AFTER_SALE_STATUS.REFUNDING:
			return "退款處理中";
		default:
			return "查看詳情";
	}
}

function IconTune() {
	return (
		<AppIcon name="filter" className="seller-orders__quick-filters-icon" />
	);
}

interface AfterSalePanelProps {
	records: ReadonlyArray<AfterSaleCase>;
	loading: boolean;
	showClosed: boolean;
	group: AfterSaleGroup;
	search: string;
	onGroupChange: (value: AfterSaleGroup) => void;
	/** Re-pull the seller after-sale list after a decision. */
	onRefresh: () => void | Promise<void>;
	/** Deep-link: auto-open the drawer for this case (from order detail). */
	initialCaseNo?: string;
	/** Clear the one-shot `caseNo` query after the drawer opens. */
	onInitialCaseConsumed?: () => void;
}

export function AfterSalePanel({
	records,
	loading,
	showClosed,
	group,
	search,
	onGroupChange,
	onRefresh,
	initialCaseNo,
	onInitialCaseConsumed,
}: AfterSalePanelProps) {
	const [activeCase, setActiveCase] = React.useState<AfterSaleCase | null>(
		null,
	);

	// Auto-open the drawer when arrived via ?caseNo= deep-link.
	const consumedRef = React.useRef<string | null>(null);
	React.useEffect(() => {
		if (!initialCaseNo || consumedRef.current === initialCaseNo) return;
		const found = records.find((c) => c.caseNo === initialCaseNo);
		if (found) {
			consumedRef.current = initialCaseNo;
			setActiveCase(found);
			onInitialCaseConsumed?.();
		}
	}, [initialCaseNo, onInitialCaseConsumed, records]);

	const byClosed = React.useMemo(
		() =>
			showClosed ? records : records.filter((c) => !afterSaleIsTerminal(c)),
		[records, showClosed],
	);
	const byGroup = React.useMemo(
		() =>
			group === "all"
				? byClosed
				: byClosed.filter((c) => afterSaleGroupOf(c) === group),
		[byClosed, group],
	);
	const visible = React.useMemo(() => {
		const q = search.trim().toLowerCase();
		if (!q) return byGroup;
		return byGroup.filter(
			(c) =>
				c.caseNo.toLowerCase().includes(q) ||
				c.orderNo.toLowerCase().includes(q) ||
				(c.buyerName ?? "").toLowerCase().includes(q),
		);
	}, [byGroup, search]);

	const countOf = React.useCallback(
		(value: AfterSaleGroup) =>
			value === "all"
				? byClosed.length
				: byClosed.filter((c) => afterSaleGroupOf(c) === value).length,
		[byClosed],
	);

	const chips: ReadonlyArray<{ value: AfterSaleGroup; label: string }> =
		showClosed
			? [...GROUP_OPTIONS, { value: "closed", label: "已結案" }]
			: GROUP_OPTIONS;

	const refundingDisabled = (status: string) =>
		status === AFTER_SALE_STATUS.REFUNDING;

	return (
		<>
			<div
				className="seller-orders__quick-filters"
				role="toolbar"
				aria-label="售後狀態篩選"
			>
				<IconTune />
				{chips.map((opt) => {
					const selected = group === opt.value;
					const count = countOf(opt.value);
					const hasCount = count > 0;
					return (
						<button
							key={opt.value}
							type="button"
							className={[
								"seller-orders__chip",
								selected ? "is-selected" : "",
								!hasCount ? "is-empty" : "",
							]
								.filter(Boolean)
								.join(" ")}
							aria-pressed={selected}
							onClick={() => onGroupChange(opt.value)}
						>
							<span>{opt.label}</span>
							<span
								className={[
									"seller-orders__chip-count",
									!hasCount ? "seller-orders__chip-count--zero" : "",
								]
									.filter(Boolean)
									.join(" ")}
							>
								{count}
							</span>
						</button>
					);
				})}
			</div>

			<div className="seller-orders__card seller-orders__desktop-view">
				<div className="seller-orders__table-wrap">
					<table className="seller-orders__table seller-orders__table--aftersale">
						<thead>
							<tr>
								<th>案件編號</th>
								<th>買家</th>
								<th>訂單編號</th>
								<th>原因</th>
								<th className="is-right">可退金額</th>
								<th>狀態</th>
								<th>下一步</th>
								<th className="is-center">操作</th>
							</tr>
						</thead>
						<tbody>
							{loading && records.length === 0 ? (
								<SellerTableSkeletonRows columns={8} rows={5} />
							) : visible.length === 0 ? (
								<tr>
									<td colSpan={8} className="seller-orders__empty">
										<SellerEmptyState variant="orders" seamless />
									</td>
								</tr>
							) : (
								visible.map((c) => (
									<tr
										key={c.caseNo}
										className="seller-orders__row seller-orders__row--clickable"
										onClick={() => setActiveCase(c)}
									>
										<td>
											<button
												type="button"
												className="seller-orders__order-no bg-transparent border-0 p-0 cursor-pointer text-left"
												onClick={(e) => {
													e.stopPropagation();
													setActiveCase(c);
												}}
											>
												{c.caseNo}
											</button>
										</td>
										<td>{c.buyerName || "—"}</td>
										<td>
											<Link
												href={`/seller-center/orders/${encodeURIComponent(c.orderNo)}`}
												className="seller-orders__order-no"
												onClick={(e) => e.stopPropagation()}
											>
												{c.orderNo}
											</Link>
										</td>
										<td>{formatReason(c.reason)}</td>
										<td className="is-right seller-orders__amount">
											{formatMoney(c.refundableAmount)}
										</td>
										<td>
											<span
												className={statusBadgeClass(c.status, c.appealState)}
											>
												{afterSaleDisplayLabel(c.status, c.appealState)}
											</span>
										</td>
										<td className="text-foreground-soft text-sm">
											{nextStepText(c.status, c.appealState)}
											{c.status === AFTER_SALE_STATUS.RETURN_AT_STORE &&
											c.storePickupDeadlineAt ? (
												<small className="block">
													{c.storePickupDeadlineAt}
												</small>
											) : c.status === AFTER_SALE_STATUS.RETURN_PENDING &&
												c.shipBackDeadlineAt ? (
												<small className="block text-warning-600">
													寄回期限 {c.shipBackDeadlineAt}
												</small>
											) : c.processDeadline ? (
												<small className="block">{c.processDeadline}</small>
											) : null}
										</td>
										<td className="is-center">
											<div className="seller-orders__actions">
												<button
													type="button"
													className="seller-orders__action seller-orders__action--primary disabled:opacity-50 disabled:cursor-not-allowed"
													disabled={refundingDisabled(c.status)}
													onClick={(e) => {
														e.stopPropagation();
														setActiveCase(c);
													}}
												>
													{actionLabel(c.status, c.appealState)}
												</button>
											</div>
										</td>
									</tr>
								))
							)}
						</tbody>
					</table>
				</div>
			</div>

			<section
				className="seller-orders__mobile-view seller-orders__mobile-view--aftersale"
				aria-label="售後案件清單"
				aria-busy={loading && records.length > 0 ? true : undefined}
			>
				{loading && records.length === 0 ? (
					<div className="seller-orders__mobile-loading" role="status">
						載入售後案件中…
					</div>
				) : visible.length === 0 ? (
					<div className="seller-orders__mobile-empty">
						<SellerEmptyState variant="orders" seamless />
					</div>
				) : (
					<ul className="seller-orders__mobile-list">
						{visible.map((c) => (
							<li key={c.caseNo}>
								<article className="seller-orders__mobile-card seller-orders__mobile-card--aftersale">
									<button
										type="button"
										className="seller-orders__mobile-card-main"
										onClick={() => setActiveCase(c)}
									>
										<span className="visually-hidden">查看售後案件詳情</span>
										<span className="seller-orders__mobile-card-head">
											<span className="seller-orders__mobile-card-heading">
												<strong className="seller-orders__mobile-card-order">
													{c.caseNo}
												</strong>
												<small className="seller-orders__mobile-card-time">
													訂單 {c.orderNo}
												</small>
											</span>
											<span
												className={`${statusBadgeClass(c.status, c.appealState)} seller-orders__mobile-card-status`}
											>
												{afterSaleDisplayLabel(c.status, c.appealState)}
											</span>
										</span>

										<span className="seller-orders__mobile-card-summary">
											<strong className="seller-orders__mobile-card-buyer">
												{c.buyerName || "—"}
											</strong>
											<strong className="seller-orders__mobile-card-amount">
												{formatMoney(c.refundableAmount)}
											</strong>
										</span>

										<span className="seller-orders__mobile-card-case-reason">
											{formatReason(c.reason)}
										</span>
										<span className="seller-orders__mobile-card-facts">
											<span>{nextStepText(c.status, c.appealState)}</span>
											{c.status === AFTER_SALE_STATUS.RETURN_AT_STORE &&
											c.storePickupDeadlineAt ? (
												<span>{c.storePickupDeadlineAt}</span>
											) : c.status === AFTER_SALE_STATUS.RETURN_PENDING &&
												c.shipBackDeadlineAt ? (
												<span>寄回期限 {c.shipBackDeadlineAt}</span>
											) : c.processDeadline ? (
												<span>{c.processDeadline}</span>
											) : null}
										</span>
									</button>

									<div className="seller-orders__mobile-card-actions">
										<Link
											href={`/seller-center/orders/${encodeURIComponent(c.orderNo)}`}
											className="seller-orders__mobile-card-detail"
										>
											查看訂單
										</Link>
										<button
											type="button"
											className="seller-orders__action seller-orders__action--primary seller-orders__mobile-card-primary"
											disabled={refundingDisabled(c.status)}
											onClick={() => setActiveCase(c)}
										>
											{actionLabel(c.status, c.appealState)}
										</button>
									</div>
								</article>
							</li>
						))}
					</ul>
				)}
			</section>

			<AfterSaleDecisionDrawer
				open={activeCase !== null}
				summary={activeCase}
				onClose={() => setActiveCase(null)}
				onMutated={onRefresh}
			/>
		</>
	);
}
