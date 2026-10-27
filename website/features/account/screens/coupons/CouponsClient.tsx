"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import * as React from "react";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import { useAuth } from "@/lib/auth/context";
import {
	type CouponItem,
	fetchMyCoupons,
	type MyCouponItem,
} from "@/lib/coupons/repo";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { queryKeys } from "@/lib/query/keys";
import { sellerPath } from "@/lib/seo/urls";

// Note: 「全站可領」 tab is intentionally hidden — the backend's
// `/app/coupons/available` endpoint requires both `store_id`
// and `order_amount` (cart context) at runtime, despite OpenAPI marking
// them optional. There is no "list all platform coupons" endpoint
// today. Re-add the tab when backend ships one.
const TABS: Array<{ value: string; label: string }> = [
	{ value: "unused", label: "可使用" },
	{ value: "used", label: "已使用" },
	{ value: "expired", label: "已過期" },
];

// 秒／毫秒判別與台北時區位移都在 `toEpochMs` / `formatTaipeiDate` 裡。
const formatExpire = formatTaipeiDate;

type CouponKindVisual = {
	/** Coupon kind — drives the badge tint via the `badgeColor` lookup. */
	kind: "cash" | "percent";
	/** Big top line inside the badge — e.g. "9% OFF", "$50 OFF". */
	amountLabel: string;
	/** Small bottom line inside the badge — e.g. "優惠折扣券". */
	typeLabel: string;
};

function couponVisual(c: CouponItem): CouponKindVisual {
	const kind: CouponKindVisual["kind"] =
		c.kind === "percent" || c.type === "percent" ? "percent" : "cash";

	if (kind === "percent") {
		const pct = c.value ? Math.round(c.value) : 0;
		return {
			kind,
			amountLabel: pct ? `${pct}% OFF` : "折扣",
			typeLabel: "優惠折扣券",
		};
	}
	const amt = c.value ? Math.round(c.value) : 0;
	return {
		kind,
		amountLabel: amt ? `折 $${amt.toLocaleString()}` : "現金折抵",
		typeLabel: "現金折價券",
	};
}

