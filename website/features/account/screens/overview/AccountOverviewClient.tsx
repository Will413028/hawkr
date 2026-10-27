"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import Link from "@/components/shared/Link";
import MeasuresAccountEntry from "@/features/seller/measures/MeasuresAccountEntry";
import { useAuth } from "@/lib/auth/context";
import { formatTaipeiMonthDay as formatShortDate } from "@/lib/date/taipei";
import { displayableFavorites } from "@/lib/favorites/visible";
import {
	useCoupons,
	useFavorites,
	useFollowing,
	useMemberCards,
} from "@/lib/hooks/useAccount";
import { useOrderList, useOrderStatusCounts } from "@/lib/hooks/useOrders";
import { imageUri } from "@/lib/media/image";
import type { Order } from "@/lib/orders/types";
import { sellerPath } from "@/lib/seo/urls";

/**
 * Account home — buyer personalized overview.
 *
 * Three big blocks:
 *   1. Stat strip — consumption snapshot (orders / spend / AOV / coupons /
 *      member cards / favorites / following). Each card links to the relevant
 *      detail surface.
 *   2. Two-column panel:
 *      - 訂單待辦: accurate per-status counts from `useOrderStatusCounts`
 *        (one /app/orders call per bucket, page-size 1 → cheap totals).
 *      - 消費洞察: period-switched aggregates derived locally from the
 *        loaded order page — top sellers, top categories, last purchase,
 *        and an honest "this page" subtotal disclaimer.
 *   3. 收藏與回購: horizontal scroll of favorite products with quick links
 *      back to the PDP for one-tap re-buy.
 *
 * Notes:
 *   - All aggregates run client-side off existing list hooks; no new
 *     backend endpoints required. Trade-off is the totals are capped at
 *     the order list page (20 by default) — good enough for "recent
 *     activity" overview without a dedicated /dashboard aggregator.
 *   - Period switcher operates on the same loaded page, filtered by
 *     `createdAt`. Switching periods doesn't refetch.
 */

const PERIOD_OPTIONS = [
	{ key: "30d", label: "近 30 天", days: 30 },
	{ key: "90d", label: "近 90 天", days: 90 },
	{ key: "ytd", label: "今年", days: null as number | null },
	{ key: "all", label: "全部", days: null as number | null },
] as const;

type PeriodKey = (typeof PERIOD_OPTIONS)[number]["key"];

// Status buckets surfaced as 訂單待辦. Each entry maps to one fetchOrders
// call via useOrderStatusCounts (CSV string with comma-separated statuses).
const TODO_BUCKETS = [
	{
		key: "pending_payment",
		label: "待付款",
		statusCsv: "pending_payment",
		href: "/account/orders?status=pending_payment",
		icon: "wallet",
	},
	{
		key: "preparing",
		label: "待出貨",
		statusCsv: "paid,preparing",
		href: "/account/orders?status=preparing",
		icon: "box",
	},
	{
		key: "shipping",
		label: "待取貨",
		statusCsv: "shipping,waiting_pickup",
		href: "/account/orders?status=shipping",
		icon: "truck",
	},
	{
		key: "review",
		label: "待評價",
		// `delivered` = arrived but not yet confirmed; treated as
		// review-eligible alongside `completed` because Hawkr's
		// auto-confirm window still lets the buyer review.
		statusCsv: "delivered,completed",
		href: "/account/orders?status=completed",
		icon: "chat",
	},
	{
		key: "refunding",
		label: "退款中",
		statusCsv: "after_sale",
		href: "/account/orders?status=cancelled",
		icon: "coin",
	},
] as const;

// Pre-computed at module scope so `useOrderStatusCounts` array identity
// stays stable across renders (the hook keys its effect on identity).
const TODO_STATUS_CSVS = TODO_BUCKETS.map((b) => b.statusCsv);

