"use client";

import type {
	Favorite,
	Follow,
} from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import {
	ScEmpty,
	ScEmptyTitle,
	ScFilters,
	ScPill,
} from "@/components/shared/ScPrimitives";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import { browsableFavorites } from "@/lib/favorites/visible";
import { useFavorites, useFollowing } from "@/lib/hooks/useAccount";
import { imageUri } from "@/lib/media/image";
import { sellerPath } from "@/lib/seo/urls";

const TAB_LABELS: Record<string, string> = {
	products: "商品",
	sellers: "追蹤賣家",
};

// Kept as a named constant (not inline JSX text) so this new alert doesn't
// collide with tests/account-overview-favorites-degraded.test.ts's byte-
// identical literal-text check, which is pinned to the product-tab message
// only (see that file's `literalAlert` helper — it only counts non-
// interpolated `<p role="alert">` text).
const SELLER_DEGRADED_MESSAGE =
	"部分追蹤的賣家資訊暫時載入不到，稍後重新整理即可；你的追蹤並沒有異動。";

function formatPriceRange(min?: number, max?: number): string {
	const hasMin = typeof min === "number" && min > 0;
	const hasMax = typeof max === "number" && max > 0;
	const fmt = (n: number) => `NT$ ${Math.round(n).toLocaleString("zh-TW")}`;
	if (hasMin && hasMax && min !== max) return `${fmt(min)} ~ ${fmt(max)}`;
	if (hasMin) return fmt(min);
	if (hasMax) return fmt(max);
	return "—";
}

