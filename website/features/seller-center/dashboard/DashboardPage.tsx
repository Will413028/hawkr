"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import EmptyStatePanel from "@/components/shared/EmptyStatePanel";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import {
	SellerMobileDisclosureIcon,
	SellerMobileDisclosureList,
} from "@/features/seller-center/components/SellerMobileDisclosureList";
import { formatDeadline } from "@/features/seller-center/orders/orderDisplay";
import { useAuth } from "@/lib/auth/context";
import {
	OrderStatus,
	ProductStatus,
	SHIPPING_METHOD_LABEL,
	type ShippingMethodValue,
} from "@/lib/codemeta/enums";
import { taipeiParts } from "@/lib/date/taipei";
import { useOrderList } from "@/lib/hooks/useOrders";
import { useSellerAfterSales } from "@/lib/hooks/useSellerAfterSales";
import { useSellerDashboard } from "@/lib/hooks/useSellerDashboard";
import { useSellerProducts } from "@/lib/hooks/useSellerProducts";
import { useSellerSelf } from "@/lib/hooks/useSellerSelf";
import {
	useSellerShipmentAnomalies,
	useSellerShipmentAnomalyStats,
} from "@/lib/hooks/useSellerShipmentAnomalies";
import { useShippingOnboardings } from "@/lib/hooks/useShippingOnboardings";
import { useUiText } from "@/lib/i18n/ui-text";
import type { Order } from "@/lib/orders/types";
import { STATUS_LABEL } from "@/lib/orders/types";
import {
	type AfterSaleCase,
	afterSaleNeedsSeller,
} from "@/lib/seller/after-sales";
import { useCertGate } from "@/lib/seller/cert-gate";
import {
	periodGrowthPercent,
	periodLabel,
	type SellerDashboard,
} from "@/lib/seller/dashboard";
import { payuniHasState, payuniStateTitle } from "@/lib/seller/payuni";
import { isTcatCalled } from "@/lib/seller/shipment";
import type { ShipmentAnomalyItem } from "@/lib/shipping/anomalies";
import { SellerTableSkeletonRows } from "../components/SellerSkeletons";
import { StatCard } from "../components/StatCard";
import OnboardingProgressCard from "./OnboardingProgressCard";

// 待出貨 = PAID(10) + PENDING_SHIPMENT(20), as the numeric CodeMeta CSV the
// backend `/app/orders` status filter expects (matching SellerOrdersPage's
// "待出貨" tab) — NOT the web semantic name "preparing".
const PENDING_SHIP_STATUS_CSV = [
	OrderStatus.PAID,
	OrderStatus.PENDING_SHIPMENT,
].join(",");

const SELLER_ORDERS_PENDING_HREF = "/seller-center/orders?tab=pending";
const SELLER_ORDERS_IN_TRANSIT_HREF = "/seller-center/orders?tab=in_transit";
const SELLER_ORDERS_COMPLETED_HREF = "/seller-center/orders?tab=completed";
const SELLER_ORDERS_AFTERSALE_HREF = "/seller-center/orders?tab=aftersale";

