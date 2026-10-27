"use client";

import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import CarouselDots from "@/components/shared/CarouselDots";
import Link from "@/components/shared/Link";
import { useCarouselAutoplay } from "@/components/shared/useCarouselAutoplay";
import { usePrefersReducedMotion } from "@/lib/a11y/reduced-motion";
import { resolveBannerActionHref } from "@/lib/banners/action";
import { consumeBannerSwipeClick } from "@/lib/banners/interaction";
import { type Banner, fetchBanners } from "@/lib/banners/repo";
import { queryKeys } from "@/lib/query/keys";
import { cn } from "@/lib/utils/cn";
import BannerLoadingCard from "./BannerLoadingCard";
import BannerPopupDialog from "./BannerPopupDialog";
import {
	FEATURE_CARD_BG,
	FEATURE_CARD_CHROME,
	FEATURE_CARD_WELL,
} from "./featureCard";

/** Horizontal travel (px) past which a pointer/touch swipe commits to the
 *  next / previous banner instead of snapping back. */
const SWIPE_THRESHOLD = 48;

/**
 * 上一張 / 下一張 chrome. Same frosted-glass surface as the navbar theme
 * toggle (`--page-theme-toggle-*`), so the two read as one control language
 * in both themes; 44×44 outright, overlaid on the vertical centre of the
 * slide so it never sits on the caption (bottom of the slide) or the dots
 * (outside the viewport). `compact` (the megamenu slot) has no room for it.
 */
const ARROW_CHROME =
	// geometry — 44×44 touch target, parked on the vertical centre of the slide
	"absolute top-1/2 z-10 -translate-y-1/2 inline-flex size-11 shrink-0 items-center justify-center rounded-full p-0 cursor-pointer " +
	// the theme toggle's frosted-glass surface, token for token
	"bg-[var(--page-theme-toggle-bg)] border border-[var(--page-theme-toggle-border)] text-[var(--page-theme-toggle-fg)] backdrop-blur-[6px] " +
	// motion: colour feedback only, and none at all under reduced motion
	"[transition:background_200ms_ease,color_200ms_ease,border-color_200ms_ease] motion-reduce:transition-none " +
	"hover:bg-[var(--page-theme-toggle-hover-bg)] active:scale-95 motion-reduce:active:scale-100 " +
	// same focus ring as the toggle
	"focus-visible:outline-2 focus-visible:outline-[var(--color-brand-primary-strong)] focus-visible:outline-offset-2";

interface BannerCarouselProps {
	/**
	 * Banners prefetched by a server component. When omitted the carousel
	 * lazy-fetches on mount — useful for client-only surfaces like the
	 * megamenu where there's no server boundary handy.
	 */
	initialBanners?: Banner[];
	/** Visual scale. `hero` for the homepage top, `compact` for embeds. */
	size?: "hero" | "compact";
	/** Auto-rotate interval. Set to 0 to disable auto-rotation. */
	intervalMs?: number;
	/** Heading text rendered above the slide area. Omit to hide. */
	heading?: string;
	className?: string;
}

/**
 * Buyer-facing rotating banner carousel. A valid empty source list renders
 * nothing; transport and backend failures remain Query errors so the shared
 * Query retry policy gets a chance to recover before the surface degrades.
 *
 * Used on:
 *   - Homepage hero zone (size="hero", SSR-prefetched banners)
 *   - Hawkr 市集 megamenu's right slot (size="compact", lazy-fetched)
 *
 * Auto-rotation holds while the cursor is over the carousel, while focus is
 * anywhere inside it, while the tab is hidden, and is switched off entirely
 * under `prefers-reduced-motion` (`useCarouselAutoplay`). Without the focus
 * and reduced-motion halves a keyboard user had no way to stop it at all
 * (WCAG 2.2.2).
 */