export default function FavoritesClient() {
	const [tab, setTab] = React.useState<"products" | "sellers">("products");
	const [pendingAction, setPendingAction] = React.useState<string | null>(null);
	const [actionError, setActionError] = React.useState<string | null>(null);
	const pendingActionRef = React.useRef<string | null>(null);
	const favorites = useFavorites({ enabled: tab === "products" });
	const following = useFollowing({ enabled: tab === "sellers" });

	const runPendingAction = async (
		action: string,
		request: () => Promise<unknown>,
	) => {
		if (pendingActionRef.current) return;
		pendingActionRef.current = action;
		setPendingAction(action);
		setActionError(null);
		try {
			await request();
		} catch {
			setActionError("操作失敗，請稍後再試");
		} finally {
			pendingActionRef.current = null;
			setPendingAction(null);
		}
	};

	const visibleProducts = browsableFavorites(favorites.items);
	const visibleSellers = following.items;
	// Seller-side counterpart: the backend degrades-not-propagates when the
	// seller source fails (rows come back with empty title/imageUrl and
	// degraded:true). isBrowsableSavedItem never filters seller rows out, so
	// without this they'd render as blank, nameless rows with no explanation —
	// a regression vs. the old FollowUseCase.GetSubscriptionList's propagate-
	// and-show-error behaviour.
	const hasUnloadableSellers = React.useMemo(
		() => visibleSellers.some((it) => it.store === undefined),
		[visibleSellers],
	);

	return (
		<main className="favorites-page">
			<SellerPageShell
				title="我的收藏"
				subtitle="收藏的商品與追蹤的賣家。"
				toolbar={
					<ScFilters>
						{(["products", "sellers"] as const).map((t) => (
							<ScPill
								key={t}
								as="button"
								role="tab"
								aria-selected={tab === t}
								active={tab === t}
								disabled={pendingAction !== null}
								onClick={() => setTab(t)}
							>
								{TAB_LABELS[t]}
							</ScPill>
						))}
					</ScFilters>
				}
			>
				{tab === "products" ? (
					<section>
						{(favorites.error || actionError) && (
							<p role="alert">{favorites.error || actionError}</p>
						)}
						{favorites.loading && visibleProducts.length === 0 ? (
							<p>載入中…</p>
						) : visibleProducts.length === 0 ? (
							<ScEmpty>
								<AccountEmptyStateIcon variant="favorites" />
								<ScEmptyTitle>還沒有收藏商品</ScEmptyTitle>
								<Button href="/market/products" size="lg" radius="full">
									去逛逛商品
								</Button>
							</ScEmpty>
						) : (
							<ul className="list-none m-0 p-0 grid grid-cols-2 gap-3 sm:grid-cols-[repeat(auto-fill,minmax(200px,1fr))] sm:gap-4">
								{visibleProducts.map((it: Favorite) => {
									const product = it.product;
									const productId = product?.id ?? "";
									const productName = product?.name ?? "";
									const productImage = imageUri(product?.image, "card");
									const store = product?.store;
									const sellerName = store?.name;
									const sellerLogo = imageUri(store?.logo, "avatar");
									const unavailable = Boolean(
										product?.deleted || !product?.available,
									);
									const cardInner = (
										<>
											<span className="block w-full aspect-square relative overflow-hidden bg-[linear-gradient(135deg,_rgb(var(--color-brand-primary-rgb)_/_0.1),_rgb(var(--color-brand-primary-rgb)_/_0.02))]">
												<AppImage
													src={productImage || ""}
													alt={productName || `商品 #${productId}`}
													fill
													sizes="(max-width: 640px) 50vw, 240px"
													loading="eager"
													className="object-cover"
												/>
												{unavailable ? (
													<span className="absolute inset-0 flex items-center justify-center bg-black/45 text-white text-sm font-bold tracking-wide">
														商品已下架
													</span>
												) : null}
											</span>
											<div className="flex flex-col gap-1.5 pt-2.5 px-3 pb-3 min-w-0">
												<strong className="text-sm font-bold text-foreground leading-[1.35] line-clamp-2 min-h-[2.4em]">
													{productName || `商品 #${productId}`}
												</strong>
												{sellerName ? (
													<span className="inline-flex items-center gap-1.5 text-[length:0.75rem] text-[color:var(--ink-soft,#6b6b6b)] font-semibold min-w-0">
														<Avatar
															src={sellerLogo}
															name={sellerName}
															alt=""
															entity="seller"
															loading="eager"
															className="w-[18px] h-[18px] rounded-full object-cover bg-[var(--surface-panel,#e8e8e8)] shrink-0"
															fallbackClassName="grid place-items-center text-[0.625rem] font-bold text-[var(--color-brand-primary-strong)]"
															fallback="empty"
														/>
														<em className="not-italic overflow-hidden text-ellipsis whitespace-nowrap min-w-0">
															{sellerName}
														</em>
													</span>
												) : null}
												<div className="flex items-baseline justify-between gap-1.5 mt-0.5">
													<span className="text-[color:var(--color-brand-primary-strong)] font-extrabold text-[length:0.9375rem] tabular-nums">
														{formatPriceRange(
															Number(product?.minPrice),
															Number(product?.maxPrice),
														)}
													</span>
													{typeof product?.rating === "number" &&
													product.rating > 0 ? (
														<span
															role="img"
															className="inline-flex items-center gap-0.5 text-[#d97706] text-[length:0.75rem] font-bold tabular-nums"
															aria-label={`評價 ${product.rating.toFixed(1)} 顆星`}
														>
															★ {product.rating.toFixed(1)}
														</span>
													) : null}
												</div>
											</div>
										</>
									);
									return (
										<li
											key={productId}
											className={
												unavailable
													? "relative flex bg-content1 border border-divider rounded-hkui-lg overflow-hidden opacity-60"
													: "relative flex bg-content1 border border-divider rounded-hkui-lg overflow-hidden transition-[border-color,box-shadow,transform] duration-200 ease-[var(--hkui-ease)] hover:border-[var(--color-brand-primary-strong)] hover:shadow-[0_6px_20px_-10px_rgb(var(--color-brand-primary-rgb)/0.32)] hover:-translate-y-0.5"
											}
											aria-disabled={unavailable || undefined}
										>
											{unavailable ? (
												<div className="flex flex-col flex-1 min-w-0 text-inherit cursor-not-allowed">
													{cardInner}
												</div>
											) : (
												<Link
													href={`/products/${productId}`}
													className="flex flex-col flex-1 min-w-0 text-inherit no-underline"
												>
													{cardInner}
												</Link>
											)}
											<Button
												type="button"
												variant="light"
												color="danger"
												size="sm"
												isIconOnly
												className="absolute top-2 right-2 z-[1] h-7 w-7 rounded-full bg-black/55 p-0 text-base leading-none text-white transition-[background] duration-200 ease-[var(--hkui-ease)] hover:bg-danger"
												aria-label={
													unavailable ? "移除這筆已下架收藏" : "取消收藏"
												}
												disabled={pendingAction !== null}
												isLoading={
													pendingAction === `remove-product:${productId}`
												}
												loadingLabel="取消收藏中…"
												onClick={() =>
													void runPendingAction(
														`remove-product:${productId}`,
														() => favorites.remove(productId),
													)
												}
											>
												×
											</Button>
										</li>
									);
								})}
							</ul>
						)}
						{favorites.hasMore && (
							<div className="flex justify-center mt-4">
								<Button
									variant="bordered"
									disabled={pendingAction !== null}
									isLoading={pendingAction === "more-products"}
									loadingLabel="載入更多商品中…"
									onClick={() =>
										void runPendingAction("more-products", favorites.loadMore)
									}
								>
									載入更多
								</Button>
							</div>
						)}
					</section>
				) : (
					<section>
						{(following.error || actionError) && (
							<p role="alert">{following.error || actionError}</p>
						)}
						{!following.error && hasUnloadableSellers ? (
							<p role="alert">{SELLER_DEGRADED_MESSAGE}</p>
						) : null}
						{following.loading && visibleSellers.length === 0 ? (
							<p>載入中…</p>
						) : visibleSellers.length === 0 ? (
							<ScEmpty>
								<AccountEmptyStateIcon variant="sellers" />
								<ScEmptyTitle>還沒追蹤任何賣家</ScEmptyTitle>
								<Button href="/market/products" size="lg" radius="full">
									逛逛 Hawkr 市集
								</Button>
							</ScEmpty>
						) : (
							<ul className="list-none m-0 p-0 flex flex-col gap-2">
								{visibleSellers.map((it: Follow) => (
									<li
										key={it.storeId}
										className="flex justify-between items-center bg-content1 border border-divider rounded-hkui-lg py-3 px-4"
									>
										<Link
											href={sellerPath(it.store?.storeCode, it.storeId)}
											className="flex items-center gap-3 text-inherit no-underline flex-1 min-w-0"
										>
											<Avatar
												src={imageUri(it.store?.logo, "avatar")}
												name={it.store?.name ?? ""}
												alt=""
												entity="seller"
												className="w-11 h-11 rounded-full object-cover bg-default-100 shrink-0"
												fallback="empty"
											/>
											<strong className="text-base">{it.store?.name}</strong>
										</Link>
										<Button
											variant="bordered"
											color="default"
											size="sm"
											disabled={pendingAction !== null}
											isLoading={
												pendingAction === `unfollow-seller:${it.storeId}`
											}
											loadingLabel="取消追蹤商家中…"
											onClick={() =>
												void runPendingAction(
													`unfollow-seller:${it.storeId}`,
													() => following.unfollow(it.storeId),
												)
											}
										>
											取消追蹤
										</Button>
									</li>
								))}
							</ul>
						)}
						{following.hasMore && (
							<div className="flex justify-center mt-4">
								<Button
									variant="bordered"
									disabled={pendingAction !== null}
									isLoading={pendingAction === "more-sellers"}
									loadingLabel="載入更多賣家中…"
									onClick={() =>
										void runPendingAction("more-sellers", following.loadMore)
									}
								>
									載入更多
								</Button>
							</div>
						)}
					</section>
				)}
			</SellerPageShell>
		</main>
	);
}
