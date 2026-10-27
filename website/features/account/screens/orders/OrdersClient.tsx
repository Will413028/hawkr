"use client";

import { useSearchParams } from "next/navigation";
import { createParser, useQueryState } from "nuqs";
import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import Link from "@/components/shared/Link";
import {
	ScEmpty,
	ScEmptyDesc,
	ScEmptyTitle,
	ScFilters,
	ScPillCount,
} from "@/components/shared/ScPrimitives";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import OrderPaymentCountdown from "@/features/account/screens/orders/OrderPaymentCountdown";
import OrderQuickActionDrawer from "@/features/account/screens/orders/OrderQuickActionDrawer";
import { buyerOrderStatusBadgeClass } from "@/features/account/screens/orders/statusBadgeStyles";
import { AfterSaleStatusBadge } from "@/features/after-sales/components/AfterSaleStatusBadge";
import type { AfterSaleCase } from "@/lib/after-sales/types";
import { OrderStatus as CodeMetaOrderStatus } from "@/lib/codemeta/enums";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useMyAfterSales } from "@/lib/hooks/useMyAfterSales";
import { useOrderList, useOrderStatusCounts } from "@/lib/hooks/useOrders";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { type Order, STATUS_LABEL } from "@/lib/orders/types";

const PILL_BASE =
	"inline-flex items-center gap-1.5 h-8 px-[14px] border rounded-[var(--radius-pill)] text-[0.8125rem] font-bold no-underline cursor-pointer transition-[background,color,border-color] duration-[160ms] ease-out";
const PILL_INACTIVE =
	"border-divider bg-content1 text-foreground-soft hover:bg-content2 hover:text-foreground";
const PILL_ACTIVE =
	"border-transparent bg-hkui-primary text-hk-cream hover:bg-hkui-primary hover:text-hk-cream";

interface StatusTab {
	value: string;
	label: string;
	/** Backend `/app/orders?status=` accepts codemeta CSV values. */
	statusCsv?: string;
}

const STATUS_TABS: StatusTab[] = [
	{ value: "", label: "全部" },
	{
		value: "pending_payment",
		label: "待付款",
		statusCsv: CodeMetaOrderStatus.PENDING_PAYMENT,
	},
	{
		value: "preparing",
		label: "待出貨",
		statusCsv: [
			CodeMetaOrderStatus.PAID,
			CodeMetaOrderStatus.PENDING_SHIPMENT,
		].join(","),
	},
	{
		value: "shipping",
		label: "運送中",
		statusCsv: [
			CodeMetaOrderStatus.IN_TRANSIT,
			CodeMetaOrderStatus.WAITING_PICKUP,
			CodeMetaOrderStatus.SHIPPING_ABNORMAL,
			CodeMetaOrderStatus.DELIVERED,
		].join(","),
	},
	{
		value: "completed",
		label: "已完成",
		statusCsv: CodeMetaOrderStatus.COMPLETED,
	},
	{
		value: "cancelled",
		label: "取消/退款",
		statusCsv: [
			CodeMetaOrderStatus.PARTIALLY_REFUNDED,
			CodeMetaOrderStatus.FULLY_REFUNDED,
			CodeMetaOrderStatus.CANCELLED,
		].join(","),
	},
];

function normalizeStatusTab(value: string | null | undefined) {
	if (!value) return "";
	const normalized = value.trim();
	const tab = STATUS_TABS.find(
		(item) => item.value === normalized || item.statusCsv === normalized,
	);
	return tab?.value ?? "";
}

/** `?status=` 讀取走 nuqs：接受 tab value 或 legacy codemeta CSV（normalize
 * 成 tab value），未知值落回預設「全部」。寫入維持 `<Link>` 導覽。 */
const statusTabParser = createParser({
	parse: (value) => normalizeStatusTab(value),
	serialize: (value: string) => value,
}).withDefault("");