// Inline Tailwind class sets reused by the overview's repeated rows. Migrated
// from .account-stats__card / .account-todos__all / .account-insights__empty
// in styles/features/account.css. Dark mode on the stat cards mirrors the
// .account-shell .seller-card frosted-glass group (white/4 + backdrop blur).
const STAT_CARD =
	"flex flex-col gap-1 py-[18px] px-5 rounded-[18px] bg-content1 border border-divider text-inherit no-underline shadow-[0_4px_16px_rgb(var(--color-ink-950-rgb)/0.04)] transition-[transform,box-shadow,border-color] duration-[160ms] hover:-translate-y-0.5 hover:border-[rgb(var(--color-ink-400-rgb)/0.4)] hover:shadow-[0_10px_28px_rgb(var(--color-ink-950-rgb)/0.08)] dark:bg-white/4 dark:backdrop-blur-[12px] dark:backdrop-saturate-[1.2] dark:border-[rgba(245,222,165,0.12)] dark:shadow-[inset_0_1px_0_rgba(245,222,165,0.1),0_12px_28px_rgba(0,0,0,0.32)] dark:text-foreground dark:hover:border-[rgba(241,205,112,0.4)]";
const STAT_CARD_LABEL =
	"text-xs text-foreground-soft font-semibold tracking-[0.04em]";
const STAT_CARD_VALUE =
	"font-display text-[length:clamp(22px,2.4vw,30px)] font-bold tracking-[var(--hkui-tracking-tight)] text-foreground leading-[1.1] dark:text-[var(--hk-text-strong,#f5e9c3)]";
const STAT_CARD_HINT = "text-xs text-foreground-soft leading-[1.4]";
const OVERVIEW_LINK =
	"ml-auto text-sm text-foreground-soft no-underline transition-colors duration-[120ms] hover:text-hkui-primary";
const INSIGHTS_EMPTY = "m-0 text-sm text-foreground-soft";

function formatTwd(amount: number): string {
	if (!Number.isFinite(amount)) return "NT$ 0";
	return `NT$ ${amount.toLocaleString("zh-TW", {
		minimumFractionDigits: 0,
		maximumFractionDigits: 0,
	})}`;
}

// Order.createdAt ships as a unix timestamp in seconds OR milliseconds depending
// on the legacy row. Normalise to ms for consistent date math.
function toMs(ts: number | null | undefined): number {
	if (!ts) return 0;
	return ts > 1e12 ? ts : ts * 1000;
}

function withinPeriod(order: Order, period: PeriodKey): boolean {
	if (period === "all") return true;
	const created = toMs(order.createdAt);
	if (!created) return false;
	if (period === "ytd") {
		const start = new Date(new Date().getFullYear(), 0, 1).getTime();
		return created >= start;
	}
	const days = PERIOD_OPTIONS.find((p) => p.key === period)?.days ?? 0;
	const cutoff = Date.now() - days * 24 * 60 * 60 * 1000;
	return created >= cutoff;
}

function isUsableOrder(o: Order): boolean {
	return o.status !== "cancelled" && o.status !== "refunded";
}

function orderAmount(o: Order): number {
	return Number(o.totalActualAmount ?? o.totalOriginalAmount ?? 0);
}

interface RankedEntry {
	id: string;
	/**
	 * 賣家店面 canonical 短碼；沒有就不連結（不回退 id — id 路徑多吃一次 308）。
	 * 常買商品那份 ranking 沒有這個概念，所以是選填。
	 */
	storeCode?: string;
	name: string;
	count: number;
	spend: number;
	/** Max `createdAt` (ms) of contributing orders — drives "上次 X 月 X 日". */
	lastOrderMs: number;
}

/**
 * Rank sellers/items by frequency over an order list. Returns descending
 * order; ties broken by total spend. Used for "Top 3 常買賣家 / 常買商品".
 */
function rankSellers(orders: Order[]): RankedEntry[] {
	const map = new Map<string, RankedEntry>();
	for (const o of orders) {
		if (!isUsableOrder(o)) continue;
		const id = String(o.storeId || "");
		if (!id) continue;
		const createdMs = toMs(o.createdAt);
		const existing = map.get(id);
		if (existing) {
			existing.count += 1;
			existing.spend += orderAmount(o);
			if (createdMs > existing.lastOrderMs) existing.lastOrderMs = createdMs;
		} else {
			map.set(id, {
				id,
				storeCode: o.storeCode,
				name: o.sellerName || "賣家",
				count: 1,
				spend: orderAmount(o),
				lastOrderMs: createdMs,
			});
		}
	}
	return [...map.values()].sort(
		(a, b) => b.count - a.count || b.spend - a.spend,
	);
}