export default function DashboardPage() {
	const { identity } = useAuth();
	// `view: "seller"` is required — `/app/orders` defaults to the BUYER view,
	// so without it a seller account gets its (empty) buyer orders and the
	// "待處理訂單" block is always empty. See SellerOrdersPage.
	const pendingShip = useOrderList(PENDING_SHIP_STATUS_CSV, {
		view: "seller",
	});
	const certGate = useCertGate();

	const { data: self } = useSellerSelf();
	const ut = useUiText();
	const payuni = certGate.flow?.payuni;
	const payuniBanner = payuniHasState(payuni) ? payuni : null;
	const {
		data: dash,
		loadFailed: dashLoadFailed,
		refresh: refreshDashboard,
	} = useSellerDashboard();
	const todosPhase = todosPhaseOf(dash, dashLoadFailed);
	const dashLoading = todosPhase === "loading";
	const todosUnknown = todosPhase === "unknown";
	// Just the `.total` for the onboarding progress card — shares the seller
	// products cache and refetches when ProductsPage/InventoryPage mutate.
	const { data: productsList } = useSellerProducts({
		page: 1,
		pageSize: 1,
		status: ProductStatus.ACTIVE,
	});
	const productCount = productsList?.total ?? null;
	// shipping shares its seller-scoped query with WalletClient /
	// ShippingClient / SkuShippingEditor (same payload, no params → deduped).
	const { rows: shippingRows } = useShippingOnboardings();
	// Keep the badge lightweight, but fetch a small sample so provider failure
	// details can be surfaced without loading the full orders table.
	const { stats: anomalyStats } = useSellerShipmentAnomalyStats();
	const { records: anomalyRecords, loading: anomalyRecordsLoading } =
		useSellerShipmentAnomalies({ pageSize: 3 });

	const greeting = self?.store?.storeName || identity?.nickname || "賣家";
	const todayLabel = formatToday();

	const stats = buildStats(dash);
	const nearestPendingShipDeadline = React.useMemo(
		() => findNearestShipmentDeadline(pendingShip.orders),
		[pendingShip.orders],
	);
	const pendingShipmentTodoDescription = React.useMemo(() => {
		const deadline = formatDeadline(nearestPendingShipDeadline);
		return deadline ? `最近出貨期限：${deadline}` : undefined;
	}, [nearestPendingShipDeadline]);

	// 售後案件也屬於「待處理」——撈賣家售後案件，只留需要賣家動作的
	// (待處理 / 已收退貨待確認)，最新的排前面。
	const afterSales = useSellerAfterSales();
	const pendingAfterSales = React.useMemo(
		() =>
			Array.from(afterSales.lookup.values())
				.filter(afterSaleNeedsSeller)
				.sort((a, b) => (b.createdAt > a.createdAt ? 1 : -1)),
		[afterSales.lookup],
	);
	const afterSalesTodoDescription = React.useMemo(
		() => formatAfterSalesTodoDescription(pendingAfterSales),
		[pendingAfterSales],
	);

	// Shipment anomalies surface as a 今日待辦 item (rather than a standalone
	// banner) so all "needs action today" lives in one list. Prepended as the
	// most urgent entry; independent of the dash rollup so it shows even before
	// `dash` resolves.
	const todos = React.useMemo(() => {
		const list = buildTodos(dash, {
			afterSalesDescription: afterSalesTodoDescription,
			pendingShipmentDescription: pendingShipmentTodoDescription,
		});
		if (anomalyStats.total > 0) {
			list.unshift({
				id: "anomaly",
				title: `${anomalyStats.total} 筆配送異常待處理`,
				description:
					formatAnomalyTodoDescription(anomalyRecords[0]) ??
					(anomalyRecordsLoading
						? "正在讀取異常原因..."
						: "點進訂單列表查看配送異常"),
				action: "處理",
				href: SELLER_ORDERS_IN_TRANSIT_HREF,
				urgent: true,
			});
		}
		return list;
	}, [
		dash,
		afterSalesTodoDescription,
		pendingShipmentTodoDescription,
		anomalyStats.total,
		anomalyRecords,
		anomalyRecordsLoading,
	]);

	return (
		<div className="seller-dashboard">
			<header className="seller-dashboard__head">
				<div>
					<h1>賣家總覽</h1>
					<div className="seller-dashboard__head-meta">
						歡迎回來，{greeting}
						<span>·</span>
						{todayLabel}
					</div>
				</div>
				<Button
					href="/seller-center/posts/new"
					className="seller-page-header-cta"
				>
					＋ 發布動態
				</Button>
			</header>

			{/* Payuni banner is sticky between the head and main content.
			    （商家認證 banner 已移除 — 認證狀態由 OnboardingProgressCard 與
			    待辦清單導流，dashboard 不再置頂提示。） */}

			{payuniBanner && (
				<div className="seller-banner">
					<div className="seller-banner__text">
						<strong>{payuniStateTitle(ut, payuniBanner.state)}</strong>
						<span>{payuniBanner.explanation?.message}</span>
					</div>
					<Button
						href="/seller-center/wallet"
						color="warning"
						size="sm"
						className="flex-none"
					>
						前往處理
					</Button>
				</div>
			)}

			{dashLoadFailed && (
				<div className="seller-banner">
					<div className="seller-banner__text">
						<strong>無法載入總覽資料</strong>
					</div>
					<Button
						onClick={() => void refreshDashboard()}
						color="warning"
						size="sm"
						className="flex-none"
					>
						重試
					</Button>
				</div>
			)}

			<OnboardingProgressCard
				self={self}
				flowStatus={certGate.flowStatus}
				shippingRows={shippingRows}
				// GetMyStore 沒有商品數：商品清單的 total 是唯一來源。
				productCount={productCount}
			/>

			<div className="seller-dashboard__top">
				<section className="seller-card">
					<div className="seller-card__head">
						<h2>今日待辦</h2>
						{dashLoading ? null : (
							<small>{todosUnknown ? UNSET_TEXT : todos.length} 項待處理</small>
						)}
					</div>
					{todos.length === 0 && !dashLoading && !todosUnknown && (
						<EmptyStatePanel
							icon="trophy"
							title="目前沒有待辦事項"
							description="繼續加油，今天也把好食材照顧好了。"
							className="seller-empty seller-empty--todos"
							iconSize={128}
						/>
					)}
					{todos.length > 0 && (
						<div className="seller-todos">
							{todos.map((t) => (
								<Link
									key={t.id}
									href={t.href}
									className={
										t.urgent
											? "seller-todo seller-todo--urgent"
											: "seller-todo seller-todo--quiet"
									}
								>
									<span className="seller-todo__dot" />
									<span className="seller-todo__content">
										<span className="seller-todo__title">{t.title}</span>
										{t.description && (
											<span
												className="seller-todo__description"
												title={t.description}
											>
												{t.description}
											</span>
										)}
									</span>
									<span className="seller-todo__action">{t.action}</span>
								</Link>
							))}
						</div>
					)}
					{todosUnknown && (
						<div className="seller-empty seller-empty--todos flex flex-col items-center justify-center gap-1">
							<strong>暫時無法取得待辦</strong>
							<span>重新整理頁面，或到訂單頁查看</span>
						</div>
					)}
				</section>

				<aside className="seller-card">
					<div className="seller-card__head">
						<h2>快速操作</h2>
					</div>
					<div className="seller-quickactions">
						<Link
							href="/seller-center/products/new"
							className="seller-quickaction"
						>
							<span className="seller-quickaction__icon">
								<IconPlus />
							</span>
							新增商品
						</Link>
						<Link
							href="/seller-center/coupons/new"
							className="seller-quickaction"
						>
							<span className="seller-quickaction__icon">
								<IconTicket />
							</span>
							建立優惠券
						</Link>
						<Link href="/seller-center/orders" className="seller-quickaction">
							<span className="seller-quickaction__icon">
								<IconDownload />
							</span>
							匯出訂單
						</Link>
						<Link
							href="/seller-center/posts/new"
							className="seller-quickaction"
						>
							<span className="seller-quickaction__icon">
								<IconSend />
							</span>
							發布動態
						</Link>
					</div>
				</aside>
			</div>

			<section
				className="seller-stats seller-summary-rail"
				aria-label="營運摘要"
				// biome-ignore lint/a11y/noNoninteractiveTabindex: This horizontally scrollable summary needs a keyboard focus target.
				tabIndex={0}
			>
				{stats.map((s) => (
					<StatCard
						key={s.label}
						icon={s.icon}
						value={s.value}
						label={s.label}
						change={s.change}
						changeNeutral={s.changeNeutral}
						caption={s.caption}
					/>
				))}
			</section>

			<section className="seller-card">
				<div className="seller-card__head">
					<h2>待處理訂單與售後</h2>
					<Link href={SELLER_ORDERS_PENDING_HREF} className="seller-card__link">
						查看待出貨
						<AppIcon name="arrow-right" size={16} />
					</Link>
				</div>
				<PendingOrderTable
					orders={pendingShip.orders}
					afterSales={pendingAfterSales}
					loading={pendingShip.loading || afterSales.loading}
				/>
			</section>
		</div>
	);
}

