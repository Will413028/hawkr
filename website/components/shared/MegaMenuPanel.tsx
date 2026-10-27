"use client";

import { useQuery } from "@tanstack/react-query";
import dynamic from "next/dynamic";
import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import BannerLoadingSkeleton from "@/components/shared/banner/BannerLoadingSkeleton";
import {
	FEATURE_CARD_BG,
	FEATURE_CARD_CHROME,
	FEATURE_CARD_WELL,
} from "@/components/shared/banner/featureCard";
import CarouselDots from "@/components/shared/CarouselDots";
import Link from "@/components/shared/Link";
import RotatingProduceIcon from "@/components/shared/RotatingProduceIcon";
import { useCarouselAutoplay } from "@/components/shared/useCarouselAutoplay";
import { usePrefersReducedMotion } from "@/lib/a11y/reduced-motion";
import { fetchCategoryTree } from "@/lib/api";
import type { PostListItem } from "@/lib/api/openapi-types";
import { fetchTopWeeklyPosts } from "@/lib/api/posts";
import type { NavItem, NavMegaLink } from "@/lib/content/data";
import { postCardPresetForMedia, resolveCdnUrl } from "@/lib/media/cdn";
import { fetchTrendingPostTags } from "@/lib/post-tags/repo";
import { queryKeys } from "@/lib/query/keys";
import { fetchPopularHashtags } from "@/lib/search/repo";
import type { MegaMenuDynamicLinks } from "./MegaMenu";

interface CategoryTreeNode {
	id?: string | number;
	name?: string;
	children?: CategoryTreeNode[];
}

async function fetchMegaMenuDynamicLinks(): Promise<MegaMenuDynamicLinks> {
	const [tree, hashtags, topPosts] = await Promise.all([
		fetchCategoryTree().catch(() => []),
		fetchPopularHashtags({ limit: 5 }).catch(() => []),
		fetchTopWeeklyPosts(5).catch(() => []),
	]);
	const categoryLinks: NavMegaLink[] = (
		Array.isArray(tree) ? (tree as CategoryTreeNode[]) : []
	)
		.filter((node) => node?.name)
		.map((node) => ({
			label: String(node.name),
			href: `/market/products?category=${encodeURIComponent(String(node.name))}`,
		}));
	const hashtagLinks: NavMegaLink[] = hashtags.map((tag) => ({
		label: tag.label || `#${tag.name}`,
		href: `/market/products?hashtags=${encodeURIComponent(tag.name)}`,
	}));
	return { categories: categoryLinks, hashtags: hashtagLinks, topPosts };
}

interface TrendingHashtagLink {
	label: string;
	href: string;
	count: number;
}

function withTrendingHashtags(
	items: NavItem[],
	hashtags: TrendingHashtagLink[],
): NavItem[] {
	return items.map((item) => {
		if (item.label !== "Hawkr 動態" || !item.panel) return item;
		return {
			...item,
			panel: {
				...item.panel,
				columns: item.panel.columns.map((column) => {
					if (!column.title.toLowerCase().includes("hashtag")) return column;
					return {
						...column,
						links: hashtags.map((tag) => ({
							label: tag.label,
							href: tag.href,
							desc: `最近 7 天 ${tag.count.toLocaleString()} 則動態`,
						})),
					};
				}),
			},
		};
	});
}

const BannerCarousel = dynamic(
	() => import("@/components/shared/banner/BannerCarousel"),
	{
		ssr: false,
		// Covers the brief gap before the carousel chunk mounts. A pulsing
		// skeleton in the exact feature-card footprint (no animation, so the
		// truck JSON stays out of the global navbar bundle) — once mounted the
		// carousel takes over with the truck loader, then the real banner.
		loading: () => <BannerLoadingSkeleton />,
	},
);

export interface MegaMenuPanelProps {
	item: NavItem;
	dynamicLinks?: MegaMenuDynamicLinks;
	close: () => void;
}