function rankProducts(orders: Order[]): RankedEntry[] {
	const map = new Map<string, RankedEntry>();
	for (const o of orders) {
		if (!isUsableOrder(o)) continue;
		const createdMs = toMs(o.createdAt);
		for (const item of o.items ?? []) {
			const id = String(item.productId || "");
			if (!id) continue;
			const qty = Number(item.quantity ?? 1) || 1;
			const subtotal =
				Number(item.subtotal ?? 0) || qty * Number(item.price ?? 0);
			const existing = map.get(id);
			if (existing) {
				existing.count += qty;
				existing.spend += subtotal;
				if (createdMs > existing.lastOrderMs) existing.lastOrderMs = createdMs;
			} else {
				map.set(id, {
					id,
					name: item.name || "商品",
					count: qty,
					spend: subtotal,
					lastOrderMs: createdMs,
				});
			}
		}
	}
	return [...map.values()].sort(
		(a, b) => b.count - a.count || b.spend - a.spend,
	);
}

interface QuickRangeProps {
	period: PeriodKey;
	onChange: (next: PeriodKey) => void;
}

function PeriodSwitcher({ period, onChange }: QuickRangeProps) {
	return (
		<div
			className="inline-flex gap-0.5 p-[3px] rounded-full bg-[rgb(var(--color-ink-400-rgb)/0.08)] dark:bg-[rgba(245,222,165,0.08)]"
			role="tablist"
			aria-label="期間切換"
		>
			{PERIOD_OPTIONS.map((p) => (
				<button
					key={p.key}
					type="button"
					role="tab"
					aria-selected={period === p.key}
					data-active={period === p.key || undefined}
					onClick={() => onChange(p.key)}
					className="border-0 bg-transparent py-1.5 px-3.5 rounded-full text-sm text-foreground-soft cursor-pointer transition-[background,color] duration-[120ms] hover:text-foreground data-[active=true]:bg-hkui-primary data-[active=true]:text-hkui-primary-foreground data-[active=true]:font-semibold dark:text-[rgb(var(--color-ink-100-rgb)/0.7)] dark:data-[active=true]:bg-[var(--color-brand-secondary)] dark:data-[active=true]:text-[var(--color-neutral-ink)]"
				>
					{p.label}
				</button>
			))}
		</div>
	);
}

