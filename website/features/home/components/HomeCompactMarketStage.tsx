"use client";

import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import EmptyStatePanel from "@/components/shared/EmptyStatePanel";
import Link from "@/components/shared/Link";
import type { OpenOriginRect } from "@/features/feed/components/PostDetailViewer";
import HomePostDetailViewer from "@/features/home/components/HomePostDetailViewer";
import { prewarmHomePostDetailViewer } from "@/features/home/components/HomePostDetailViewerLoader";
import HomeStageFeedCard from "@/features/home/components/HomeStageFeedCard";
import type { MarketProductItem } from "@/features/market/components/MarketProductsList";
import type { PostListItem } from "@/lib/api/openapi-types";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { useAppUiStore } from "@/lib/state/app-ui-store";
import { useLoginRequiredOverlay } from "@/lib/state/use-login-required-overlay";

type HomeCompactMarketStageProps = {
	posts: PostListItem[];
	products: MarketProductItem[];
};

function productImage(product: MarketProductItem) {
	return resolveCdnUrl(product.image) || product.image;
}

// Directional slide+fade for the carousel. `direction` is +1 when moving to a
// newer item (next / auto-rotate) and -1 when going back, so the outgoing card
// slides the opposite way to the incoming one. `distance` is zeroed out under
// prefers-reduced-motion to leave a plain cross-fade.
function makeSlideVariants(distance: number) {
	return {
		enter: (direction: number) => ({
			opacity: 0,
			x: direction >= 0 ? distance : -distance,
		}),
		center: { opacity: 1, x: 0 },
		exit: (direction: number) => ({
			opacity: 0,
			x: direction >= 0 ? -distance : distance,
		}),
	};
}

// Expo-out easing — quick start, soft settle.
const SLIDE_TRANSITION = { duration: 0.34, ease: [0.22, 1, 0.36, 1] } as const;
const FEED_SWIPE_THRESHOLD = 48;