export default function BannerCarousel({
	initialBanners,
	size = "hero",
	intervalMs = 5000,
	heading,
	className,
}: BannerCarouselProps) {
	// SSR-prefetched callers (homepage hero) pass `initialBanners` → seeds the
	// cache and skips the fetch; the lazy (megamenu) path has no seed → fetches.
	// staleTime:Infinity keeps the SSR seed authoritative (initialData defaults to
	// dataUpdatedAt=0, which under the global 30s staleTime would trigger a
	// needless mount refetch) — banners are near-static, fetched once per session.
	const query = useQuery({
		queryKey: queryKeys.banners.list(),
		queryFn: () => fetchBanners(),
		initialData: initialBanners,
		staleTime: Number.POSITIVE_INFINITY,
	});
	const banners = query.data ?? [];
	const loading = query.isLoading;
	const [active, setActive] = React.useState(0);
	const [popupBanner, setPopupBanner] = React.useState<Banner | null>(null);
	// `aria-controls` target for the dot tablist — the tabs previously claimed
	// a tablist role while controlling nothing.
	const slideRegionId = React.useId();
	const reducedMotion = usePrefersReducedMotion();
	const [bannerAspectRatios, setBannerAspectRatios] = React.useState<
		Record<string, number>
	>({});
	// Pointer-drag / swipe state — mirrors the Hawkr 動態 carousel so the
	// banner block reads as the same affordance: the slide follows the
	// finger, then snaps to the next / previous banner past a threshold.
	const [drag, setDrag] = React.useState(0);
	const [dragging, setDragging] = React.useState(false);
	const startX = React.useRef<number | null>(null);
	// True once a gesture travels far enough to count as a swipe — lets us
	// swallow the trailing click so dragging a linked banner never also
	// navigates.
	const movedRef = React.useRef(false);
	const multi = banners.length > 1;

	const step = React.useCallback(
		(dir: number) => {
			// Swipe stops at the ends (the track has no slide to reveal past the
			// first / last banner); autoplay below still wraps to loop.
			setActive((i) => Math.max(0, Math.min(banners.length - 1, i + dir)));
		},
		[banners.length],
	);

	const onPointerDown = (e: React.PointerEvent) => {
		if (!multi) return;
		startX.current = e.clientX;
		movedRef.current = false;
		setDragging(true);
		e.currentTarget.setPointerCapture?.(e.pointerId);
	};
	const onPointerMove = (e: React.PointerEvent) => {
		if (startX.current === null) return;
		let dx = e.clientX - startX.current;
		if (Math.abs(dx) > 6) movedRef.current = true;
		// Rubber-band resistance at the ends so the track gives only a little
		// when there's nothing more to slide to.
		if ((active === 0 && dx > 0) || (active === banners.length - 1 && dx < 0))
			dx *= 0.3;
		setDrag(dx);
	};
	const endDrag = (e: React.PointerEvent) => {
		if (startX.current === null) return;
		const dx = e.clientX - startX.current;
		startX.current = null;
		setDragging(false);
		setDrag(0);
		if (dx <= -SWIPE_THRESHOLD) step(1);
		else if (dx >= SWIPE_THRESHOLD) step(-1);
	};

	const advance = React.useCallback(() => {
		setActive((i) => (i + 1) % banners.length);
	}, [banners.length]);
	// Arrow navigation wraps (unlike the swipe, which stops at the ends because
	// there is no slide left to drag into view).
	const paginate = React.useCallback(
		(dir: number) => {
			if (banners.length < 2) return;
			setActive((i) => (i + dir + banners.length) % banners.length);
		},
		[banners.length],
	);
	// A single-render `paused` pulse: `useCarouselAutoplay` rebuilds its timer
	// whenever `running` flips, so this hands an arrow click a full fresh
	// interval before the carousel moves on by itself — no extra hook API, and
	// nothing to keep in sync with the hook's own hold reasons.
	const [autoplayRestart, setAutoplayRestart] = React.useState(false);
	React.useEffect(() => {
		if (autoplayRestart) setAutoplayRestart(false);
	}, [autoplayRestart]);
	const stepByArrow = (dir: number) => {
		setAutoplayRestart(true);
		paginate(dir);
	};
	const autoplay = useCarouselAutoplay<HTMLElement>({
		count: banners.length,
		intervalMs,
		paused: dragging || autoplayRestart,
		onTick: advance,
	});

	// If the list shrinks (rare — backend dropping a banner mid-session),
	// snap back to a valid index.
	React.useEffect(() => {
		if (banners.length > 0 && active >= banners.length) setActive(0);
	}, [active, banners.length]);

	// Loading (lazy path only) shows the brand truck loader; a resolved-but-
	// empty list renders nothing so missing/disabled banners degrade silently.
	if (loading) {
		return size === "compact" ? <BannerLoadingCard heading={heading} /> : null;
	}
	if (banners.length === 0) return null;

	const wrapperClass = cn(
		"flex flex-col",
		size === "hero"
			? "gap-5 w-full max-w-[1280px] mx-auto px-5"
			: // compact = megamenu right slot: wear the shared feature-card chrome
				// so it reads as the same surface as the 本週主題 card, and fade in
				// when it swaps out from the loader.
				`gap-[var(--hkui-space-4)] w-full overflow-hidden p-[var(--hkui-space-5)] animate-hk-fade-in ${FEATURE_CARD_CHROME}`,
		className,
	);

	const viewportClass =
		size === "hero"
			? "relative overflow-hidden bg-[var(--page-banner-viewport-bg)] aspect-[16/9] rounded-[var(--radius-lg)] min-[601px]:rounded-[22px]"
			: FEATURE_CARD_WELL;

	// `duration-400` is not on Tailwind's default duration scale, so the class
	// emitted nothing and the hover zoom snapped instantly; `duration-300` is
	// the nearest real step.
	const imgClass = `block h-full w-full object-center transition-transform duration-300 ease group-hover:scale-[1.02] motion-reduce:transition-none ${
		size === "hero" ? "object-cover" : "object-contain"
	}`;
	const compactAspectRatio =
		banners[active] && bannerAspectRatios[banners[active].id]
			? bannerAspectRatios[banners[active].id]
			: 16 / 9;
	const captionClass = `absolute left-0 right-0 bottom-0 bg-gradient-to-t from-[var(--page-banner-caption-scrim)] to-transparent text-[var(--color-neutral-cream)] font-bold tracking-[0.5px] [text-shadow:0_1px_4px_rgb(0_0_0_/_0.4)] pointer-events-none ${
		size === "hero"
			? "text-[length:var(--text-base)] pt-6 px-[18px] pb-3.5 min-[601px]:text-[length:var(--text-xl)] min-[601px]:pt-9 min-[601px]:px-8 min-[601px]:pb-6"
			: "text-[0.9375rem] pt-[22px] px-5 pb-3.5"
	}`;

	return (
		<>
			<section
				ref={autoplay.containerRef}
				className={wrapperClass}
				style={size === "compact" ? FEATURE_CARD_BG : undefined}
				aria-roledescription="carousel"
				aria-label={heading ?? "最新消息"}
			>
				{heading ? (
					<h3 className="m-0 text-[length:var(--text-sm)] font-bold leading-normal text-[var(--page-banner-heading-fg)] tracking-[0.5px]">
						{heading}
					</h3>
				) : null}
				<div
					id={slideRegionId}
					className={viewportClass}
					style={
						size === "compact"
							? { aspectRatio: String(compactAspectRatio) }
							: undefined
					}
					// Announcing every slide while it rotates by itself would talk
					// over the user; announcing a slide the user just chose is
					// exactly what they asked for. So: live only while stopped.
					aria-live={autoplay.running ? "off" : "polite"}
				>
					<span className="sr-only">
						第 {active + 1} 張，共 {banners.length} 張
					</span>
					{/* All slides live in one flex track that slides left/right, so a
					 *  swipe reads as a real card switch and every slide image is
					 *  already in the DOM (no "loads after you swipe" gap).
					 *  biome-ignore lint/a11y/noStaticElementInteractions: swipe is a progressive enhancement; the dot tablist and each banner control remain accessible and keyboard-operable. */}
					<div
						// gap-[16px] puts space between slides; the translate offsets by
						// that same 16px per slide so the active card still lands flush.
						className="flex h-full w-full gap-[16px] select-none touch-pan-y [&_img]:pointer-events-none"
						style={{
							transform: `translate3d(calc(${-active * 100}% - ${active * 16}px + ${drag}px),0,0)`,
							// Reduced motion still swaps the slide, it just does not slide.
							transition:
								dragging || reducedMotion
									? "none"
									: "transform 460ms var(--hkui-ease)",
							cursor: multi ? (dragging ? "grabbing" : "grab") : "default",
						}}
						onPointerDown={onPointerDown}
						onPointerMove={onPointerMove}
						onPointerUp={endDrag}
						onPointerCancel={endDrag}
						onDragStart={(e) => e.preventDefault()}
					>
						{banners.map((b) => {
							const href = resolveBannerActionHref(b.action);
							const hasPopup = b.popupImage !== undefined;
							const inner = (
								<>
									<AppImage
										image={b.previewImage}
										alt={b.title}
										fill
										preset="banner"
										sizes="(max-width: 1280px) 100vw, 1280px"
										className={imgClass}
										onLoad={(event) => {
											if (size !== "compact") return;
											const image = event.currentTarget;
											if (image.naturalWidth <= 0 || image.naturalHeight <= 0)
												return;
											const ratio = image.naturalWidth / image.naturalHeight;
											setBannerAspectRatios((current) =>
												current[b.id] === ratio
													? current
													: { ...current, [b.id]: ratio },
											);
										}}
									/>
									<span className={captionClass}>{b.title}</span>
								</>
							);
							return (
								<div
									key={b.id}
									className="relative shrink-0 grow-0 basis-full h-full"
								>
									{hasPopup ? (
										<button
											type="button"
											className="group relative block h-full w-full cursor-pointer border-0 bg-transparent p-0 text-inherit"
											aria-label={`查看${b.title}`}
											draggable={false}
											onClick={(e) => {
												if (consumeBannerSwipeClick(movedRef, e.detail)) {
													e.preventDefault();
													return;
												}
												setPopupBanner(b);
											}}
										>
											{inner}
										</button>
									) : href ? (
										href.startsWith("/") ? (
											<Link
												href={href}
												className="block relative w-full h-full text-inherit no-underline group"
												aria-label={b.title}
												draggable={false}
												onClick={(e) => {
													// A swipe ends with a synthetic click — swallow it so
													// dragging the track never navigates.
													if (consumeBannerSwipeClick(movedRef, e.detail))
														e.preventDefault();
												}}
											>
												{inner}
											</Link>
										) : (
											<a
												href={href}
												className="block relative w-full h-full text-inherit no-underline group"
												aria-label={b.title}
												draggable={false}
												rel="noopener noreferrer"
												onClick={(e) => {
													if (consumeBannerSwipeClick(movedRef, e.detail))
														e.preventDefault();
												}}
											>
												{inner}
											</a>
										)
									) : (
										<div className="block relative w-full h-full text-inherit no-underline group">
											{inner}
										</div>
									)}
								</div>
							);
						})}
					</div>
					{multi && size !== "compact" ? (
						<>
							<button
								type="button"
								className={`${ARROW_CHROME} left-3 min-[601px]:left-4`}
								aria-label="上一張"
								aria-controls={slideRegionId}
								onClick={() => stepByArrow(-1)}
							>
								<AppIcon name="chevron-left" size={22} />
							</button>
							<button
								type="button"
								className={`${ARROW_CHROME} right-3 min-[601px]:right-4`}
								aria-label="下一張"
								aria-controls={slideRegionId}
								onClick={() => stepByArrow(1)}
							>
								<AppIcon name="chevron-right" size={22} />
							</button>
						</>
					) : null}
				</div>
				<div
					className={
						size === "compact"
							? "mt-auto min-h-2.5 pt-[var(--hkui-space-1)] flex justify-center"
							: "contents"
					}
				>
					<CarouselDots
						count={banners.length}
						active={active}
						onSelect={setActive}
						controls={slideRegionId}
						label="banner 切換"
						dotKey={(i) => banners[i].id}
						dotLabel={(i) => `第 ${i + 1} 張：${banners[i].title}`}
					/>
				</div>
			</section>
			<BannerPopupDialog
				banner={popupBanner}
				onClose={() => setPopupBanner(null)}
			/>
		</>
	);
}
