"use client";

import * as React from "react";
import Link from "@/components/shared/Link";
import MarketProductCard from "@/features/market/components/MarketProductCard";
import type { MarketProductItem } from "@/features/market/components/MarketProductsList";

type HomeCompactNewArrivalsProps = {
	products: MarketProductItem[];
	title?: string;
	href?: string;
	ariaLabel?: string;
};

export default function HomeCompactNewArrivals({
	products,
	title = "新品上市",
	href = "/market/products?sort=最新上架",
	ariaLabel,
}: HomeCompactNewArrivalsProps) {
	const railRef = React.useRef<HTMLDivElement | null>(null);
	const listContext = React.useMemo(
		() => ({ listId: "home_new_arrivals", listName: title }),
		[title],
	);
	// Mouse drag-to-scroll. `moved` gates the post-drag click so a pull
	// doesn't navigate into a product. Touch is left to native scrolling.
	const drag = React.useRef({
		active: false,
		moved: false,
		startX: 0,
		startScroll: 0,
	});
	if (products.length === 0) return null;

	const scroll = (dir: -1 | 1) => {
		const rail = railRef.current;
		if (!rail) return;
		rail.scrollBy({
			left: dir * Math.min(rail.clientWidth * 0.85, 800),
			behavior: "smooth",
		});
	};

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

	// Capture phase: if the pointer dragged, swallow the click before it
	// reaches the <Link> (preventDefault kills native nav, stopPropagation
	// kills Next.js' router push).
	const onClickCapture = (e: React.MouseEvent<HTMLDivElement>) => {
		if (drag.current.moved) {
			e.preventDefault();
			e.stopPropagation();
			drag.current.moved = false;
		}
	};

	return (
		<section
			className="home-fixed-section relative z-[2]"
			aria-label={ariaLabel ?? title}
		>
			{/* The rail is a scroll container (`overflow-x: auto` forces
			    `overflow-y: auto` too), so its padding box is a hard clip edge —
			    the cards' 30px-blur drop shadow gets chopped mid-blur no matter
			    how much padding sits on the OUTSIDE of the rail. Hence the
			    breathing room lives on the rail itself: `pb-5` here (bottom
			    padding moved off this container) and `-mx-4 px-4` sideways,
			    where the negative margin keeps the cards and the arrows at
			    exactly the same position while pushing the clip edge out. */}
			<div className="home-section-surface rounded-[24px] border border-[rgba(58,42,26,0.07)] bg-hk-home-postcard px-[18px] pt-[18px] shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_12px_36px_-22px_rgba(76,50,20,0.18)] dark:border-[rgba(245,222,165,0.12)] dark:shadow-[0_12px_36px_-22px_rgba(0,0,0,0.55)]">
				<header className="mb-4 flex items-center justify-between gap-3 px-1">
					<h2 className="m-0 text-[1.625rem] font-black leading-tight text-hk-heading">
						{title}
					</h2>
					<Link
						href={href}
						className="rounded-full border border-hk-border-soft bg-hk-surface-card px-4 py-2 text-sm font-black text-hk-heading no-underline transition-colors hover:bg-hk-primary hover:text-hk-inverse"
					>
						查看全部
					</Link>
				</header>
				<div className="relative">
					<button
						type="button"
						aria-label="上一頁"
						className="absolute left-0 top-1/2 z-10 grid size-10 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full border border-hk-border-soft bg-hk-surface-card text-2xl text-hk-heading shadow-hk-card max-[640px]:hidden"
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
						className="grid auto-cols-[270px] grid-flow-col gap-3 overflow-x-auto -mx-4 px-4 pb-5 select-none [scrollbar-width:none] [&::-webkit-scrollbar]:hidden max-[640px]:auto-cols-[minmax(190px,52vw)] max-[640px]:gap-2.5 max-[640px]:cursor-default cursor-grab active:cursor-grabbing"
					>
						{products.map((product, index) => (
							<MarketProductCard
								key={product.id}
								product={product}
								imageLoading="lazy"
								listContext={listContext}
								index={index}
							/>
						))}
					</div>
					<button
						type="button"
						aria-label="下一頁"
						className="absolute right-0 top-1/2 z-10 grid size-10 -translate-y-1/2 translate-x-1/2 place-items-center rounded-full border border-hk-border-soft bg-hk-surface-card text-2xl text-hk-heading shadow-hk-card max-[640px]:hidden"
						onClick={() => scroll(1)}
					>
						›
					</button>
				</div>
			</div>
		</section>
	);
}