// Only mounted inside an opened menu. Keep gateway reads out of the root
// server streaming boundary and off the idle Navbar hydration path.
export default function MegaMenuPanel({
	item,
	dynamicLinks: initialDynamicLinks,
	close,
}: MegaMenuPanelProps) {
	const megaMenuQuery = useQuery({
		queryKey: queryKeys.navbar.megaMenu(),
		queryFn: fetchMegaMenuDynamicLinks,
		staleTime: 5 * 60 * 1000,
		enabled: initialDynamicLinks == null,
	});
	const dynamicLinks = initialDynamicLinks ?? megaMenuQuery.data;
	const trendingQuery = useQuery({
		queryKey: queryKeys.postTags.trending({ limit: 5 }),
		queryFn: ({ signal }) => fetchTrendingPostTags({ limit: 5, signal }),
		enabled: item.label === "Hawkr 動態",
	});
	const trendingHashtags = React.useMemo<TrendingHashtagLink[]>(
		() =>
			(trendingQuery.data ?? []).map((tag) => ({
				label: tag.label,
				href: `/feed?hashtag=${encodeURIComponent(tag.name)}`,
				count: tag.count,
			})),
		[trendingQuery.data],
	);
	const activeItem = React.useMemo(
		() => withTrendingHashtags([item], trendingHashtags)[0],
		[item, trendingHashtags],
	);
	return (
		<div data-mega-menu-content className="contents">
			{activeItem?.panel &&
				(() => {
					const allCols = activeItem.panel.columns;
					// All text columns (商品分類 + 熱門 hashtag) stack vertically in
					// the left rail; the right slot is reserved for the full banner
					// block — matching the Hawkr 動態 panel's left-rail / right-feature
					// split.

					const renderColumn = (col: (typeof allCols)[number]) => {
						// When a column is marked `source`, swap its static links
						// for the prefetched list passed in via props. Empty source
						// data renders a tasteful empty state (icon + message)
						// inside the column instead of hiding or showing fake links.
						const source = col.source;
						const renderedLinks: NavMegaLink[] = source
							? (dynamicLinks?.[source] ?? [])
							: col.links;
						const isDynamic = !!source;
						const showEmptyState = isDynamic && renderedLinks.length === 0;
						return (
							<section key={col.title} data-source={source}>
								<h3 className="text-xs font-bold tracking-[0.08em] uppercase text-foreground-soft m-0 mb-5">
									{col.title}
								</h3>
								{isDynamic &&
								megaMenuQuery.isPending &&
								!initialDynamicLinks ? (
									<div
										role="status"
										className="min-h-[96px] text-sm text-foreground-soft"
									>
										選單資料載入中…
									</div>
								) : showEmptyState && source ? (
									<MegaMenuColumnEmpty source={source} />
								) : (
									<ul
										className={
											source === "categories"
												? "list-none m-0 p-0 grid grid-cols-2 gap-x-4 gap-y-[2px]"
												: source === "hashtags"
													? "list-none m-0 p-0 flex flex-wrap gap-2"
													: "list-none m-0 p-0 flex flex-col gap-[2px]"
										}
									>
										{renderedLinks.map((link) =>
											source === "hashtags" ? (
												// Soft sage pill chips matching the Hawkr 動態 feed's
												// hashtag UI (.post-viewer__hashtag): rounded, brand-
												// green tint, wrapping horizontally — instead of a
												// plain vertical link list.
												<li key={link.href + link.label}>
													<Link
														href={link.href}
														onClick={close}
														className="inline-flex items-center h-7 px-2.5 rounded-full text-[length:var(--hkui-text-tiny)] font-bold no-underline bg-[var(--page-mega-tag-bg)] text-[var(--page-mega-tag-fg)] transition-[background,transform] duration-[var(--hkui-dur-fast)] ease-[var(--hkui-ease)] hover:bg-[var(--page-mega-tag-hover-bg)] hover:-translate-y-px motion-reduce:transition-none motion-reduce:hover:translate-y-0"
													>
														{link.label}
													</Link>
												</li>
											) : (
												<li key={link.href + link.label}>
													<Link
														href={link.href}
														onClick={close}
														className="flex flex-col gap-[2px] py-2 text-foreground no-underline transition-[color,transform] duration-[var(--hkui-dur-fast)] ease-[var(--hkui-ease)] hover:text-hkui-primary hover:translate-x-[2px]"
													>
														<span className="text-base font-medium inline-flex items-center gap-2">
															{link.label}
															{link.badge && (
																<span className="text-xs font-bold tracking-[0.04em] px-1.5 py-0.5 rounded-[var(--radius-pill)] bg-danger text-danger-foreground">
																	{link.badge}
																</span>
															)}
														</span>
														{link.desc && (
															<span className="text-sm text-foreground-soft leading-[var(--hkui-line-small)]">
																{link.desc}
															</span>
														)}
													</Link>
												</li>
											),
										)}
									</ul>
								)}
							</section>
						);
					};

					return (
						// Panel surface: warm off-white base with barely-there sage
						// (top-left) and honey (bottom-right) corner glows plus a soft
						// top sheen for paper-like depth in light mode; dark mode drops
						// the glow layers and switches to the glass tint that pairs
						// with the dark:backdrop-blur utilities below.
						<>
							<div className="flex-[0_0_280px] grid grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-[var(--hkui-space-8)] [@media(max-width:840px)]:w-full [@media(max-width:840px)]:flex-none max-[768px]:grid-cols-1 max-[768px]:gap-[var(--hkui-space-5)]">
								{allCols.map(renderColumn)}
							</div>
							{activeItem.panel.banners ? (
								<aside className="flex-[0_0_var(--mega-feature-w,440px)] w-[var(--mega-feature-w,440px)] bg-none bg-transparent p-0 rounded-none overflow-visible flex flex-col gap-[var(--hkui-space-6)] [@media(max-width:840px)]:flex-[1_1_auto] [@media(max-width:840px)]:w-full">
									<BannerCarousel size="compact" heading="最新消息" />
								</aside>
							) : activeItem.panel.weeklyPost ? (
								dynamicLinks?.topPosts && dynamicLinks.topPosts.length > 0 ? (
									<WeeklyTopPostCarousel
										posts={dynamicLinks.topPosts}
										onNavigate={close}
									/>
								) : megaMenuQuery.isPending && !initialDynamicLinks ? (
									<div
										role="status"
										className="w-[var(--mega-feature-w,440px)] max-w-full min-h-[320px] text-sm text-foreground-soft"
									>
										主題載入中…
									</div>
								) : (
									<WeeklyTopPostEmpty onNavigate={close} />
								)
							) : activeItem.panel.feature ? (
								<aside className="flex-[0_0_var(--mega-feature-w,440px)] w-[var(--mega-feature-w,440px)] bg-[var(--brand-pattern-cream)] bg-[var(--hkui-content2)] rounded-[var(--hkui-radius-xl)] p-[var(--hkui-space-6)] flex flex-col gap-[var(--hkui-space-3)] overflow-hidden [@media(max-width:840px)]:flex-[1_1_auto] [@media(max-width:840px)]:w-full">
									{activeItem.panel.feature.image && (
										<AppImage
											src={resolveCdnUrl(activeItem.panel.feature.image)}
											alt=""
											width={440}
											height={248}
											sizes="440px"
											className="w-full aspect-[16/9] rounded-[var(--hkui-radius-md)] object-cover"
										/>
									)}
									<div className="flex flex-col gap-[var(--hkui-space-2)]">
										{activeItem.panel.feature.eyebrow && (
											<span className="text-[length:var(--hkui-text-tiny)] font-bold tracking-[0.08em] uppercase text-[var(--hkui-primary)]">
												{activeItem.panel.feature.eyebrow}
											</span>
										)}
										<strong className="font-[var(--hkui-font-display)] text-[length:var(--hkui-text-large)] tracking-[var(--hkui-tracking-tight)] text-[var(--hkui-foreground)]">
											{activeItem.panel.feature.title}
										</strong>
										<p className="text-[length:var(--hkui-text-small)] text-[var(--hkui-foreground-muted)] m-0 leading-[var(--hkui-line-medium)]">
											{activeItem.panel.feature.desc}
										</p>
										<Link
											href={activeItem.panel.feature.href}
											onClick={close}
											className="self-start text-[length:var(--hkui-text-small)] font-semibold text-[var(--hkui-primary)] no-underline transition-[color] duration-[var(--hkui-dur-fast)] ease-[var(--hkui-ease)] hover:text-[var(--hkui-primary-700)]"
										>
											{activeItem.panel.feature.cta} →
										</Link>
									</div>
								</aside>
							) : null}
						</>
					);
				})()}
		</div>
	);
}

