"use client";

import * as React from "react";
import AddToCartButton from "@/components/shared/AddToCartButton";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import Avatar from "@/components/shared/Avatar";
import FavoriteButton from "@/components/shared/FavoriteButton";
import Link from "@/components/shared/Link";
import RestockNotifyButton from "@/components/shared/RestockNotifyButton";
import type { MarketProductItem } from "@/features/market/components/MarketProductsList";
import { trackSelectItem } from "@/lib/analytics/events";
import { formatUnitLabel } from "@/lib/codemeta/enums";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { sellerPath } from "@/lib/seo/urls";

interface Props {
	product: MarketProductItem;
	priority?: boolean;
	/**
	 * Lists that render a complete initial dataset use eager loading so a
	 * vertical scroll never becomes the trigger for an existing card image.
	 * `priority` remains separate and is reserved for the single LCP card.
	 */
	imageLoading?: "eager" | "lazy";
	/** GA4 item_list context, threaded from the list that renders this card. */
	listContext?: { listId: string; listName: string };
	index?: number;
	/**
	 * 預覽模式（賣家匯入精靈 Page 4 用）：layout 與真實列表完全一致，但鎖掉所有互動
	 * — FavoriteButton / 數量步進器 / AddToCartButton / RestockNotifyButton 全部不能按；
	 * 改用「預覽」chip 取代右上角愛心位置。
	 * 點圖片 / 標題仍會走 `product.href`（由父層設成預覽詳情頁路徑）。
	 */
	previewMode?: boolean;
	/** Seller storefront preview: the primary card link opens a read-only drawer. */
	storefrontPreviewMode?: boolean;
}

/** 1.2萬-style compaction so six-digit counts don't blow out the price row. */
function formatCompactCount(value: number): string {
	if (value < 10000) return String(value);
	const wan = Math.round((value / 10000) * 10) / 10;
	return `${wan}萬`;
}

const LOW_STOCK_THRESHOLD = 10;