export default function AccountOverviewClient() {
	const { identity } = useAuth();
	// TODO(B-189): profile endpoint — buyerName not on Identity; use nickname fallback.
	const displayName = identity?.nickname || "會員";

	const orders = useOrderList();
	const coupons = useCoupons();
	const memberCards = useMemberCards();
	const favorites = useFavorites();
	const following = useFollowing();
	const todoCounts = useOrderStatusCounts(TODO_STATUS_CSVS);

	// Read by BOTH the 收藏商品 stat card and the 收藏與回購 rail. They used to
	// filter differently, which is the whole of #377's fourth item — see
	// lib/favorites/visible.ts.
	const visibleFavorites = React.useMemo(
		() => displayableFavorites(favorites.items),
		[favorites.items],
	);

	const [period, setPeriod] = React.useState<PeriodKey>("90d");

	const periodOrders = React.useMemo(
		() => orders.orders.filter((o) => withinPeriod(o, period)),
		[orders.orders, period],
	);

	const { totalSpent, totalOrders, aov } = React.useMemo(() => {
		const usable = orders.orders.filter(isUsableOrder);
		const sum = usable.reduce((acc, o) => acc + orderAmount(o), 0);
		const count = orders.total || usable.length;
		// AOV uses the loaded-page sum / loaded-page count rather than the
		// backend `total` — mixing the two would understate AOV when the
		// list is paginated and we only know per-page spend.
		const aov = usable.length > 0 ? Math.round(sum / usable.length) : 0;
		return { totalSpent: sum, totalOrders: count, aov };
	}, [orders.orders, orders.total]);

	const periodSpent = React.useMemo(
		() =>
			periodOrders
				.filter(isUsableOrder)
				.reduce((acc, o) => acc + orderAmount(o), 0),
		[periodOrders],
	);

	// Sum of `order.discount` across usable orders in the period. Backend
	// returns this as a string (NT$ amount, often "0" or "0.00" when no
	// coupon/promo applied); ignore missing or non-finite values silently.
	const periodSaved = React.useMemo(
		() =>
			periodOrders.filter(isUsableOrder).reduce((acc, o) => {
				const v = Number(o.discount ?? 0);
				return Number.isFinite(v) ? acc + v : acc;
			}, 0),
		[periodOrders],
	);

	const topSellers = React.useMemo(
		() => rankSellers(periodOrders).slice(0, 3),
		[periodOrders],
	);
	const topProducts = React.useMemo(
		() => rankProducts(periodOrders).slice(0, 3),
		[periodOrders],
	);

	const lastOrder = React.useMemo(() => {
		const usable = orders.orders.filter(isUsableOrder);
		return usable.length > 0
			? usable.reduce((latest, o) =>
					toMs(o.createdAt) > toMs(latest.createdAt) ? o : latest,
				)
			: null;
	}, [orders.orders]);

	const usableCoupons = React.useMemo(
		() => coupons.my.filter((c) => c.status === "unused").length,
		[coupons.my],
	);

	const loadingStats = orders.loading && orders.orders.length === 0;

	return (
		<main className="flex flex-col gap-7 max-w-[920px]">
			<header>
				<h1 className="m-0 mb-1.5 text-[length:clamp(24px,2.2vw,32px)] font-bold tracking-[0.01em] text-foreground inline-flex items-center gap-2 dark:text-[var(--hk-text-strong,#f5e9c3)]">
					嗨，{displayName}{" "}
					<span
						className="inline-flex w-[1.2em] h-[1.2em] translate-y-0.5 [&>*]:w-full [&>*]:h-full"
						aria-hidden="true"
					>
						<AppIcon name="user" size={40} />
					</span>
				</h1>
				<p className="m-0 text-foreground-soft text-sm">
					消費紀錄、優惠券、追蹤的賣家動態都在這裡，一眼掌握。
				</p>
			</header>

			<MeasuresAccountEntry />

			<section
				className="grid grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-3.5 transition-opacity duration-[160ms] data-[loading=true]:opacity-[0.55]"
				aria-label="個人消費統計"
				data-loading={loadingStats ? "true" : undefined}
			>
				<Link href="/account/orders" className={STAT_CARD}>
					<small className={STAT_CARD_LABEL}>累計訂單</small>
					<strong className={STAT_CARD_VALUE}>
						{totalOrders.toLocaleString()}
					</strong>
					<span className={STAT_CARD_HINT}>有效訂單筆數</span>
				</Link>
				<Link href="/account/orders" className={STAT_CARD}>
					<small className={STAT_CARD_LABEL}>累計消費</small>
					<strong className={STAT_CARD_VALUE}>{formatTwd(totalSpent)}</strong>
					<span className={STAT_CARD_HINT}>不含已取消 / 已退款</span>
				</Link>
				<Link href="/account/orders" className={STAT_CARD}>
					<small className={STAT_CARD_LABEL}>平均客單</small>
					<strong className={STAT_CARD_VALUE}>{formatTwd(aov)}</strong>
					<span className={STAT_CARD_HINT}>近期單筆均值</span>
				</Link>
				<Link href="/account/coupons" className={STAT_CARD}>
					<small className={STAT_CARD_LABEL}>可用優惠券</small>
					<strong className={STAT_CARD_VALUE}>
						{usableCoupons.toLocaleString()}
					</strong>
					<span className={STAT_CARD_HINT}>{coupons.my.length} 張總計</span>
				</Link>
				<Link href="/account/wallet" className={STAT_CARD}>
					<small className={STAT_CARD_LABEL}>會員卡</small>
					<strong className={STAT_CARD_VALUE}>
						{memberCards.cards.length.toLocaleString()}
					</strong>
					<span className={STAT_CARD_HINT}>消費過的商家</span>
				</Link>
				<Link href="/account/favorites" className={STAT_CARD}>
					<small className={STAT_CARD_LABEL}>收藏商品</small>
					<strong className={STAT_CARD_VALUE}>
						{visibleFavorites.length.toLocaleString()}
					</strong>
					<span className={STAT_CARD_HINT}>常買回購靈感</span>
				</Link>
				<Link href="/account/recent" className={STAT_CARD}>
					<small className={STAT_CARD_LABEL}>追蹤賣家</small>
					<strong className={STAT_CARD_VALUE}>
						{following.items.length.toLocaleString()}
					</strong>
					<span className={STAT_CARD_HINT}>新動態會在下方更新</span>
				</Link>
			</section>

			<div className="grid grid-cols-[minmax(0,0.9fr)_minmax(0,1.3fr)] gap-5 items-stretch max-[960px]:grid-cols-1">
				<section
					className="seller-card flex flex-col gap-3"
					aria-label="訂單待辦"
				>
					<header className="seller-card__head">
						<h2>訂單待辦</h2>
						<span className="flex items-center gap-3">
							<Link href="/account/after-sales" className={OVERVIEW_LINK}>
								售後案件 ›
							</Link>
							<Link href="/account/orders" className={OVERVIEW_LINK}>
								查看全部訂單 ›
							</Link>
						</span>
					</header>
					<ul className="list-none m-0 p-0 flex flex-col gap-1">
						{TODO_BUCKETS.map((bucket) => {
							const count = todoCounts.counts[bucket.statusCsv] ?? 0;
							return (
								<li key={bucket.key}>
									<Link
										href={bucket.href}
										className="grid grid-cols-[36px_1fr_auto_auto] items-center gap-3 py-3 px-3.5 rounded-[14px] no-underline text-inherit transition-[background,transform] duration-[120ms] hover:bg-[rgb(var(--color-ink-400-rgb)/0.08)] dark:hover:bg-[rgba(245,222,165,0.08)]"
									>
										<span
											className="inline-grid place-items-center w-9 h-9 rounded-xl bg-[rgb(var(--color-ink-400-rgb)/0.12)] text-[var(--color-primary,var(--color-ink-500))] dark:bg-[rgba(245,222,165,0.12)] dark:text-[#f5dea5]"
											aria-hidden="true"
										>
											<AppIcon name={bucket.icon} size={20} />
										</span>
										<span className="text-base font-semibold text-foreground">
											{bucket.label}
										</span>
										<span
											className="font-display font-bold text-lg text-hkui-primary min-w-[28px] text-right data-[empty=true]:text-foreground-muted data-[empty=true]:font-medium dark:text-[#f5dea5]"
											data-empty={count === 0 || undefined}
										>
											{count.toLocaleString()}
										</span>
										<span
											className="text-foreground-muted text-lg"
											aria-hidden="true"
										>
											›
										</span>
									</Link>
								</li>
							);
						})}
					</ul>
					{orders.error ? (
						<p className="text-danger-700 text-sm font-semibold" role="alert">
							{orders.error}
						</p>
					) : null}
				</section>

				<section className="seller-card" aria-label="消費洞察">
					<header className="seller-card__head flex-wrap gap-3">
						<h2>消費洞察</h2>
						<PeriodSwitcher period={period} onChange={setPeriod} />
					</header>

					<div className="grid grid-cols-4 gap-3.5 mt-4 p-3.5 rounded-[14px] bg-[rgb(var(--color-ink-400-rgb)/0.06)] border border-divider max-[600px]:grid-cols-2 dark:bg-[rgba(245,222,165,0.06)] dark:border-[rgba(245,222,165,0.14)]">
						<div className="flex flex-col gap-1">
							<small className={STAT_CARD_LABEL}>期間消費</small>
							<strong className="font-display text-[length:clamp(18px,2vw,22px)] font-bold text-foreground whitespace-nowrap">
								{formatTwd(periodSpent)}
							</strong>
						</div>
						<div className="flex flex-col items-end gap-1 text-right max-[600px]:items-start max-[600px]:text-left">
							<small className={STAT_CARD_LABEL}>期間訂單</small>
							<strong className="font-display text-[length:clamp(18px,2vw,22px)] font-bold text-foreground whitespace-nowrap">
								{periodOrders.filter(isUsableOrder).length.toLocaleString()}
							</strong>
						</div>
						<div className="flex flex-col gap-1">
							<small className={STAT_CARD_LABEL}>期間省下</small>
							<strong className="font-display text-[length:clamp(18px,2vw,22px)] font-bold text-foreground whitespace-nowrap">
								{formatTwd(periodSaved)}
							</strong>
						</div>
						<div className="flex flex-col gap-1">
							<small className={STAT_CARD_LABEL}>最近一次消費</small>
							<strong className="font-display text-[length:clamp(18px,2vw,22px)] font-bold text-foreground whitespace-nowrap">
								{lastOrder ? formatShortDate(toMs(lastOrder.createdAt)) : "—"}
							</strong>
						</div>
					</div>

					{/* Honest disclosure: the period stats are computed off the
					    loaded order page (default 20). When the buyer has more
					    historical orders, "今年" / "全部" tabs would otherwise
					    appear truncated without explanation. */}
					{orders.total > orders.orders.length ? (
						<p className="mt-2 mx-1 mb-0 text-xs text-foreground-soft">
							分析最近 {orders.orders.length.toLocaleString()} 筆訂單（共{" "}
							{orders.total.toLocaleString()} 筆）。
						</p>
					) : null}

					<div className="grid grid-cols-2 gap-4 mt-3 max-[600px]:grid-cols-1">
						<div>
							<h3 className="text-sm font-semibold text-foreground-soft m-0 mb-2 tracking-[0.03em]">
								常買賣家 Top 3
							</h3>
							{topSellers.length === 0 ? (
								<p className={INSIGHTS_EMPTY}>這段期間還沒有交易。</p>
							) : (
								<ol className="list-none m-0 p-0 flex flex-col gap-1.5">
									{topSellers.map((s, idx) => {
										// 賣家頁 canonical 網址走 store_code；訂單本來就帶著它，
										// 連 storeId 只是多吃一次 308。取不到就不連結。
										const href = sellerPath(s.storeCode);
										const body = (
											<>
												<strong className="text-sm font-semibold text-foreground overflow-hidden text-ellipsis whitespace-nowrap">
													{s.name}
												</strong>
												<small className="text-[length:clamp(9px,calc(6px_+_2.5cqw),var(--text-xs,12px))] text-foreground-soft overflow-hidden text-ellipsis whitespace-nowrap">
													{s.count} 筆 ・ {formatTwd(s.spend)}
													{s.lastOrderMs
														? ` ・ 上次 ${formatShortDate(s.lastOrderMs)}`
														: ""}
												</small>
											</>
										);
										return (
											<li
												key={s.id}
												className="grid grid-cols-[28px_1fr] items-center gap-2.5"
											>
												<span className="inline-grid place-items-center w-6 h-6 rounded-lg bg-[rgba(241,205,112,0.22)] text-[color:var(--color-brand-ink)] text-xs font-bold dark:text-[#ffeec0]">
													{idx + 1}
												</span>
												{href ? (
													<Link
														href={href}
														className="flex flex-col gap-0.5 no-underline text-inherit min-w-0 [container-type:inline-size] [&:hover_strong]:text-hkui-primary"
													>
														{body}
													</Link>
												) : (
													<span className="flex flex-col gap-0.5 min-w-0 [container-type:inline-size]">
														{body}
													</span>
												)}
											</li>
										);
									})}
								</ol>
							)}
						</div>

						<div>
							<h3 className="text-sm font-semibold text-foreground-soft m-0 mb-2 tracking-[0.03em]">
								常買商品 Top 3
							</h3>
							{topProducts.length === 0 ? (
								<p className={INSIGHTS_EMPTY}>這段期間還沒有交易。</p>
							) : (
								<ol className="list-none m-0 p-0 flex flex-col gap-1.5">
									{topProducts.map((p, idx) => (
										<li
											key={p.id}
											className="grid grid-cols-[28px_1fr] items-center gap-2.5"
										>
											<span className="inline-grid place-items-center w-6 h-6 rounded-lg bg-[rgba(241,205,112,0.22)] text-[color:var(--color-brand-ink)] text-xs font-bold dark:text-[#ffeec0]">
												{idx + 1}
											</span>
											<Link
												href={`/products/${p.id}`}
												className="flex flex-col gap-0.5 no-underline text-inherit min-w-0 [container-type:inline-size] [&:hover_strong]:text-hkui-primary"
											>
												<strong className="text-sm font-semibold text-foreground overflow-hidden text-ellipsis whitespace-nowrap">
													{p.name}
												</strong>
												<small className="text-[length:clamp(9px,calc(6px_+_2.5cqw),var(--text-xs,12px))] text-foreground-soft overflow-hidden text-ellipsis whitespace-nowrap">
													{p.count} 件 ・ {formatTwd(p.spend)}
													{p.lastOrderMs
														? ` ・ 上次 ${formatShortDate(p.lastOrderMs)}`
														: ""}
												</small>
											</Link>
										</li>
									))}
								</ol>
							)}
						</div>
					</div>
				</section>
			</div>

			<section className="seller-card" aria-label="收藏與回購">
				<header className="seller-card__head">
					<h2>收藏與回購</h2>
					<Link href="/account/favorites" className={OVERVIEW_LINK}>
						全部收藏 ›
					</Link>
				</header>
				{(() => {
					// visibleFavorites is hoisted so the stat card above counts exactly
					// what this rail renders — see its definition for why.
					return visibleFavorites.length === 0 ? (
						favorites.items.length === 0 ? (
							<p className={INSIGHTS_EMPTY}>
								還沒有收藏商品。逛逛{" "}
								<Link
									href="/market/products"
									className="text-hkui-primary underline underline-offset-[3px]"
								>
									市集商品
								</Link>{" "}
								把喜歡的加進收藏，下次回購一鍵搞定。
							</p>
						) : null
					) : (
						<ul className="list-none m-0 pt-1 px-1 pb-2 grid grid-flow-col auto-cols-[minmax(180px,200px)] gap-3 overflow-x-auto snap-x snap-proximity [scrollbar-width:thin]">
							{visibleFavorites.slice(0, 12).map((fav) => {
								const product = fav.product;
								if (!product) return null;
								const productImage = imageUri(product.image, "card");
								const priceMin = Number(product.minPrice);
								const priceMax = Number(product.maxPrice);
								const priceLabel =
									priceMin && priceMax && priceMin !== priceMax
										? `NT$ ${priceMin.toLocaleString()} – ${priceMax.toLocaleString()}`
										: priceMin
											? `NT$ ${priceMin.toLocaleString()}`
											: "";
								return (
									<li key={product.id} className="snap-start">
										<Link
											href={`/products/${product.id}`}
											className="flex flex-col gap-2 p-2.5 rounded-[14px] border border-divider bg-content1 no-underline text-inherit transition-[transform,border-color,box-shadow] duration-[160ms] hover:-translate-y-0.5 hover:border-[rgb(var(--color-ink-400-rgb)/0.4)] hover:shadow-[0_10px_24px_rgb(var(--color-ink-950-rgb)/0.08)] dark:hover:border-[rgba(241,205,112,0.36)] dark:hover:shadow-[0_10px_24px_rgba(0,0,0,0.36)]"
										>
											<span className="block w-full aspect-square overflow-hidden rounded-[10px] bg-[rgb(var(--color-ink-400-rgb)/0.08)]">
												{productImage ? (
													<AppImage
														src={productImage}
														alt={product.name}
														width={120}
														height={120}
														unoptimized
														className="w-full h-full object-cover"
													/>
												) : null}
											</span>
											<span className="flex flex-col gap-0.5 min-w-0">
												<strong className="text-sm font-semibold text-foreground line-clamp-2">
													{product.name}
												</strong>
												{product.store?.name ? (
													<small className="text-xs text-foreground-soft overflow-hidden text-ellipsis whitespace-nowrap">
														{product.store.name}
													</small>
												) : null}
												{priceLabel ? (
													<span className="mt-0.5 text-sm font-bold text-hkui-primary dark:text-[#f5dea5]">
														{priceLabel}
													</span>
												) : null}
											</span>
										</Link>
									</li>
								);
							})}
						</ul>
					);
				})()}
			</section>
		</main>
	);
}