/**
 * Right-slot carousel for the Hawkr 動態 panel — cycles through the
 * week's top-viewed posts. Uses `.mega-panel__feature` chrome so the
 * card visually matches the legacy static feature card. Dot indicators
 * stay local because the shared banner carousel CSS moved to Tailwind.
 *
 * Autoplay: a relaxed 6s tick, held while the pointer or focus is on the
 * carousel itself (not merely inside the panel — the panel is hover-opened,
 * so panel-level hover pausing really would mean "never autoplay"), while the
 * tab is hidden, and off entirely under `prefers-reduced-motion`. WCAG 2.2.2
 * needs a way to stop content that updates itself for longer than 5s; the
 * previous version offered none.
 */
const WEEKLY_AUTOPLAY_MS = 6000;
/** Horizontal travel (px) past which a pointer/touch swipe commits to the
 *  next / previous post instead of snapping back. */
const WEEKLY_SWIPE_THRESHOLD = 48;

function WeeklyTopPostCarousel({
	posts,
	onNavigate,
}: {
	posts: PostListItem[];
	onNavigate: () => void;
}) {
	const [active, setActive] = React.useState(0);
	const [drag, setDrag] = React.useState(0);
	const [dragging, setDragging] = React.useState(false);
	const startX = React.useRef<number | null>(null);
	// Tracks whether the current gesture moved far enough to count as a
	// swipe — used to swallow the click on <Link> so a drag never also
	// navigates to the post.
	const movedRef = React.useRef(false);
	const multi = posts.length > 1;
	// `aria-controls` target for the dot tablist.
	const slideRegionId = React.useId();
	const reducedMotion = usePrefersReducedMotion();

	const step = React.useCallback(
		(dir: number) => {
			// Swipe stops at the ends; autoplay below ping-pongs to loop.
			setActive((i) => Math.max(0, Math.min(posts.length - 1, i + dir)));
		},
		[posts.length],
	);

	// Autoplay ping-pongs (0→…→last→…→0) so each tick is a single adjacent
	// slide — a modulo wrap would slide the whole track back across every card.
	const dirRef = React.useRef(1);
	const advance = React.useCallback(() => {
		setActive((i) => {
			let dir = dirRef.current;
			if (i + dir > posts.length - 1) dir = -1;
			else if (i + dir < 0) dir = 1;
			dirRef.current = dir;
			return i + dir;
		});
	}, [posts.length]);
	const autoplay = useCarouselAutoplay<HTMLElement>({
		count: posts.length,
		intervalMs: WEEKLY_AUTOPLAY_MS,
		paused: dragging,
		onTick: advance,
	});

	// Snap back to a valid index if the source list shrinks.
	React.useEffect(() => {
		if (active >= posts.length) setActive(0);
	}, [active, posts.length]);

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
		// Rubber-band resistance at the ends — nothing to reveal past the
		// first / last post.
		if ((active === 0 && dx > 0) || (active === posts.length - 1 && dx < 0))
			dx *= 0.3;
		setDrag(dx);
	};
	const endDrag = (e: React.PointerEvent) => {
		if (startX.current === null) return;
		const dx = e.clientX - startX.current;
		startX.current = null;
		setDragging(false);
		setDrag(0);
		if (dx <= -WEEKLY_SWIPE_THRESHOLD) step(1);
		else if (dx >= WEEKLY_SWIPE_THRESHOLD) step(-1);
	};

	if (posts.length === 0) return null;
	return (
		<aside
			ref={autoplay.containerRef}
			className={`group/feat flex-[0_0_var(--mega-feature-w,440px)] w-[var(--mega-feature-w,440px)] p-[var(--hkui-space-5)] flex flex-col gap-[var(--hkui-space-4)] overflow-hidden [@media(max-width:840px)]:flex-[1_1_auto] [@media(max-width:840px)]:w-full ${FEATURE_CARD_CHROME}`}
			style={FEATURE_CARD_BG}
			aria-roledescription="carousel"
			aria-label="本週主題"
		>
			<span className="sr-only" aria-live={autoplay.running ? "off" : "polite"}>
				第 {active + 1} 張，共 {posts.length} 張
			</span>
			{/* All posts live in one flex track that slides left/right, so a swipe
			 *  reads as a real card switch and every post image is already in the
			 *  DOM (no "loads after you drag" gap).
			 *  biome-ignore lint/a11y/noStaticElementInteractions: swipe is a progressive enhancement; the dot tablist and each 看貼文 link remain the accessible, keyboard-operable controls. */}
			<div
				id={slideRegionId}
				// gap-[16px] puts space between slides; the translate offsets by
				// that same 16px per slide so the active card still lands flush.
				className="flex w-full gap-[16px] select-none touch-pan-y [&_img]:pointer-events-none"
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
				{posts.map((p) => {
					const snippet =
						(p.content || "").trim().slice(0, 72) ||
						`${p.sellerName} 的最新貼文`;
					const viewLabel = (p.viewCount ?? 0).toLocaleString();
					return (
						<div
							key={p.id}
							className="shrink-0 grow-0 basis-full flex flex-col gap-[var(--hkui-space-4)]"
						>
							<div className={`${FEATURE_CARD_WELL} aspect-[4/3]`}>
								{p.imageKeys[0] ? (
									<AppImage
										src={resolveCdnUrl(p.imageKeys[0])}
										alt=""
										fill
										preset={postCardPresetForMedia(p.imageKeys[0])}
										sizes="440px"
										className="object-contain"
									/>
								) : (
									<div className="absolute inset-0 grid place-items-center bg-[var(--page-mega-placeholder-bg)]">
										<svg
											className="w-10 h-10 text-[var(--page-accent-marker)] opacity-45"
											viewBox="0 0 24 24"
											fill="none"
											aria-hidden="true"
										>
											<path
												d="M4 5h16v11H7l-3 3V5z"
												stroke="currentColor"
												strokeWidth="1.6"
												strokeLinejoin="round"
											/>
										</svg>
									</div>
								)}
								{/* gentle bottom scrim so the overlaid chips stay legible
								    over busy product photos */}
								<div className="absolute inset-x-0 bottom-0 h-1/3 bg-[linear-gradient(to_top,var(--page-mega-scrim),transparent)] pointer-events-none" />
								<span className="absolute top-2.5 left-2.5 inline-flex items-center gap-1 rounded-[var(--radius-pill)] bg-[var(--page-mega-chip-bg)] backdrop-blur-sm px-2 py-0.5 text-xs font-bold tracking-[0.04em] text-white">
									本週主題
								</span>
								<span className="absolute bottom-2.5 right-2.5 inline-flex items-center gap-1 rounded-[var(--radius-pill)] bg-[var(--page-mega-chip-bg)] backdrop-blur-sm px-2 py-0.5 text-xs font-semibold text-white">
									<svg
										className="w-3 h-3"
										viewBox="0 0 24 24"
										fill="none"
										aria-hidden="true"
									>
										<path
											d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"
											stroke="currentColor"
											strokeWidth="1.6"
										/>
										<circle
											cx="12"
											cy="12"
											r="2.6"
											stroke="currentColor"
											strokeWidth="1.6"
										/>
									</svg>
									{viewLabel}
								</span>
							</div>
							<div className="flex flex-col gap-[var(--hkui-space-2)]">
								<strong className="font-[var(--hkui-font-display)] text-[length:var(--hkui-text-large)] tracking-[var(--hkui-tracking-tight)] text-[var(--hkui-foreground)]">
									{p.sellerName}
								</strong>
								<p className="text-[length:var(--hkui-text-small)] text-[var(--hkui-foreground-muted)] m-0 leading-[var(--hkui-line-medium)] line-clamp-2">
									{snippet}
								</p>
								<Link
									href={`/feed?post=${encodeURIComponent(String(p.id))}`}
									onClick={(e) => {
										// A swipe ends with a synthetic click — swallow it so
										// dragging the track never navigates.
										if (movedRef.current) {
											e.preventDefault();
											return;
										}
										onNavigate();
									}}
									draggable={false}
									className="group/cta mt-0.5 self-start inline-flex items-center gap-1 text-[length:var(--hkui-text-small)] font-semibold text-[var(--hkui-primary)] no-underline transition-[color] duration-[var(--hkui-dur-fast)] ease-[var(--hkui-ease)] hover:text-[var(--hkui-primary-700)]"
								>
									看貼文
									<span
										aria-hidden="true"
										className="transition-transform duration-[var(--hkui-dur-fast)] ease-[var(--hkui-ease)] group-hover/cta:translate-x-0.5"
									>
										→
									</span>
								</Link>
							</div>
						</div>
					);
				})}
			</div>
			<CarouselDots
				count={posts.length}
				active={active}
				onSelect={setActive}
				controls={slideRegionId}
				label="本週主題切換"
				dotKey={(i) => posts[i].id}
				dotLabel={(i) => `第 ${i + 1} 篇：${posts[i].sellerName}`}
				className="mt-auto pt-[var(--hkui-space-1)]"
			/>
		</aside>
	);
}