function MarketProductCard({
	product,
	priority = false,
	imageLoading = "lazy",
	listContext,
	index,
	previewMode = false,
	storefrontPreviewMode = false,
}: Props) {
	const imageSrc = resolveCdnUrl(product.image);
	const [quantity, setQuantity] = React.useState(0);

	// 已售出 hides at 0 (new listings shouldn't open with a zero); 剩餘 hides only
	// when the data source carries no stock at all — 0 stock is a real state and
	// renders as 已售完.
	const soldLabel =
		product.salesCount > 0
			? `已售出 ${formatCompactCount(product.salesCount)}`
			: null;
	const soldOut = product.stock !== undefined && product.stock <= 0;
	const lowStock =
		product.stock !== undefined &&
		product.stock > 0 &&
		product.stock <= LOW_STOCK_THRESHOLD;
	const stockLabel =
		product.stock === undefined
			? null
			: soldOut
				? "已售完"
				: `剩餘 ${formatCompactCount(product.stock)} 件`;

	// Cap the stepper at remaining stock when the list carries it, so the buyer
	// can't queue more than the seller has (checkout would reject it anyway).
	const maxQuantity = Math.min(99, product.stock ?? 99);
	const decreaseQuantity = () => setQuantity((value) => Math.max(0, value - 1));
	const increaseQuantity = () =>
		setQuantity((value) => Math.min(maxQuantity, value + 1));
	const cartItem = {
		id: product.id,
		name: product.name,
		price: product.price,
		image: product.image,
		sellerName: product.sellerName,
		unit: formatUnitLabel(product.unit),
		href: product.href,
	};

	const sellerHref = sellerPath(product.sellerStoreCode);

	return (
		<article
			className="market-product-card relative isolate overflow-hidden border border-hk-border-soft rounded-hk-xl bg-hk-surface-card text-inherit no-underline shadow-hk-card max-[840px]:rounded-hk-lg dark:bg-white/4 dark:backdrop-blur-md dark:saturate-[1.2] dark:border-[rgba(245,222,165,0.12)] dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.06),0_12px_28px_rgba(0,0,0,0.32)]"
			data-on-sale={product.isOnSale ? "true" : undefined}
		>
			<Link
				href={product.href}
				aria-label={`查看商品：${product.name}`}
				aria-haspopup={storefrontPreviewMode ? "dialog" : undefined}
				aria-expanded={storefrontPreviewMode ? false : undefined}
				data-market-product-primary-link=""
				data-storefront-product-preview-trigger={
					storefrontPreviewMode ? "true" : undefined
				}
				className="absolute inset-0 z-10 rounded-hk-xl no-underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-hk-primary max-[840px]:rounded-hk-lg"
				onClick={(event) => {
					if (event.defaultPrevented || !listContext) return;
					try {
						trackSelectItem(product, listContext, index);
					} catch {
						// Analytics must never block the product's native link contract.
					}
				}}
			>
				<span className="sr-only">查看商品：{product.name}</span>
			</Link>
			{previewMode ? (
				<span
					role="status"
					aria-label="預覽模式"
					className="pointer-events-none absolute right-[18px] top-[18px] z-20 inline-flex items-center h-[26px] px-3 rounded-full bg-hk-warning text-white text-xs font-bold tracking-[0.02em] shadow-[0_1px_4px_rgba(0,0,0,0.18)] max-[840px]:right-2.5 max-[840px]:top-2.5 max-[840px]:h-[20px] max-[840px]:px-2 max-[840px]:text-[0.625rem]"
				>
					預覽
				</span>
			) : (
				<FavoriteButton
					productId={product.id}
					productName={product.name}
					initialFavorited={product.isFavorited}
					count={product.favoriteCount}
					className="absolute right-[18px] top-[18px] z-20 max-[840px]:right-2.5 max-[840px]:top-2.5"
					size={16}
				/>
			)}
			<div className="market-product-card__media relative block p-3 max-[840px]:p-1.5">
				<div
					className="relative w-full aspect-[4/3] overflow-hidden rounded-[18px] bg-hk-surface-panel max-[840px]:rounded-[10px]"
					data-zoom-card=""
				>
					<AppImage
						src={imageSrc}
						alt={product.name}
						fill
						preset="card"
						// Cards sit in an auto-fill minmax(220px) grid / rail, so they
						// stay ~220-360px wide on desktop — they never grow to 33vw.
						// Cap the desktop slot so Next requests the ~750px candidate
						// (360px × 2 DPR) instead of rounding 33vw up to w_1920.
						sizes="(max-width: 360px) 100vw, (max-width: 680px) 50vw, (max-width: 1120px) 33vw, 360px"
						priority={priority}
						loading={priority ? undefined : imageLoading}
						// 首頁三個貨架與市集列表都用這張卡，且貨架會重複渲染同一批商品。
						// React 19 SSR 對每個非 lazy、非 low 的 <img> 自動塞 preload，一頁
						// 就是幾十條；只有 priority 那張是 LCP 候選，其餘 eager 但 low。
						fetchPriority={priority ? undefined : "low"}
						className="object-cover"
					/>
				</div>
				{product.hashtags.length > 0 ? (
					<ul
						className="market-product-card__tags absolute left-[22px] right-[22px] bottom-[22px] flex flex-wrap gap-1 min-h-0 m-0 p-0 list-none rounded-none bg-transparent pointer-events-none max-[840px]:left-2 max-[840px]:right-2 max-[840px]:bottom-2 max-[840px]:gap-[3px]"
						aria-label="標籤"
					>
						{product.hashtags.map((tag) => (
							<li
								key={tag}
								className="market-product-card__tag inline-flex items-center h-[22px] px-[9px] rounded-[11px] border-[0.5px] border-black/22 bg-white/94 text-hk-primary-strong text-[0.6875rem] font-bold tracking-[0.01em] whitespace-nowrap overflow-hidden text-ellipsis max-w-full backdrop-blur-[2px] shadow-[0_1px_4px_rgba(0,0,0,0.22)] max-[840px]:h-[18px] max-[840px]:px-[6px] max-[840px]:text-[0.625rem] max-[840px]:rounded-[9px] dark:bg-[rgb(var(--color-ink-950-rgb)/0.72)] dark:border-[rgba(245,222,165,0.32)] dark:text-hk-cream dark:shadow-[0_1px_4px_rgba(0,0,0,0.5)]"
							>
								#{tag}
							</li>
						))}
					</ul>
				) : null}
			</div>
			<div className="market-product-card__body p-[0_16px_16px] max-[840px]:p-[0_8px_10px]">
				{/* User-entered product name → system body stack, not the brand
				    face (its subset only covers fixed project strings). */}
				<h2 className="[font-family:var(--font-primary)] m-[12px_0_6px] text-hk-heading text-base leading-[1.45] tracking-normal max-[840px]:m-[8px_0_4px] max-[840px]:text-sm max-[840px]:leading-[1.3] line-clamp-2 min-h-[2.6em] max-[840px]:min-h-[2.6em]">
					{product.name}
				</h2>
				<p className="m-0 flex items-center gap-1.5 text-hk-muted text-xs leading-[1.45] max-[840px]:gap-1 max-[840px]:text-[0.6875rem] max-[840px]:leading-[1.35]">
					{sellerHref ? (
						<Link
							href={sellerHref}
							className="relative z-20 flex min-w-0 items-center gap-1.5 text-inherit no-underline transition-colors hover:text-hk-primary max-[840px]:gap-1"
						>
							<Avatar
								src={product.sellerAvatar}
								name={product.sellerName}
								alt={product.sellerName}
								entity="seller"
								loading={imageLoading}
								fetchPriority="low"
								className="size-5 shrink-0 rounded-full object-cover max-[840px]:size-4"
								fallbackClassName="grid place-items-center bg-hk-primary-subtle text-[0.625rem] font-bold text-hk-accent-on-tint"
								fallback="empty"
							/>
							<span className="truncate">{product.sellerName}</span>
						</Link>
					) : (
						<span className="flex min-w-0 items-center gap-1.5 max-[840px]:gap-1">
							<Avatar
								src={product.sellerAvatar}
								name={product.sellerName}
								alt={product.sellerName}
								entity="seller"
								loading={imageLoading}
								fetchPriority="low"
								className="size-5 shrink-0 rounded-full object-cover max-[840px]:size-4"
								fallbackClassName="grid place-items-center bg-hk-primary-subtle text-[0.625rem] font-bold text-hk-accent-on-tint"
								fallback="empty"
							/>
							<span className="truncate">{product.sellerName}</span>
						</span>
					)}
					{product.origin ? (
						<span className="shrink-0 whitespace-nowrap">
							｜{product.origin}
						</span>
					) : null}
				</p>
				<small className="mt-2 flex flex-wrap items-baseline justify-between gap-x-2 text-hk-muted text-xs max-[840px]:mt-1 max-[840px]:text-[0.6875rem] max-[840px]:leading-[1.35]">
					<span>
						{product.rating != null
							? `★ ${product.rating.toFixed(1)}｜${product.reviews} 則評價`
							: "尚無評價"}
					</span>
					{soldLabel || stockLabel ? (
						<span className="whitespace-nowrap">
							{soldLabel}
							{soldLabel && stockLabel ? "｜" : null}
							{stockLabel ? (
								<span
									className={
										soldOut || lowStock ? "font-bold text-hk-danger" : undefined
									}
								>
									{stockLabel}
								</span>
							) : null}
						</span>
					) : null}
				</small>
				<div className="market-product-card__price-row flex items-baseline gap-2 mt-2.5 flex-wrap max-[840px]:gap-1 max-[840px]:mt-1.5">
					<b className="block m-0 text-hk-primary text-lg leading-[1.34] max-[840px]:text-base max-[840px]:leading-[1.2]">
						{product.price}
					</b>
					{product.isOnSale && product.originalPrice ? (
						<s className="market-product-card__price-original text-[rgb(136,136,136,0.85)] text-[0.8125rem] font-medium line-through decoration-[1.2px] max-[840px]:text-[0.6875rem]">
							{product.originalPrice}
						</s>
					) : null}
				</div>
				<div className="market-product-card__buy-row mt-3 flex items-center gap-2 max-[840px]:mt-1.5">
					{/*
					  ⚠️ 下面兩顆按鈕都必須帶 `disabled:pointer-events-auto`。
					
					  共用 Button 的基底類別是 shadcn 慣例（components/ui/Button.tsx 的
					  `disabled:pointer-events-none`），而那個慣例在「disabled 控制項疊在
					  可點擊覆蓋層之上」時是錯的：整張卡是一條 `absolute inset-0 z-10` 的
					  商品連結，按鈕 z-20 在它上面。disabled 時若 pointer-events 為 none，
					  點擊會**穿透**到那條連結。
					
					  實測後果（2026-08-27，本機正式建置）：quantity 預設 0（:61 useState(0)）
					  ⇒ 加入購物車按鈕預設 disabled ⇒ 使用者按下去不是「沒反應」，而是**被
					  靜默導去商品頁**，完全不知道自己該先選數量。實測 15 秒後仍 disabled，
					  點擊使 /market/products → /products/{id}。
					
					  改 auto 之後按鈕自己吃掉那次點擊：不觸發任何事件（disabled 本來就不
					  派發 click），但游標是 not-allowed，使用者看得出來它現在不能按。
					
					  守它的是 tests/e2e/reliability.spec.ts:312 的
					  「favorite/seller/quantity/cart controls must stay independent」。
					*/}
					{soldOut && !previewMode ? (
						<div className="market-product-card__qty m-0 inline-flex h-10 min-w-0 flex-1 items-center justify-center overflow-hidden rounded-hk-pill border border-hk-border-soft bg-hk-surface-glass p-0 text-base font-extrabold text-hk-muted shadow-[inset_0_1px_0_rgb(255_255_255_/_0.28)]">
							已售完
						</div>
					) : (
						<fieldset
							className={`market-product-card__qty relative z-20 m-0 inline-grid h-10 min-w-0 flex-1 grid-cols-[1fr_1.6fr_1fr] items-center overflow-hidden rounded-hk-pill border border-hk-border-soft bg-hk-surface-glass p-0 text-hk-heading shadow-[inset_0_1px_0_rgb(255_255_255_/_0.28)] ${
								soldOut || previewMode ? "cursor-not-allowed opacity-45" : ""
							}`.trim()}
							disabled={soldOut || previewMode}
						>
							<legend className="sr-only">
								{product.name} 購買數量{soldOut ? "（已售完）" : ""}
							</legend>
							<button
								type="button"
								className="grid h-full place-items-center border-0 bg-transparent text-lg font-extrabold text-hk-muted transition-colors hover:bg-hk-surface-glass-hover hover:text-hk-heading disabled:cursor-not-allowed disabled:opacity-40"
								onClick={decreaseQuantity}
								disabled={quantity <= 0 || previewMode}
								aria-label={`減少 ${product.name} 購買數量`}
							>
								-
							</button>
							<output className="grid h-full place-items-center border-x border-hk-border-soft text-base font-extrabold tabular-nums text-hk-heading">
								{quantity}
							</output>
							<button
								type="button"
								className="grid h-full place-items-center border-0 bg-transparent text-lg font-extrabold text-hk-primary transition-colors hover:bg-hk-surface-glass-hover hover:text-hk-heading disabled:cursor-not-allowed disabled:opacity-45"
								onClick={increaseQuantity}
								disabled={quantity >= maxQuantity || previewMode}
								aria-label={`增加 ${product.name} 購買數量`}
							>
								+
							</button>
						</fieldset>
					)}
					{soldOut && !previewMode ? (
						<RestockNotifyButton
							className="market-product-card__cart-btn relative z-20 inline-flex h-10 flex-none items-center justify-center gap-1.5 border-0 rounded-hk-pill px-3.5 text-[0.8125rem] font-extrabold cursor-pointer transition-[background,color,filter] hover:enabled:brightness-105 disabled:pointer-events-auto disabled:cursor-not-allowed disabled:opacity-70"
							productId={product.id}
							productName={product.name}
							initialSubscribed={product.isRestockSubscribed}
							stopPropagation
						/>
					) : (
						<AddToCartButton
							className="market-product-card__cart-btn relative z-20 inline-flex h-10 flex-none items-center justify-center gap-1.5 border-0 rounded-hk-pill bg-hk-secondary-soft px-3.5 text-[0.8125rem] font-extrabold text-hk-primary-strong cursor-pointer transition-colors hover:bg-hk-primary-strong hover:text-hk-cream disabled:pointer-events-auto disabled:cursor-not-allowed disabled:bg-hk-secondary-soft/45 dark:bg-hk-ink-500 dark:text-hk-cream dark:shadow-[inset_0_0_0_1px_rgba(245,222,165,0.18)] dark:hover:bg-hk-ink-400 dark:disabled:bg-white/12 dark:disabled:text-hk-ink-100/45"
							item={{
								...cartItem,
								quantity,
							}}
							disabled={quantity <= 0 || previewMode}
							onAdded={() => setQuantity(0)}
							addedLabel={
								<span className="inline-flex animate-hk-fade-in items-center justify-center">
									<AppIcon
										name="check"
										size={24}
										style={{ color: "var(--color-hk-cream)" }}
									/>
									<span className="sr-only">已加入購物車</span>
								</span>
							}
							ariaLabel={
								previewMode
									? `預覽模式：${product.name}`
									: quantity > 0
										? `加入購物車：${product.name}，數量 ${quantity}`
										: `請先選擇 ${product.name} 購買數量`
							}
						>
							<span className="market-product-card__cart-btn-label">
								加入購物車
							</span>
							<AppIcon
								className="market-product-card__cart-btn-icon"
								name="bag"
								size={22}
								style={{ color: "var(--icon-accent)" }}
							/>
						</AddToCartButton>
					)}
				</div>
			</div>
		</article>
	);
}

// Appending a page preserves existing product references. Let masonry update
// positions without re-running unchanged cards. Use React's complete shallow
// prop comparison: changed business data, context and local state must still
// update normally (never compare only the product ID).
export default React.memo(MarketProductCard);