export default function CouponsClient() {
	const { isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const [tab, setTab] = React.useState("unused");

	// Each tab is a different status filter → its own cache entry. Switching tabs
	// keeps the previous tab's rows on screen (keepPreviousData) until the new
	// page lands, instead of flashing the loading text.
	const query = useQuery({
		queryKey: queryKeys.buyer.coupons(identity, { status: tab, pageSize: 50 }),
		queryFn: () => fetchMyCoupons({ status: tab, pageSize: 50 }),
		enabled: isLoggedIn,
		placeholderData: keepPreviousData,
	});
	const list: Array<MyCouponItem | CouponItem> = query.data ?? [];
	const loading = query.isLoading;
	const error = query.error instanceof Error ? query.error.message : null;

	return (
		<main className="coupons-page">
			<SellerPageShell
				title="優惠券"
				subtitle="結帳時系統會自動套用最優惠的組合。"
				toolbar={
					<div className="flex flex-wrap gap-2" role="tablist">
						{TABS.map((t) => (
							<button
								key={t.value}
								type="button"
								role="tab"
								aria-selected={tab === t.value}
								className={[
									"inline-flex items-center gap-1.5 h-8 px-3.5 border border-divider rounded-full text-sm font-bold cursor-pointer transition-colors duration-150 ease-out",
									tab === t.value
										? "bg-hkui-primary text-[var(--color-neutral-cream)] border-transparent"
										: "bg-content1 text-foreground-muted hover:bg-content2 hover:text-foreground",
								]
									.filter(Boolean)
									.join(" ")}
								onClick={() => setTab(t.value)}
							>
								{t.label}
							</button>
						))}
					</div>
				}
			>
				{error && (
					<p className="text-danger-700 text-sm font-semibold" role="alert">
						{error}
					</p>
				)}

				{loading && list.length === 0 ? (
					<p className="text-foreground-soft text-sm">載入中…</p>
				) : list.length === 0 ? (
					<div className="flex flex-col items-center justify-center gap-3 py-8 px-5 text-center text-foreground-soft">
						<AccountEmptyStateIcon variant="coupons" />
						<p className="text-base font-extrabold text-foreground">
							這個分類沒有優惠券
						</p>
						<p className="max-w-[32ch] text-sm leading-relaxed">
							市集首頁與商家頁會不定期推出優惠券，可以追蹤喜歡的小農。
						</p>
					</div>
				) : (
					<ul className="flex flex-col gap-3">
						{list.map((c) => {
							const my = c as MyCouponItem;
							const status = my.status;
							const isUsed = status === "used" || status === "expired";
							const visual = couponVisual(c);
							const expiry = c.expires_at
								? `有效期限 ${formatExpire(c.expires_at)}`
								: "";
							const threshold = c.min_spend
								? `滿 NT$${Number(c.min_spend).toLocaleString()} 可用`
								: "";

							// Backend uses `applicableStoreId` (camelCase) for the
							// normalized record; `seller_name` (snake) for the
							// pre-normalization legacy alias. Prefer the camelCase one
							// since `normalizeCoupon` writes it explicitly.
							const sellerName = c.sellerName ?? c.seller_name;
							// 賣家頁 canonical 網址走 store_code；applicableStoreId 只是
							// 內部代理鍵，連它每次點擊都要多吃一次 308。coupon-service
							// 為了 sellerName 本來就查了賣家，兩者同源，所以取不到
							// store_code（全平台券／賣家服務降級）就不連結。
							const sellerHref = sellerPath(c.storeCode) || null;

							// Per-kind badge tint (was the CSS `data-kind` attribute); a
							// used/expired coupon overrides every kind to the muted chip.
							// Per-kind badge tint (was the CSS `data-kind` attribute); a
							// used/expired coupon overrides every kind to the muted chip.
							const badgeColor = isUsed
								? "bg-default-100 text-default-600 dark:bg-hk-ink-100/8 dark:text-hk-ink-100/60"
								: visual.kind === "cash"
									? "bg-hkui-primary-50 text-hkui-primary-700 dark:bg-hk-ink-300/18 dark:text-hk-ink-200"
									: "bg-warning-50 text-warning-700 dark:bg-[#f1cd70]/16 dark:text-[#f1cd70]";
							const statusColor =
								status === "used"
									? "bg-default-100 text-default-600 dark:bg-hk-ink-100/8 dark:text-hk-ink-100/60"
									: status === "expired"
										? "bg-danger-50 text-danger-700 dark:bg-[#da6e6e]/16 dark:text-[#e6a3a3]"
										: "bg-success-50 text-success-700 dark:bg-hk-ink-300/18 dark:text-hk-ink-200";

							return (
								<li
									key={String(c.id)}
									className={`grid grid-cols-[112px_1fr_auto] [grid-template-areas:'badge_info_aside'] gap-4 items-center max-[600px]:grid-cols-[96px_1fr] max-[600px]:[grid-template-areas:'badge_info'_'badge_aside'] bg-content1 dark:bg-white/4 border border-divider dark:border-[#f5dea5]/14 rounded-hkui-lg p-4 transition-colors hover:border-hkui-primary-200${isUsed ? " opacity-60" : ""}`}
								>
									<div
										className={`[grid-area:badge] flex flex-col items-center justify-center gap-1 w-28 h-22 max-[600px]:w-24 max-[600px]:h-20 rounded-hkui-md py-2 px-3 text-center ${badgeColor}`}
										aria-hidden="true"
									>
										<span className="font-display text-lg font-bold tracking-[-0.02em] leading-[1.1]">
											{visual.amountLabel}
										</span>
										<span className="text-xs font-semibold opacity-90">
											{visual.typeLabel}
										</span>
									</div>

									<div className="[grid-area:info] flex flex-col gap-0.5 min-w-0">
										{sellerName ? (
											sellerHref ? (
												<Link
													href={sellerHref}
													className="group inline-flex items-center gap-1.5 text-xs no-underline max-w-full min-w-0 mb-0.5 text-hkui-primary-700 dark:text-hk-ink-200 cursor-pointer"
													aria-label={`查看 ${sellerName} 的店面`}
												>
													<span className="inline-flex items-center justify-center w-5 h-5 rounded-full overflow-hidden bg-default-100 dark:bg-hk-ink-100/8 shrink-0">
														<Avatar
															className="w-full h-full object-cover"
															src={c.sellerAvatarUrl}
															name={sellerName}
															alt=""
															entity="seller"
															fallback="empty"
														/>
													</span>
													<span className="truncate min-w-0 group-hover:underline">
														{sellerName}
													</span>
												</Link>
											) : (
												<span className="inline-flex items-center gap-1.5 text-xs text-foreground-soft dark:text-hk-ink-100/70 no-underline max-w-full min-w-0 mb-0.5">
													<span className="inline-flex items-center justify-center w-5 h-5 rounded-full overflow-hidden bg-default-100 dark:bg-hk-ink-100/8 shrink-0">
														<Avatar
															className="w-full h-full object-cover"
															src={c.sellerAvatarUrl}
															name={sellerName}
															alt=""
															entity="seller"
															fallback="empty"
														/>
													</span>
													<span className="truncate min-w-0 group-hover:underline">
														{sellerName}
													</span>
												</span>
											)
										) : (
											<span className="inline-flex items-center gap-1.5 text-xs text-foreground-soft dark:text-hk-ink-100/70 no-underline max-w-full min-w-0 mb-0.5">
												<span className="inline-flex items-center justify-center w-5 h-5 rounded-full overflow-hidden shrink-0 bg-hkui-primary text-hkui-primary-foreground dark:bg-hk-ink-300/34 dark:text-[#f5e9c3] font-display text-[0.6875rem] font-bold leading-none">
													日
												</span>
												<span className="truncate min-w-0 group-hover:underline">
													Hawkr · 全平台券
												</span>
											</span>
										)}
										<strong className="text-base font-bold text-foreground dark:text-[#f5e9c3] truncate">
											{c.name}
										</strong>
										{threshold && (
											<span className="text-xs text-foreground-soft dark:text-hk-ink-100/70 truncate">
												{threshold}
											</span>
										)}
										{expiry && (
											<span className="text-xs text-foreground-soft dark:text-hk-ink-100/70 truncate">
												{expiry}
											</span>
										)}
									</div>

									<div className="[grid-area:aside] flex flex-col items-end gap-2 shrink-0 max-[600px]:flex-row max-[600px]:items-center max-[600px]:justify-end">
										<span
											className={`text-xs font-semibold px-2.5 py-0.5 rounded-full ${statusColor}`}
										>
											{status === "used"
												? "已使用"
												: status === "expired"
													? "已過期"
													: "可使用"}
										</span>
										{isUsed ? (
											<span className="text-sm text-foreground-soft dark:text-hk-ink-100/55">
												查看詳情
											</span>
										) : (
											// "unused" + transient states like "locked" all route to
											// the market — buyer can spend the coupon as long as it's
											// not in a terminal used/expired state.
											<Link
												href="/market/products"
												className="text-sm font-semibold text-hkui-primary dark:text-hk-ink-200 no-underline hover:text-hkui-primary-700 dark:hover:text-hk-ink-100 hover:underline"
											>
												去使用
											</Link>
										)}
									</div>
								</li>
							);
						})}
					</ul>
				)}
			</SellerPageShell>
		</main>
	);
}