export default function HomeCompactMarketStage({
	posts,
	products,
}: HomeCompactMarketStageProps) {
	const feedItems = posts.slice(0, 5);
	const productItems = products.slice(0, 9);
	const [feedIndex, setFeedIndex] = React.useState(0);
	const [feedDrag, setFeedDrag] = React.useState(0);
	const [feedDragging, setFeedDragging] = React.useState(false);
	const [productPage, setProductPage] = React.useState(0);
	const [productDir, setProductDir] = React.useState(1);
	const [openFeedId, setOpenFeedId] = React.useState<string | null>(null);
	const [openFeedOrigin, setOpenFeedOrigin] =
		React.useState<OpenOriginRect | null>(null);
	const openFeedOpenerRef = React.useRef<HTMLElement | null>(null);
	const feedDragStartX = React.useRef<number | null>(null);
	const feedSuppressClickRef = React.useRef(false);
	const feedAutoDirRef = React.useRef(1);
	const loginOpen = useAppUiStore(
		(state) => state.overlay?.name === "login-required",
	);
	const loginPayload = React.useMemo(
		() => ({
			redirect: "/",
			title: "登入後即可按讚",
			description:
				"登入 Hawkr 後，就能幫喜歡的內容按讚、收藏，並追蹤商家的最新動態。",
			actionLabel: "登入 / 註冊",
		}),
		[],
	);
	const openLoginRequired = useLoginRequiredOverlay(loginPayload);
	const reduceMotion = useReducedMotion();
	const slideVariants = React.useMemo(
		() => makeSlideVariants(reduceMotion ? 0 : 36),
		[reduceMotion],
	);
	const activeFeed = feedItems[feedIndex];
	const openFeedIndex = React.useMemo(() => {
		if (!openFeedId) return null;
		const index = feedItems.findIndex((feed) => String(feed.id) === openFeedId);
		return index >= 0 ? index : null;
	}, [feedItems, openFeedId]);
	const openFeedPost = React.useCallback(
		(
			post: PostListItem,
			originRect: DOMRect | undefined,
			opener: HTMLElement | null,
		) => {
			const index = feedItems.findIndex((feed) => feed.id === post.id);
			if (index < 0) return;
			prewarmHomePostDetailViewer();
			openFeedOpenerRef.current = opener;
			setFeedIndex(index);
			setOpenFeedId(String(post.id));
			setOpenFeedOrigin(
				originRect
					? {
							top: originRect.top,
							left: originRect.left,
							width: originRect.width,
							height: originRect.height,
						}
					: null,
			);
		},
		[feedItems],
	);
	const closeFeedPost = React.useCallback(() => {
		setOpenFeedId(null);
		setOpenFeedOrigin(null);
	}, []);
	const productPages = React.useMemo(() => {
		const pages: MarketProductItem[][] = [];
		for (let i = 0; i < productItems.length; i += 3) {
			pages.push(productItems.slice(i, i + 3));
		}
		return pages;
	}, [productItems]);
	const activeProducts = productPages[productPage] ?? productPages[0] ?? [];
	const canMoveFeed = feedItems.length > 1;
	const canMoveProducts = productPages.length > 1;
	const canMoveFeedPrev = feedIndex > 0;
	const canMoveFeedNext = feedIndex < feedItems.length - 1;

	const moveFeed = React.useCallback(
		(direction: number) => {
			const len = feedItems.length;
			if (len < 2) return;
			feedAutoDirRef.current = direction;
			setFeedIndex((current) =>
				Math.max(0, Math.min(len - 1, current + direction)),
			);
		},
		[feedItems.length],
	);

	const onFeedPointerDown = React.useCallback(
		(event: React.PointerEvent<HTMLDivElement>) => {
			if (!canMoveFeed) return;
			feedDragStartX.current = event.clientX;
			feedSuppressClickRef.current = false;
		},
		[canMoveFeed],
	);

	const onFeedPointerMove = React.useCallback(
		(event: React.PointerEvent<HTMLDivElement>) => {
			if (feedDragStartX.current === null) return;
			let dx = event.clientX - feedDragStartX.current;
			if (Math.abs(dx) <= 6) return;
			// Pointer capture changes the eventual mouse click target. Capturing
			// immediately on pointerdown made a plain desktop click land on this
			// outer track instead of HomeStageFeedCard. Capture only once the
			// gesture has clearly become a drag.
			setFeedDragging(true);
			if (!event.currentTarget.hasPointerCapture?.(event.pointerId)) {
				event.currentTarget.setPointerCapture?.(event.pointerId);
			}
			// Match the Hawkr 動態 dropdown carousel: dragging past the first/last
			// card still gives a little resistance, without revealing empty space.
			if (
				(feedIndex === 0 && dx > 0) ||
				(feedIndex === feedItems.length - 1 && dx < 0)
			)
				dx *= 0.3;
			setFeedDrag(dx);
		},
		[feedIndex, feedItems.length],
	);

	const endFeedDrag = React.useCallback(
		(event: React.PointerEvent<HTMLDivElement>) => {
			if (feedDragStartX.current === null) return;
			const dx = event.clientX - feedDragStartX.current;
			feedDragStartX.current = null;
			setFeedDragging(false);
			setFeedDrag(0);
			if (event.currentTarget.hasPointerCapture?.(event.pointerId)) {
				event.currentTarget.releasePointerCapture(event.pointerId);
			}
			// Only consume the click emitted after a real carousel swipe. Mouse
			// clicks often have a few pixels of pointer jitter, which used to set
			// the old 6px "moved" flag and make the post appear unclickable.
			feedSuppressClickRef.current = Math.abs(dx) >= FEED_SWIPE_THRESHOLD;
			if (dx <= -FEED_SWIPE_THRESHOLD) moveFeed(1);
			else if (dx >= FEED_SWIPE_THRESHOLD) moveFeed(-1);
			window.setTimeout(() => {
				feedSuppressClickRef.current = false;
			}, 0);
		},
		[moveFeed],
	);

	const suppressFeedClickAfterDrag = React.useCallback(
		(event: React.MouseEvent<HTMLDivElement>) => {
			if (!feedSuppressClickRef.current) return;
			event.preventDefault();
			event.stopPropagation();
			feedSuppressClickRef.current = false;
		},
		[],
	);

	const moveProducts = React.useCallback(
		(direction: number) => {
			setProductDir(direction);
			setProductPage((current) =>
				Math.max(0, Math.min(productPages.length - 1, current + direction)),
			);
		},
		[productPages.length],
	);

	React.useEffect(() => {
		// Pause rotation while the login modal is open so the card the user is
		// acting on doesn't slide out from under the prompt.
		if (!canMoveFeed || loginOpen || feedDragging || openFeedIndex !== null)
			return;
		const id = window.setInterval(() => {
			setFeedIndex((current) => {
				let direction = feedAutoDirRef.current;
				if (current + direction > feedItems.length - 1) direction = -1;
				else if (current + direction < 0) direction = 1;
				feedAutoDirRef.current = direction;
				return current + direction;
			});
		}, 6000);
		return () => window.clearInterval(id);
	}, [canMoveFeed, feedItems.length, loginOpen, feedDragging, openFeedIndex]);

	React.useEffect(() => {
		if (feedIndex >= feedItems.length) setFeedIndex(0);
	}, [feedIndex, feedItems.length]);

	React.useEffect(() => {
		if (productPage >= productPages.length) setProductPage(0);
	}, [productPage, productPages.length]);

	return (
		<div className="home-fixed-section relative z-[2] grid grid-cols-[minmax(0,1.08fr)_minmax(360px,0.92fr)] gap-3 max-[1080px]:grid-cols-1">
			<section
				className="home-section-surface flex min-h-0 flex-col overflow-hidden rounded-[24px] bg-hk-home-paper/88 p-4 shadow-hk-card"
				aria-label="產地大小事"
			>
				<header className="mb-3 flex items-center justify-between gap-4">
					<div>
						<h2 className="m-0 text-2xl font-black leading-tight text-hk-heading">
							產地大小事
						</h2>
					</div>
					<div className="flex items-center gap-2">
						<button
							type="button"
							aria-label="上一則"
							disabled={!canMoveFeedPrev}
							className="grid size-9 place-items-center rounded-full border border-hk-border-soft bg-hk-surface-card text-xl text-hk-heading disabled:cursor-not-allowed disabled:opacity-40"
							onClick={() => moveFeed(-1)}
						>
							‹
						</button>
						<span className="text-xs font-bold text-hk-muted">
							{activeFeed ? feedIndex + 1 : 0} / {feedItems.length}
						</span>
						<button
							type="button"
							aria-label="下一則"
							disabled={!canMoveFeedNext}
							className="grid size-9 place-items-center rounded-full border border-hk-border-soft bg-hk-surface-card text-xl text-hk-heading disabled:cursor-not-allowed disabled:opacity-40"
							onClick={() => moveFeed(1)}
						>
							›
						</button>
					</div>
				</header>
				{activeFeed ? (
					<div className="relative flex-1 min-h-0 overflow-hidden">
						{/* All feed cards live in one flex track, same as the Hawkr 動態
						    dropdown carousel, so drag reveals the adjacent card before
						    snapping to it.
						    biome-ignore lint/a11y/noStaticElementInteractions: swipe is a progressive enhancement; the arrow buttons and the card's own keyboard handler remain the accessible controls. */}
						<div
							className="flex h-full w-full gap-[16px] select-none touch-pan-y [&_img]:pointer-events-none"
							style={{
								transform: `translate3d(calc(${-feedIndex * 100}% - ${feedIndex * 16}px + ${feedDrag}px),0,0)`,
								transition: feedDragging
									? "none"
									: "transform 460ms var(--hkui-ease)",
								cursor: canMoveFeed
									? feedDragging
										? "grabbing"
										: "grab"
									: "default",
							}}
							onPointerDown={onFeedPointerDown}
							onPointerMove={onFeedPointerMove}
							onPointerUp={endFeedDrag}
							onPointerCancel={endFeedDrag}
							onClickCapture={suppressFeedClickAfterDrag}
							onDragStart={(event) => event.preventDefault()}
						>
							{feedItems.map((feed, index) => (
								<div
									key={feed.id}
									data-home-feed-active={
										feed.id === activeFeed.id ? "true" : "false"
									}
									className="h-full shrink-0 grow-0 basis-full"
								>
									<HomeStageFeedCard
										post={feed}
										imageSrc={
											resolveCdnUrl(feed.imageKeys[0]) || feed.imageKeys[0]
										}
										imagePriority={index === 0}
										imageLoading={index === feedIndex ? "eager" : "lazy"}
										onRequireLogin={openLoginRequired}
										onPrewarmPostDetail={prewarmHomePostDetailViewer}
										onOpenPost={openFeedPost}
									/>
								</div>
							))}
						</div>
					</div>
				) : (
					<EmptyStatePanel
						className="home-compact-empty-state flex-1 min-h-[260px]"
						icon="document"
						title="目前沒有真實產地動態"
						description="這個畫面目前是空的。等商家發布新貼文後，最新採收與到貨消息會自動顯示在這裡。"
						iconSize={116}
					/>
				)}
			</section>
			{openFeedIndex !== null ? (
				<HomePostDetailViewer
					posts={feedItems}
					initialIndex={openFeedIndex}
					onClose={closeFeedPost}
					hasMore={false}
					openRect={openFeedOrigin}
					returnFocusRef={openFeedOpenerRef}
				/>
			) : null}

			<section
				className="home-section-surface flex min-h-0 flex-col overflow-hidden rounded-[24px] bg-hk-home-paper/88 p-4 shadow-hk-card"
				aria-label="小編推薦"
			>
				<header className="mb-3 flex items-center justify-between gap-3">
					<h2 className="m-0 text-2xl font-black leading-tight text-hk-heading">
						小編推薦
					</h2>
					<div className="flex items-center gap-2">
						<button
							type="button"
							aria-label="上一頁"
							disabled={!canMoveProducts}
							className="grid size-9 place-items-center rounded-full border border-hk-border-soft bg-hk-surface-card text-xl text-hk-heading disabled:cursor-not-allowed disabled:opacity-40"
							onClick={() => moveProducts(-1)}
						>
							‹
						</button>
						<button
							type="button"
							aria-label="下一頁"
							disabled={!canMoveProducts}
							className="grid size-9 place-items-center rounded-full border border-hk-border-soft bg-hk-surface-card text-xl text-hk-heading disabled:cursor-not-allowed disabled:opacity-40"
							onClick={() => moveProducts(1)}
						>
							›
						</button>
						<Link
							href="/market/products"
							className="rounded-full border border-hk-border-soft bg-hk-surface-card px-4 py-2 text-xs font-black text-hk-heading no-underline"
						>
							查看全部
						</Link>
					</div>
				</header>
				{/* Fixed-height stage = 3 product cards (86px each) + 2 gaps (8px) =
				    274px. Pinning the height on this stable wrapper (not the
				    per-page motion.div) stops the row from collapsing while
				    AnimatePresence swaps pages, and reserves all three slots so a
				    short last page (e.g. 2 items) doesn't shrink the block. The
				    left feed column matches via the grid's items-stretch. */}
				<div className="min-h-[274px] flex-1">
					<AnimatePresence mode="wait" custom={productDir} initial={false}>
						{activeProducts.length > 0 ? (
							<motion.div
								key={productPage}
								custom={productDir}
								variants={slideVariants}
								initial="enter"
								animate="center"
								exit="exit"
								transition={SLIDE_TRANSITION}
								className="grid h-full grid-cols-1 content-start gap-2"
							>
								{activeProducts.map((product) => (
									<Link
										key={product.id}
										href={product.href}
										className="flex items-center gap-3 rounded-hk-lg border border-hk-border-soft bg-hk-surface-card p-2.5 text-inherit no-underline shadow-[0_8px_18px_-18px_rgba(76,50,20,0.25)] transition-transform hover:-translate-y-0.5"
									>
										<div className="grid size-16 shrink-0 place-items-center overflow-hidden rounded-hk-md bg-hk-surface-muted">
											<AppImage
												src={productImage(product)}
												alt={product.name}
												width={64}
												height={64}
												preset="thumb"
												sizes="64px"
												loading="eager"
												// 64px 縮圖不是 LCP 候選；low 只拿掉 React 19 SSR 自動
												// 產生的 preload 與抓取優先權，仍在首次繪製就下載。
												fetchPriority="low"
												className="size-full object-cover"
												// The unlayered global img baseline uses height:auto. Pin
												// both axes inline for this intentionally square crop so it
												// cannot fall back to the source's 4:3 ratio (64×48) or emit
												// Next's one-axis aspect-ratio warning.
												style={{ width: "100%", height: "100%" }}
											/>
										</div>
										<div className="min-w-0 flex-1">
											<div className="truncate text-[0.625rem] font-black uppercase tracking-[0.08em] text-hk-muted">
												{product.sellerName}
											</div>
											<div className="truncate text-sm font-black leading-snug text-hk-heading">
												{product.name}
											</div>
											<div className="mt-1 flex items-baseline gap-2">
												<strong className="text-sm font-black text-hk-primary">
													{product.price}
												</strong>
												{product.originalPrice ? (
													<s className="text-xs text-hk-faint">
														{product.originalPrice}
													</s>
												) : null}
											</div>
										</div>
										<span className="px-1 text-2xl font-light text-hk-faint">
											›
										</span>
									</Link>
								))}
							</motion.div>
						) : (
							<EmptyStatePanel
								className="home-compact-empty-state h-full w-full"
								icon="box"
								title="目前沒有可顯示的推薦商品"
								description="這個畫面目前是空的。商品上架後，會自動出現在這裡。"
								iconSize={104}
							/>
						)}
					</AnimatePresence>
				</div>
			</section>
		</div>
	);
}
