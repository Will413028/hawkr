"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import HomeMerchantPrizeIcon from "@/features/home/components/HomeMerchantPrizeIcon";
import type { MarketSellerItem } from "@/features/market/components/MarketSellerCard";
import SellerFollowChip from "@/features/seller/components/SellerFollowChip";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { SELLER_COVER_ASPECT } from "@/lib/media/seller-cover";
import { canPrefetchMainNav } from "@/lib/navigation/main-nav-prefetch";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

type HomeCompactMerchantsProps = {
	sellers: MarketSellerItem[];
	title?: string;
	href?: string;
};

/**
 * 推薦商家 row, restyled to match the compact product shelves
 * (`HomeCompactNewArrivals`): same cream card container, single-line
 * header, and a wider horizontal rail so the section keeps the same
 * height as its neighbours while giving seller cards enough width for
 * real covers, avatars, and compact stats to read cleanly.
 */
export default function HomeCompactMerchants({
	sellers,
	title = "推薦商家",
	href = "/market/products?tab=nearby",
}: HomeCompactMerchantsProps) {
	const railRef = React.useRef<HTMLDivElement | null>(null);
	const router = useObservedRouter("home_merchants");
	const [scrollState, setScrollState] = React.useState({
		canPrev: false,
		canNext: false,
	});
	// Mouse drag-to-scroll. `moved` gates the post-drag click so a pull
	// doesn't navigate into a seller. Touch is left to native scrolling.
	const drag = React.useRef({
		active: false,
		moved: false,
		startX: 0,
		startScroll: 0,
	});

	const updateScrollState = React.useCallback(() => {
		const rail = railRef.current;
		if (!rail) return;
		const maxScroll = Math.max(0, rail.scrollWidth - rail.clientWidth);
		setScrollState({
			canPrev: rail.scrollLeft > 2,
			canNext: rail.scrollLeft < maxScroll - 2,
		});
	}, []);

	React.useEffect(() => {
		const rail = railRef.current;
		if (!rail) return;

		updateScrollState();
		const onScroll = () => updateScrollState();
		rail.addEventListener("scroll", onScroll, { passive: true });

		const resizeObserver =
			typeof ResizeObserver !== "undefined"
				? new ResizeObserver(updateScrollState)
				: null;
		resizeObserver?.observe(rail);
		window.addEventListener("resize", updateScrollState);

		return () => {
			rail.removeEventListener("scroll", onScroll);
			resizeObserver?.disconnect();
			window.removeEventListener("resize", updateScrollState);
		};
	}, [updateScrollState]);

	// 行動端沒有 hover，卡片捲進 rail 視口就用與桌面 hover 暖身相同的
	// router.prefetch 預抓 storefront payload——點擊時已在 client cache，
	// 消掉「點賣家等整輪後端 RTT」的死等。每個 href 只抓一次，rail 最多
	// 8 張卡，成本有上限。
	React.useEffect(() => {
		const rail = railRef.current;
		if (!rail || typeof IntersectionObserver === "undefined") return;
		const railHrefs = new Set(sellers.map((seller) => seller.href));
		const prefetched = new Set<string>();
		const observer = new IntersectionObserver(
			(entries) => {
				// Same network gate as the main-tab idle warm: this observer fires on
				// scroll, not on user intent, so it must back off exactly where that
				// coordinator does (offline / save-data / 2G / 3G) instead of racing a
				// real navigation for the connection.
				if (!canPrefetchMainNav("idle")) return;
				for (const entry of entries) {
					if (!entry.isIntersecting) continue;
					const href = entry.target.getAttribute("href");
					if (!href || !railHrefs.has(href) || prefetched.has(href)) continue;
					prefetched.add(href);
					router.prefetch(href);
					observer.unobserve(entry.target);
				}
			},
			// 橫向預留一張卡寬，捲動到達前就開抓。
			{ root: rail, rootMargin: "0px 240px" },
		);
		for (const anchor of rail.querySelectorAll<HTMLAnchorElement>(
			'a[href^="/sellers/"]',
		)) {
			observer.observe(anchor);
		}
		return () => observer.disconnect();
	}, [sellers, router]);

	const scroll = (dir: -1 | 1) => {
		const rail = railRef.current;
		if (!rail) return;
		rail.scrollBy({
			left: dir * Math.min(rail.clientWidth * 0.85, 800),
			behavior: "smooth",
		});
	};

	if (sellers.length === 0) return null;

	const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
		if (e.pointerType !== "mouse" || e.button !== 0) return;
		const rail = railRef.current;
		if (!rail) return;
		drag.current = {
			active: true,
			moved: false,
			startX: e.clientX,
			startScroll: rail.scrollLeft,
		};
	};

	const onPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
		const state = drag.current;
		if (!state.active) return;
		const rail = railRef.current;
		if (!rail) return;
		const dx = e.clientX - state.startX;
		if (!state.moved && Math.abs(dx) > 4) {
			state.moved = true;
			rail.setPointerCapture(e.pointerId);
		}
		if (state.moved) rail.scrollLeft = state.startScroll - dx;
	};

	const endDrag = (e: React.PointerEvent<HTMLDivElement>) => {
		const rail = railRef.current;
		if (rail?.hasPointerCapture(e.pointerId)) {
			rail.releasePointerCapture(e.pointerId);
		}
		drag.current.active = false;
	};

	const onClickCapture = (e: React.MouseEvent<HTMLDivElement>) => {
		if (drag.current.moved) {
			e.preventDefault();
			e.stopPropagation();
			drag.current.moved = false;
		}
	};

	return (
		<section className="home-fixed-section relative z-[2]" aria-label={title}>
			{/* Padding lives on the rail (`-mx-4 px-4 pb-5`), not here — see the
			    full note in HomeCompactNewArrivals: the rail is a scroll
			    container and clips the cards' hover shadow at its own padding
			    box, so room on the outside doesn't help. */}
			<div className="home-section-surface rounded-[24px] border border-[rgba(58,42,26,0.07)] bg-hk-home-postcard px-[18px] pt-[18px] shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_12px_36px_-22px_rgba(76,50,20,0.18)] dark:border-[rgba(245,222,165,0.12)] dark:shadow-[0_12px_36px_-22px_rgba(0,0,0,0.55)]">
				<header className="mb-4 flex items-center justify-between gap-3 px-1">
					<h2 className="m-0 text-[1.625rem] font-black leading-tight text-hk-heading">
						{title}
					</h2>
					<Link
						href={href}
						className="rounded-full border border-hk-border-soft bg-hk-surface-card px-4 py-2 text-sm font-black text-hk-heading no-underline transition-colors hover:bg-hk-primary hover:text-hk-inverse dark:border-[rgba(245,222,165,0.16)] dark:bg-white/6 dark:text-[#f5e9c3] dark:hover:bg-[#f1cd70] dark:hover:text-hk-ink-950"
					>
						看全部商家
					</Link>
				</header>
				<div className="relative">
					<button
						type="button"
						aria-label="上一頁"
						aria-hidden={!scrollState.canPrev}
						tabIndex={scrollState.canPrev ? 0 : -1}
						className={`absolute left-0 top-1/2 z-10 grid size-10 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full border border-hk-border-soft bg-hk-surface-card text-2xl text-hk-heading shadow-hk-card transition-[opacity,transform] max-[640px]:hidden dark:border-[rgba(245,222,165,0.16)] dark:bg-hk-ink-850 dark:text-[#f5e9c3] ${
							scrollState.canPrev
								? "opacity-100"
								: "pointer-events-none opacity-0"
						}`}
						onClick={() => scroll(-1)}
					>
						‹
					</button>
					{/* biome-ignore lint/a11y/noStaticElementInteractions: pointer handlers only add mouse drag-to-scroll on a scroll container; the interactive targets are the <Link> cards inside, keyboard/touch use native scroll + the arrow buttons. */}
					<div
						ref={railRef}
						onPointerDown={onPointerDown}
						onPointerMove={onPointerMove}
						onPointerUp={endDrag}
						onPointerCancel={endDrag}
						onClickCapture={onClickCapture}
						onDragStart={(e) => e.preventDefault()}
						className="grid auto-cols-[230px] grid-flow-col gap-3 overflow-x-auto -mx-4 px-4 pb-5 select-none [scrollbar-width:none] [&::-webkit-scrollbar]:hidden max-[640px]:auto-cols-[minmax(190px,52vw)] max-[640px]:gap-2.5 max-[640px]:cursor-default cursor-grab active:cursor-grabbing"
					>
						{sellers.map((seller) => (
							<article
								key={seller.id}
								className="relative flex h-full flex-col overflow-hidden rounded-hk-xl border border-hk-border-soft bg-hk-surface-card transition-[transform,box-shadow] hover:-translate-y-1 hover:shadow-hk-lift dark:border-[rgba(245,222,165,0.14)] dark:bg-hk-ink-850 dark:hover:shadow-[0_18px_36px_-24px_rgba(0,0,0,0.72)]"
							>
								<Link
									href={seller.href}
									draggable={false}
									aria-label={`查看商家 ${seller.name}`}
									className="relative block overflow-hidden bg-hk-surface-muted"
									style={{ aspectRatio: SELLER_COVER_ASPECT }}
								>
									<AppImage
										src={resolveCdnUrl(seller.image) || seller.image}
										alt={seller.name}
										fill
										preset="banner"
										sizes="(max-width: 640px) 52vw, 230px"
										loading="lazy"
										// 推薦商家在首屏之下，交給瀏覽器接近視口時載入。
										fetchPriority="low"
										className="object-cover object-center"
									/>
									{seller.location ? (
										<span className="absolute left-2.5 top-2.5 z-[2] inline-flex items-center gap-1 rounded-full bg-black/55 px-2 py-1 text-[0.625rem] font-black text-white backdrop-blur-sm">
											<LocationPin />
											{seller.location}
										</span>
									) : null}
								</Link>
								<div className="flex flex-1 flex-col px-2.5 pb-2.5 pt-0">
									{/* Keep the avatar just below the location chip on the shallow
									    4:1 seller cover while retaining a small seam overlap. */}
									<Link
										href={seller.href}
										draggable={false}
										aria-label={`查看商家 ${seller.name}`}
										className="relative z-[2] -mt-5 w-fit max-[640px]:-mt-3"
									>
										<Avatar
											src={seller.logo_url}
											name={seller.name}
											alt={seller.name}
											entity="seller"
											className="block size-14 rounded-full border-[3px] border-hk-surface-card bg-hk-surface-card object-cover shadow-[0_8px_18px_-4px_rgb(var(--color-ink-900-rgb)/0.24)] max-[640px]:size-12"
											fallback="empty"
											loading="lazy"
											fetchPriority="low"
										/>
									</Link>
									<Link
										href={seller.href}
										draggable={false}
										className="mt-1.5 flex items-center gap-1 text-inherit no-underline"
									>
										<span className="truncate text-sm font-black leading-snug text-hk-heading">
											{seller.name}
										</span>
										{seller.certified ? (
											<AppIcon
												className="shrink-0 text-[var(--icon-accent)]"
												name="shield"
												size={16}
												title="已通過商家認證"
											/>
										) : null}
									</Link>
									{seller.prize ? (
										<span
											className="mt-1 inline-flex w-fit max-w-full items-center gap-1 rounded-full bg-warning-50 py-[3px] pl-1.5 pr-2 text-[0.625rem] font-black text-warning-700 dark:bg-[#f1cd70]/14 dark:text-[#f1cd70]"
											title={seller.prize}
										>
											<HomeMerchantPrizeIcon />
											<span className="truncate">{seller.prize}</span>
										</span>
									) : null}
									<div className="mt-1 truncate text-[0.6875rem] font-bold text-hk-muted">
										{/* 商品數只在後端真的給正數時顯示 — stats 的
										    total_products 目前一律回 0，顯示會變成謊報。 */}
										{[
											seller.products > 0 ? `商品 ${seller.products}` : null,
											seller.rating > 0
												? `評分 ${seller.rating.toFixed(1)}`
												: "尚無評價",
											seller.followers > 0
												? `追蹤人數 ${seller.followers.toLocaleString()}`
												: null,
										]
											.filter(Boolean)
											.join(" · ")}
									</div>
									<div className="mt-auto pt-2.5">
										<SellerFollowChip
											sellerId={seller.id}
											className="market-seller-card__follow w-full"
										/>
									</div>
								</div>
							</article>
						))}
					</div>
					<button
						type="button"
						aria-label="下一頁"
						aria-hidden={!scrollState.canNext}
						tabIndex={scrollState.canNext ? 0 : -1}
						className={`absolute right-0 top-1/2 z-10 grid size-10 -translate-y-1/2 translate-x-1/2 place-items-center rounded-full border border-hk-border-soft bg-hk-surface-card text-2xl text-hk-heading shadow-hk-card transition-[opacity,transform] max-[640px]:hidden dark:border-[rgba(245,222,165,0.16)] dark:bg-hk-ink-850 dark:text-[#f5e9c3] ${
							scrollState.canNext
								? "opacity-100"
								: "pointer-events-none opacity-0"
						}`}
						onClick={() => scroll(1)}
					>
						›
					</button>
				</div>
			</div>
		</section>
	);
}

function LocationPin() {
	return (
		<svg
			width="11"
			height="11"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2.4"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
			focusable="false"
		>
			<path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 1 1 16 0Z" />
			<circle cx="12" cy="10" r="3" />
		</svg>
	);
}