function hrefForStatusTab(
	status: string,
	searchParams: ReturnType<typeof useSearchParams>,
) {
	const params = new URLSearchParams(searchParams?.toString());
	if (status) {
		params.set("status", status);
	} else {
		params.delete("status");
	}
	const query = params.toString();
	return query ? `/account/orders?${query}` : "/account/orders";
}

// Stable list of statusCsv keys to feed `useOrderStatusCounts`. Defined
// at module scope so the hook's dep-key digest stays the same across
// renders.
const COUNT_KEYS: ReadonlyArray<string> = STATUS_TABS.map(
	(tab) => tab.statusCsv ?? "",
);

export default function OrdersClient() {
	const searchParams = useSearchParams();
	const [status] = useQueryState("status", statusTabParser);
	const activeTab =
		STATUS_TABS.find((tab) => tab.value === status) ?? STATUS_TABS[0];
	const list = useOrderList(activeTab.statusCsv);
	const { counts } = useOrderStatusCounts(COUNT_KEYS);
	const afterSales = useMyAfterSales();
	const [activeOrder, setActiveOrder] = React.useState<Order | null>(null);
	const activeAfterSale = activeOrder
		? afterSales.lookup.get(activeOrder.orderNo)
		: undefined;
	const subtitle = activeTab.value
		? `${activeTab.label} ${list.total} 筆訂單。點訂單可操作下一步或查看詳情。`
		: `總共 ${list.total} 筆訂單。點訂單可操作下一步或查看詳情。`;

	return (
		<main className="orders-page">
			<SellerPageShell
				title="我的訂單"
				subtitle={subtitle}
				toolbar={
					<ScFilters aria-label="訂單狀態篩選">
						{STATUS_TABS.map((t) => {
							const active = status === t.value;
							const count = counts[t.statusCsv ?? ""];
							return (
								<Link
									aria-current={active ? "page" : undefined}
									aria-selected={active}
									className={[PILL_BASE, active ? PILL_ACTIVE : PILL_INACTIVE]
										.filter(Boolean)
										.join(" ")}
									href={hrefForStatusTab(t.value, searchParams)}
									key={t.value || "all"}
									prefetch={false}
									role="tab"
								>
									{t.label}
									{typeof count === "number" && count > 0 ? (
										<ScPillCount active={active}>{count}</ScPillCount>
									) : null}
								</Link>
							);
						})}
					</ScFilters>
				}
			>
				{list.error && <p role="alert">{list.error}</p>}

				{list.loading && list.orders.length === 0 ? (
					<p className="text-foreground-soft text-sm">載入中…</p>
				) : list.orders.length === 0 ? (
					<EmptyOrders status={status} statusLabel={activeTab.label} />
				) : (
					<ul className="list-none m-0 p-0 flex flex-col gap-3">
						{list.orders.map((order) => (
							<OrderRow
								key={order.orderNo}
								order={order}
								aftersale={afterSales.lookup.get(order.orderNo)}
								onOpen={setActiveOrder}
							/>
						))}
					</ul>
				)}

				{list.hasMore && (
					<div className="flex justify-center mt-5">
						<Button
							variant="bordered"
							color="default"
							onClick={() => void list.loadMore()}
							disabled={list.loadingMore}
							isLoading={list.loadingMore}
							loadingLabel="載入訂單中…"
						>
							載入更多
						</Button>
					</div>
				)}
			</SellerPageShell>

			<OrderQuickActionDrawer
				order={activeOrder}
				aftersale={activeAfterSale}
				afterSaleLoading={afterSales.loading}
				open={activeOrder !== null}
				onClose={() => setActiveOrder(null)}
				onMutated={() => {
					void list.refresh();
					void afterSales.refresh();
				}}
			/>
		</main>
	);
}