/**
 * Empty-state for the Hawkr 動態 panel's right slot when /public/posts
 * returns nothing. Mirrors the carousel's aside chrome (same width /
 * cream card) so the panel keeps its two-pane balance, then offers a
 * quiet illustration + a CTA into the feed instead of fabricated posts.
 */
function WeeklyTopPostEmpty({ onNavigate }: { onNavigate: () => void }) {
	return (
		<aside
			className={`flex-[0_0_var(--mega-feature-w,440px)] w-[var(--mega-feature-w,440px)] p-[var(--hkui-space-6)] flex flex-col items-center justify-center text-center gap-[var(--hkui-space-3)] min-h-[220px] overflow-hidden [@media(max-width:840px)]:flex-[1_1_auto] [@media(max-width:840px)]:w-full [@media(max-width:840px)]:min-h-[160px] ${FEATURE_CARD_CHROME}`}
			style={FEATURE_CARD_BG}
			role="status"
		>
			<span className="grid place-items-center w-14 h-14 rounded-full bg-[var(--page-mega-empty-disc-bg)]">
				<svg
					className="w-7 h-7 text-[var(--page-accent-marker)] opacity-70"
					viewBox="0 0 24 24"
					fill="none"
					aria-hidden="true"
				>
					<title>暫無貼文</title>
					<path
						d="M4 5h16v11H7l-3 3V5z"
						stroke="currentColor"
						strokeWidth="1.6"
						strokeLinejoin="round"
					/>
					<path
						d="M8 9h8M8 12h5"
						stroke="currentColor"
						strokeWidth="1.6"
						strokeLinecap="round"
						opacity="0.6"
					/>
				</svg>
			</span>
			<div className="flex flex-col gap-1">
				<strong className="font-[var(--hkui-font-display)] text-[length:var(--hkui-text-medium)] tracking-[var(--hkui-tracking-tight)] text-[var(--hkui-foreground)]">
					本週還沒有熱門貼文
				</strong>
				<p className="text-[length:var(--hkui-text-small)] text-[var(--hkui-foreground-muted)] m-0 leading-[var(--hkui-line-medium)]">
					小農與賣家正在準備新鮮內容，先逛逛 Hawkr 動態吧。
				</p>
			</div>
			<Link
				href="/feed"
				onClick={onNavigate}
				className="group/cta mt-0.5 inline-flex items-center gap-1 text-[length:var(--hkui-text-small)] font-semibold text-[var(--hkui-primary)] no-underline transition-[color] duration-[var(--hkui-dur-fast)] ease-[var(--hkui-ease)] hover:text-[var(--hkui-primary-700)]"
			>
				逛逛動態
				<span
					aria-hidden="true"
					className="transition-transform duration-[var(--hkui-dur-fast)] ease-[var(--hkui-ease)] group-hover/cta:translate-x-0.5"
				>
					→
				</span>
			</Link>
		</aside>
	);
}

/**
 * Empty-state shown inside a dynamic megamenu column when the backend
 * returns nothing. Quiet outlined-leaf icon + soft sage label — sits
 * comfortably inside the cream panel without competing with the live
 * columns next to it.
 */
function MegaMenuColumnEmpty({ source }: { source: string }) {
	const message =
		source === "hashtags"
			? "暫無熱門 hashtag"
			: source === "categories"
				? "暫無分類"
				: "暫無資料";
	return (
		<div
			className="flex flex-col items-center justify-center gap-[10px] py-6 px-3 min-h-[96px] text-[var(--page-mega-empty-fg)] opacity-70 text-center"
			role="status"
		>
			<RotatingProduceIcon
				size={32}
				className="w-8 h-8 text-[var(--page-accent-marker)] opacity-55 dark:opacity-50"
				aria-hidden="true"
			/>
			<span className="text-[0.8125rem] tracking-[0.4px]">{message}</span>
		</div>
	);
}