/* ------------------------------------------------------------------ */

/* ------------------------------------------------------------------ */

type ActionRow =
	| { kind: "order"; order: Order }
	| { kind: "aftersale"; case: AfterSaleCase };

function PendingOrderTable({
	orders,
	afterSales,
	loading,
}: {
	orders: Order[];
	afterSales: AfterSaleCase[];
	loading: boolean;
}) {
	// 售後優先（較急），其後接待出貨訂單；整體取前 5 筆。
	const rows: ActionRow[] = [
		...afterSales.map((c) => ({ kind: "aftersale", case: c }) as const),
		...orders.map((o) => ({ kind: "order", order: o }) as const),
	];
	if (loading && rows.length === 0) {
		// Match the 8-col layout of the loaded table below so the column
		// widths stay stable when data arrives.
		return (
			<div className="seller-table-wrap seller-mobile-disclosure-desktop">
				<table className="seller-table">
					<thead>
						<tr>
							<th>編號</th>
							<th>顧客</th>
							<th>商品 / 事由</th>
							<th>數量</th>
							<th>金額</th>
							<th>配送 / 類型</th>
							<th>狀態</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						<SellerTableSkeletonRows columns={8} rows={3} />
					</tbody>
				</table>
			</div>
		);
	}
	if (rows.length === 0) {
		return (
			<EmptyStatePanel
				icon="paper"
				title="目前沒有待處理項目"
				description="新的訂單或售後申請進來後，會在這裡提醒你處理。"
				className="seller-empty seller-empty--orders"
				iconSize={118}
			/>
		);
	}

	const visible = rows.slice(0, 5).map(toDisplayRow);

	return (
		<>
			{/* Desktop: full 8-column table */}
			<div className="seller-table-wrap seller-mobile-disclosure-desktop">
				<table className="seller-table">
					<thead>
						<tr>
							<th>編號</th>
							<th>顧客</th>
							<th>商品 / 事由</th>
							<th>數量</th>
							<th>金額</th>
							<th>配送 / 類型</th>
							<th>狀態</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						{visible.map((r) => (
							<tr key={r.key}>
								<td className="seller-table__id">{r.id}</td>
								<td className="seller-table__buyer">
									<strong>{r.buyer}</strong>
									{r.buyerPhone && <small>{r.buyerPhone}</small>}
								</td>
								<td>{r.product}</td>
								<td>{r.qty || "—"}</td>
								<td className="seller-table__amount">{r.amount}</td>
								<td>{r.delivery}</td>
								<td>
									<span className={`seller-status ${r.statusClass}`}>
										{r.statusLabel}
									</span>
								</td>
								<td>
									<Link href={r.href} className="seller-table__action">
										{r.actionLabel}
									</Link>
								</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>

			<SellerMobileDisclosureList
				items={visible}
				getKey={displayRowKey}
				ariaLabel="手機版待處理訂單與售後"
				getSummary={(row) => ({
					label: `${row.statusLabel} ${row.id}`,
					leading: (
						<SellerMobileDisclosureIcon
							name={row.delivery === "售後" ? "reorder" : "bag"}
						/>
					),
					title: row.id,
					meta: row.amount,
					status: (
						<span className={`seller-status ${row.statusClass}`}>
							{row.statusLabel}
						</span>
					),
				})}
				renderDetails={(row) => (
					<>
						<div className="seller-mobile-disclosure__body">
							<div className="seller-mobile-disclosure__detail-row">
								<span>顧客</span>
								<strong>
									{row.buyer}
									{row.buyerPhone ? ` · ${row.buyerPhone}` : ""}
								</strong>
							</div>
							<div className="seller-mobile-disclosure__detail-row">
								<span>商品 / 事由</span>
								<strong>
									{row.product}
									{row.qty ? ` ×${row.qty}` : ""}
								</strong>
							</div>
							<div className="seller-mobile-disclosure__detail-row">
								<span>配送 / 類型</span>
								<strong>{row.delivery}</strong>
							</div>
						</div>
						<div className="seller-mobile-disclosure__actions">
							<Link href={row.href} className="seller-table__action">
								{row.actionLabel}
							</Link>
						</div>
					</>
				)}
			/>
		</>
	);
}

/* Normalised row shared by the desktop table and the mobile card list, so the
 * order-vs-aftersale branching lives in one place. */
interface DisplayRow {
	key: string;
	id: string;
	buyer: string;
	buyerPhone?: string;
	product: string;
	/** Quantity as a string; "" for after-sale rows (rendered as "—"). */
	qty: string;
	amount: string;
	delivery: string;
	statusLabel: string;
	statusClass: string;
	href: string;
	actionLabel: string;
}

function displayRowKey(row: DisplayRow): string {
	return row.key;
}

function toDisplayRow(row: ActionRow): DisplayRow {
	if (row.kind === "aftersale") {
		const c = row.case;
		const href = c.caseNo
			? `${SELLER_ORDERS_AFTERSALE_HREF}&caseNo=${encodeURIComponent(c.caseNo)}`
			: `${SELLER_ORDERS_AFTERSALE_HREF}&focus=${encodeURIComponent(c.orderNo)}`;
		return {
			key: `as-${c.caseNo || c.orderNo}`,
			id: c.orderNo,
			buyer: c.buyerName || "—",
			product: c.reason || "退換貨申請",
			qty: "",
			amount: c.refundableAmount
				? `NT$ ${Number(c.refundableAmount).toLocaleString()}`
				: "—",
			delivery: "售後",
			statusLabel: afterSaleStatusLabel(c.status),
			statusClass: "seller-status--cancelled",
			href,
			actionLabel: "處理售後",
		};
	}
	const o = row.order;
	const firstItem = o.items?.[0];
	const itemCount = (o.items ?? []).reduce(
		(sum, it) => sum + Number(it.quantity || 0),
		0,
	);
	return {
		key: o.orderNo,
		id: o.orderNo,
		buyer: o.shipping?.receiverName || "—",
		buyerPhone: o.shipping?.receiverPhone || undefined,
		product: firstItem?.name || "—",
		qty: String(itemCount),
		amount: `NT$ ${Number(o.totalActualAmount || 0).toLocaleString()}`,
		delivery: o.shipping?.method
			? (SHIPPING_METHOD_LABEL[o.shipping.method as ShippingMethodValue] ??
				o.shipping.method)
			: "宅配",
		statusLabel: STATUS_LABEL[o.status] || o.status,
		statusClass: statusClass(o.status),
		href: `${ordersHrefForOrder(o)}&focus=${encodeURIComponent(o.orderNo)}`,
		actionLabel: "處理出貨",
	};
}

/* ------------------------------------------------------------------ */
/* Helpers                                                              */
/* ------------------------------------------------------------------ */

/**
 * 賣家儀表板抬頭的「今天」。以**台北**牆上時間計算，不是裝置時區：這是台灣商家
 * 的營運日期，賣家人在國外時該看到的仍是台灣的今天（出貨期限、對帳都以它為準）。
 * 排版 `YYYY-MM-DD 週X` 既有 formatter 湊不出來，所以走 `taipeiParts` 逃生門。
 */
function formatToday(): string {
	const parts = taipeiParts(Date.now());
	if (!parts) return "";
	const pad = (n: number) => String(n).padStart(2, "0");
	const weekdays = ["日", "一", "二", "三", "四", "五", "六"];
	return `${parts.year}-${pad(parts.month)}-${pad(parts.day)} 週${weekdays[parts.weekday]}`;
}

/** Seller-action after-sale states (codemeta AfterSaleCaseStatus): 0 待處理,
 *  30 已收退貨待確認. Other states are filtered out upstream. */
function afterSaleStatusLabel(status: string): string {
	switch (status) {
		case "0":
			return "售後待處理";
		case "30":
			return "待確認退貨";
		default:
			return "售後處理中";
	}
}

function statusClass(status: string): string {
	switch (status) {
		case "preparing":
		case "paid":
		case "pending_payment":
			return "seller-status--pending";
		case "shipping":
		case "delivered":
			return "seller-status--shipping";
		case "completed":
			return "seller-status--complete";
		case "cancelled":
		case "refunded":
			return "seller-status--cancelled";
		default:
			return "seller-status--pending";
	}
}

function ordersHrefForOrder(order: Order): string {
	if (isTcatCalled(order.shipping)) return SELLER_ORDERS_IN_TRANSIT_HREF;
	switch (order.status) {
		case "shipping":
		case "waiting_pickup":
		case "shipping_abnormal":
		case "delivered":
			return SELLER_ORDERS_IN_TRANSIT_HREF;
		case "completed":
		case "cancelled":
		case "refunded":
		case "partially_refunded":
		case "after_sale":
			return SELLER_ORDERS_COMPLETED_HREF;
		default:
			return SELLER_ORDERS_PENDING_HREF;
	}
}

function formatAnomalyTodoDescription(
	anomaly: ShipmentAnomalyItem | undefined,
): string | undefined {
	if (!anomaly) return undefined;

	const orderLabel = anomaly.orderNo ? `訂單 ${anomaly.orderNo}` : "配送異常";
	const providerMessage = anomaly.providerMessage?.trim();
	if (providerMessage) return `${orderLabel}｜PayUni：${providerMessage}`;

	return `${orderLabel}｜${anomaly.anomalyTypeLabel}`;
}

function normalizeDeadlineMs(ts: number | null | undefined): number | null {
	if (ts == null) return null;
	const ms = ts < 1e12 ? ts * 1000 : ts;
	return Number.isFinite(ms) ? ms : null;
}

function findNearestShipmentDeadline(
	orders: ReadonlyArray<Order>,
): number | null {
	let nearest: number | null = null;
	for (const order of orders) {
		const deadline = normalizeDeadlineMs(order.autoCancelAt);
		if (deadline == null) continue;
		if (nearest == null || deadline < nearest) nearest = deadline;
	}
	return nearest;
}

function normalizeAfterSaleTimeMs(
	raw: string | null | undefined,
): number | null {
	if (!raw) return null;
	const numeric = Number(raw);
	const ms = Number.isFinite(numeric)
		? numeric < 1e12
			? numeric * 1000
			: numeric
		: Date.parse(raw);
	return Number.isFinite(ms) ? ms : null;
}

function findNearestAfterSaleProcessDeadline(
	cases: ReadonlyArray<AfterSaleCase>,
): number | null {
	let nearest: number | null = null;
	for (const item of cases) {
		const deadline = normalizeAfterSaleTimeMs(item.processDeadline);
		if (deadline == null) continue;
		if (nearest == null || deadline < nearest) nearest = deadline;
	}
	return nearest;
}

function formatAfterSalesTodoDescription(
	cases: ReadonlyArray<AfterSaleCase>,
): string | undefined {
	const nearestDeadline = findNearestAfterSaleProcessDeadline(cases);
	const deadlineLabel = formatDeadline(nearestDeadline);
	if (deadlineLabel) return `最近處理期限：${deadlineLabel}`;

	const latestCreated = normalizeAfterSaleTimeMs(cases[0]?.createdAt);
	const latestCreatedLabel = formatDeadline(latestCreated);
	return latestCreatedLabel ? `最新售後申請：${latestCreatedLabel}` : undefined;
}

/**
 * 今日待辦的三種前提：第一次讀取中（沒有資料也沒有失敗）、不知道（第一次讀取失敗，或
 * `orderStatusCounts` 沒設）、讀到了。「不知道」不是「沒有待辦」。
 */
export function todosPhaseOf(
	dash: SellerDashboard | null,
	loadFailed: boolean,
): "loading" | "unknown" | "ready" {
	if (dash === null && !loadFailed) return "loading";
	return dash?.orderStatusCounts ? "ready" : "unknown";
}

interface TodoItem {
	id: string;
	title: string;
	description?: string;
	action: string;
	href: string;
	urgent: boolean;
}

interface BuildTodosOptions {
	afterSalesDescription?: string;
	pendingShipmentDescription?: string;
}

function buildTodos(
	dash: SellerDashboard | null,
	options: BuildTodosOptions = {},
): TodoItem[] {
	const counts = dash?.orderStatusCounts;
	if (!counts) return [];
	const out: TodoItem[] = [];
	if (counts.pendingShipmentCount > 0) {
		out.push({
			id: "ship",
			title: `${counts.pendingShipmentCount} 筆訂單待出貨`,
			...(options.pendingShipmentDescription
				? { description: options.pendingShipmentDescription }
				: {}),
			action: "處理",
			href: SELLER_ORDERS_PENDING_HREF,
			urgent: true,
		});
	}
	if (counts.aftersalesCount > 0) {
		out.push({
			id: "after",
			title: `${counts.aftersalesCount} 筆售後待處理`,
			...(options.afterSalesDescription
				? { description: options.afterSalesDescription }
				: {}),
			action: "處理",
			href: SELLER_ORDERS_AFTERSALE_HREF,
			urgent: false,
		});
	}
	if (counts.inProgressCount > 0) {
		out.push({
			id: "shipping",
			title: `${counts.inProgressCount} 筆訂單運送中`,
			action: "追蹤",
			href: SELLER_ORDERS_IN_TRANSIT_HREF,
			urgent: false,
		});
	}
	return out;
}

interface StatItem {
	label: string;
	value: string;
	change: string;
	changeNeutral?: boolean;
	/** 數字所屬的期間；訂單狀態是目前快照，沒有期間。 */
	caption?: string;
	icon: React.ReactNode;
}

const UNSET_TEXT = "—";

export function buildStats(dash: SellerDashboard | null): StatItem[] {
	const revenue = dash?.sales?.revenue;
	const counts = dash?.orderStatusCounts;
	const views = dash?.productViews;
	const newFollowers = dash?.followers?.newFollowers;
	const period = periodLabel(dash?.currentPeriod);

	const revenueChange = formatGrowth(periodGrowthPercent(revenue));
	const viewsChange = formatGrowth(periodGrowthPercent(views));

	return [
		{
			label: "近 7 日營收",
			value: revenue
				? `NT$ ${Number(revenue.current).toLocaleString()}`
				: UNSET_TEXT,
			change: revenueChange.text,
			changeNeutral: revenueChange.neutral,
			caption: period,
			icon: <IconDollar />,
		},
		{
			label: "待出貨訂單",
			value: counts ? String(counts.pendingShipmentCount) : UNSET_TEXT,
			change:
				counts && counts.aftersalesCount > 0
					? `${counts.aftersalesCount} 筆售後`
					: UNSET_TEXT,
			changeNeutral: !counts || counts.aftersalesCount === 0,
			icon: <IconBox />,
		},
		{
			label: "商品瀏覽",
			value: views ? views.current.toLocaleString() : UNSET_TEXT,
			change: viewsChange.text,
			changeNeutral: viewsChange.neutral,
			caption: period,
			icon: <IconEye />,
		},
		{
			label: "新追蹤者",
			value: newFollowers
				? newFollowers.current > 0
					? `+${newFollowers.current}`
					: "0"
				: UNSET_TEXT,
			change: newFollowers ? "近 7 日" : UNSET_TEXT,
			changeNeutral: true,
			caption: period,
			icon: <IconUsers />,
		},
	];
}

/**
 * 較前 7 日的成長百分比（`periodGrowthPercent` 由兩個期間算出）。前期為 0 或區塊沒設
 * 時沒有基準，畫中性的破折號，不掛期間詞（「較前 7 日 —」讀不出意思）。
 */
export function formatGrowth(percent: number | undefined): {
	text: string;
	neutral: boolean;
} {
	if (percent === undefined) return { text: UNSET_TEXT, neutral: true };
	const sign = percent > 0 ? "+" : "";
	return { text: `較前 7 日 ${sign}${percent}%`, neutral: percent === 0 };
}

function IconPlus() {
	return <AppIcon name="plus" />;
}
function IconTicket() {
	return <AppIcon name="ticket" />;
}
function IconDownload() {
	return <AppIcon name="download" />;
}
function IconSend() {
	return <AppIcon name="send" />;
}
function IconDollar() {
	return <AppIcon name="wallet" />;
}
function IconBox() {
	return <AppIcon name="box" />;
}
function IconEye() {
	return <AppIcon name="show" />;
}
function IconUsers() {
	return <AppIcon name="people" />;
}