function OrderRow({
	order,
	aftersale,
	onOpen,
}: {
	order: Order;
	aftersale?: AfterSaleCase;
	onOpen: (order: Order) => void;
}) {
	const itemCount = order.items.reduce(
		(sum, it) => sum + Number(it.quantity ?? 0),
		0,
	);
	const cover = order.items[0]?.image;
	const coverSrc = cover ? resolveCdnUrl(cover) : "";
	const created = order.createdAt
		? new Date(
				order.createdAt < 1e12 ? order.createdAt * 1000 : order.createdAt,
			)
		: null;
	const statusLabel = STATUS_LABEL[order.status] || order.status;

	return (
		<li className="bg-content1 border border-divider rounded-hkui-lg transition-colors duration-200 hover:border-hkui-primary-200">
			<button
				type="button"
				onClick={() => onOpen(order)}
				className="flex w-full flex-col gap-3 p-4 text-left text-inherit no-underline cursor-pointer"
			>
				<div className="flex justify-between items-center">
					<strong className="text-base font-semibold">
						{order.sellerName || "未知賣家"}
					</strong>
					<span className="inline-flex items-center gap-2">
						{aftersale ? (
							<AfterSaleStatusBadge
								status={aftersale.status}
								appealState={aftersale.appealState}
								size="sm"
							/>
						) : null}
						<span
							className={`text-xs font-semibold px-2.5 py-1 rounded-full ${buyerOrderStatusBadgeClass(order.status)}`}
						>
							{statusLabel}
						</span>
					</span>
				</div>
				<div className="grid grid-cols-[64px_1fr_auto] gap-3 items-center">
					{coverSrc ? (
						<AppImage
							src={coverSrc}
							alt={order.items[0]?.name || ""}
							width={64}
							height={64}
							className="w-16 h-16 rounded-hkui-md object-cover bg-default-100"
							loading="eager"
							sizes="64px"
						/>
					) : (
						<span
							className="w-16 h-16 rounded-hkui-md bg-default-100"
							aria-hidden="true"
						/>
					)}
					<div className="flex flex-col gap-[2px] text-sm text-hk-body">
						{order.items.slice(0, 2).map((it) => (
							<span key={String(it.orderItemId)}>
								{it.name}
								<small className="text-foreground-soft"> × {it.quantity}</small>
							</span>
						))}
						{order.items.length > 2 && (
							<small className="text-foreground-soft">
								等 {order.items.length} 項商品
							</small>
						)}
					</div>
					<div className="text-right flex flex-col">
						<small className="text-xs text-foreground-soft">
							共 {itemCount} 件
						</small>
						<strong className="text-base text-hk-primary font-bold">
							NT$ {Number(order.totalActualAmount || 0).toLocaleString()}
						</strong>
					</div>
				</div>
				{order.status === "pending_payment" && order.paymentDeadline ? (
					<OrderPaymentCountdown
						deadline={order.paymentDeadline}
						className="inline-flex items-center gap-1 mt-1.5 text-xs font-semibold data-[expired=true]:text-foreground-soft data-[expired=true]:font-medium"
					/>
				) : null}
				{aftersale ? (
					<div className="rounded-hkui-md bg-content2 px-3 py-2 text-xs text-hk-body">
						<span className="font-semibold">售後案件 #{aftersale.caseNo}</span>
						<span className="text-foreground-soft">
							{" "}
							· 點開可查看退貨 / 退款進度
						</span>
					</div>
				) : null}
				<div className="flex justify-between text-xs text-foreground-soft">
					<span>{formatTaipeiDateTime(created)}</span>
					<span>#{order.orderNo}</span>
				</div>
			</button>
		</li>
	);
}

function EmptyOrders({
	status,
	statusLabel,
}: {
	status: string;
	statusLabel: string;
}) {
	return (
		<ScEmpty>
			<AccountEmptyStateIcon variant="orders" />
			<ScEmptyTitle>
				{status ? `${statusLabel}沒有訂單` : "還沒有任何訂單"}
			</ScEmptyTitle>
			<ScEmptyDesc>到 Hawkr 市集看看新鮮上架的小農好物吧。</ScEmptyDesc>
			<Button href="/market/products" size="lg" radius="full">
				去逛市集
			</Button>
		</ScEmpty>
	);
}
