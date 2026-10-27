"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import AppIcon from "@/components/shared/AppIcon";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import CommentsBottomSheet from "@/features/feed/components/CommentsBottomSheet";
import PostComments from "@/features/feed/components/PostComments";
import { zoomOriginCenter, zoomShrinkStyle } from "@/lib/animation/zoom";
import type { PostDetail, PostListItem } from "@/lib/api/openapi-types";
import { FALLBACK_POST_IMAGE, sharePost } from "@/lib/api/posts";
import { useAuth } from "@/lib/auth/context";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import { usePostInteractions } from "@/lib/hooks/useSocial";
import { buildImageUrl, isGifMedia, ossImageUrl } from "@/lib/media/cdn";
import { sellerPath } from "@/lib/seo/urls";
import { useAppUiStore } from "@/lib/state/app-ui-store";
import HashtagText from "./HashtagText";
import PostTagChips from "./PostTagChips";

/** Box (viewport px) of the grid thumbnail the viewer was opened from. */
export interface OpenOriginRect {
	top: number;
	left: number;
	width: number;
	height: number;
}

interface PostDetailViewerProps {
	posts: PostListItem[];
	initialIndex: number | null;
	onClose: () => void;
	onNeedMore?: () => Promise<void> | void;
	hasMore?: boolean;
	/** The tapped card's thumbnail box. The whole detail surface scales out of
	 *  it on open and back into it on close. Null → plain fade (e.g. deep link). */
	openRect?: OpenOriginRect | null;
}

const CloseIcon = () => (
	<svg
		width="22"
		height="22"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		strokeWidth="2.2"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<path d="M18 6 6 18M6 6l12 12" />
	</svg>
);
const ChevronUpIcon = () => (
	<svg
		width="20"
		height="20"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		strokeWidth="2.4"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<path d="m18 15-6-6-6 6" />
	</svg>
);

function formatCount(n: number): string {
	if (n >= 10000) return `${(n / 10000).toFixed(1)}萬`;
	if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
	return String(n);
}

function formatPostProductPrice(price: string): string {
	const trimmed = price.trim();
	if (!trimmed) return "價格洽商家";
	if (/^(NT\$|NTD|[$＄]|免費)/i.test(trimmed)) return trimmed;
	if (/^\d[\d,]*(?:\.\d+)?$/.test(trimmed)) return `NT$ ${trimmed}`;
	return trimmed;
}

/** Single post page within the snap viewer. */
function PostPage({
	post,
	isActive,
	commentsActive,
	showInlineComments,
	onLikeChange,
	onRequestComments,
	onPrev,
	onNext,
	canPrev,
	canNext,
}: {
	post: PostListItem;
	isActive: boolean;
	/** False while the mobile sheet shows the SAME post's comments — keeps
	 *  this aside copy from holding a duplicate SSE stream + REST fetch. */
	commentsActive: boolean;
	/** Mobile comments are intentionally mounted only in the bottom sheet. */
	showInlineComments: boolean;
	onLikeChange?: (id: number | string, liked: boolean) => void;
	onRequestComments?: (postId: number | string) => void;
	onPrev?: () => void;
	onNext?: () => void;
	canPrev?: boolean;
	canNext?: boolean;
}) {
	const [detail] = React.useState<PostDetail | null>(null);
	const {
		liked,
		likeCount,
		likeBusy,
		toggleLike: togglePostLike,
	} = usePostInteractions(post.id, {
		liked: post.isLiked,
		likeCount: post.likeCount,
	});
	const [activeImage, setActiveImage] = React.useState(0);
	const [notice, setNotice] = React.useState("");
	// Which gated button was tapped — picks the login-modal copy when the
	// gate resolves to a guest.
	const intentRef = React.useRef<"like" | "comment">("like");
	const openLoginRequired = useAppUiStore((state) => state.openLoginRequired);
	const { isLoggedIn } = useAuth();
	const { authGate, authWaiting } = useAuthGate(() => {
		const copy =
			intentRef.current === "like"
				? {
						title: "登入後即可幫小農加油",
						description: "幫你的最愛產地按個讚，讓更多人看見好食材。",
					}
				: {
						title: "登入後即可留言互動",
						description: "和小農、其他買家一起分享心得與煮食小撇步。",
					};
		openLoginRequired({
			redirect:
				typeof location !== "undefined"
					? `${location.pathname}${location.search}`
					: "/feed",
			...copy,
			actionLabel: "登入 / 註冊",
		});
	});

	const images = React.useMemo(() => {
		if (detail?.imageKeys?.length) return detail.imageKeys;
		return post.imageKeys;
	}, [detail, post.imageKeys]);

	const products = React.useMemo(() => {
		if (detail?.products?.length) return detail.products;
		return post.product ? [post.product] : [];
	}, [detail, post.product]);

	const content = detail?.content || post.content;
	const tags = detail?.tags?.length ? detail.tags : post.tags;

	React.useEffect(() => {
		if (!notice) return;
		const t = window.setTimeout(() => setNotice(""), 2400);
		return () => window.clearTimeout(t);
	}, [notice]);

	/* ── Caption collapse (mobile text-over-photo panel) ────────────────
	 * The reader panel opens showing one line of caption and grows upward
	 * when expanded, mirroring the buyer app's `_ExpandableCaption`.
	 * Desktop's two-column layout never clamps, so `captionOverflows` stays
	 * false there and the toggle never renders — no breakpoint check needed
	 * in JS, the clamp itself is the signal. */
	const [captionExpanded, setCaptionExpanded] = React.useState(false);
	const [captionOverflows, setCaptionOverflows] = React.useState(false);
	const articleScrollRef = React.useRef<HTMLDivElement>(null);

	// Collapse a page once the user swipes off it, so returning finds the photo
	// unobscured. The track keeps activeIndex ± 1 mounted, so without this an
	// expansion would survive a there-and-back swipe — `goTo` resets the
	// destination's scroll for the same reason ("each post starts fresh").
	React.useEffect(() => {
		if (isActive) return;
		setCaptionExpanded(false);
	}, [isActive]);

	React.useEffect(() => {
		// Measure only while clamped. Expanding removes the clamp, which would
		// otherwise report "fits" and pull the toggle out from under the user
		// mid-interaction (the app guards the same case with `|| expanded`).
		if (captionExpanded) return;
		const host = articleScrollRef.current;
		const caption = host?.querySelector<HTMLElement>(".post-viewer__content");
		// An empty caption renders no `.post-viewer__content` at all. Reading
		// `content` here also re-measures when the text itself changes: the clamp
		// pins `clientHeight` to one line, so the ResizeObserver below never
		// fires on a text swap even though `scrollHeight` moved.
		if (!content || !caption) {
			setCaptionOverflows(false);
			return;
		}
		const measure = () => {
			// `scrollHeight` is useless here: current Chrome implements
			// `-webkit-line-clamp` by genuinely clipping (the element blockifies
			// to `flow-root`), so scrollHeight collapses to clientHeight and the
			// caption always looks like it fits. Lift the clamp, read the natural
			// height, put it back — all in one task, so nothing paints in between
			// and there is no flicker. This is the browser equivalent of the app's
			// `TextPainter.didExceedMaxLines` probe.
			const restore = caption.style.webkitLineClamp;
			caption.style.webkitLineClamp = "unset";
			const natural = caption.scrollHeight;
			caption.style.webkitLineClamp = restore;
			setCaptionOverflows(natural - caption.clientHeight > 1);
		};
		measure();
		const observer = new ResizeObserver(measure);
		observer.observe(caption);
		return () => observer.disconnect();
	}, [captionExpanded, content]);

	// Tags and the product list live in the expanded state, so they justify a
	// toggle even when the caption itself fits on its single line.
	const captionToggleable =
		captionOverflows || tags.length > 0 || products.length > 0;

	/* ── How far the drawer may rise ────────────────────────────────────
	 * The open drawer stops at the TOP EDGE OF THE PHOTO, so the photo is
	 * never scrolled off behind it. Under `object-fit: contain` the painted
	 * photo is smaller than its element box and the letterbox split depends on
	 * `object-position`, so that edge is not expressible in CSS — measure it and
	 * publish the budget as `--post-drawer-max` for the stylesheet to cap
	 * against. Falls back to the card's mid-line if anything is unmeasurable. */
	const pageRef = React.useRef<HTMLElement>(null);
	const mediaImgRef = React.useRef<HTMLImageElement>(null);

	React.useEffect(() => {
		const img = mediaImgRef.current;
		const page = pageRef.current;
		if (!img || !page) return;

		const apply = () => {
			const { naturalWidth: nw, naturalHeight: nh } = img;
			// `clientWidth/Height`, NOT `getBoundingClientRect()`: the viewer root
			// carries a scale transform through the open animation, and a rect read
			// mid-zoom reports the scaled box — that shipped a budget of 269px where
			// the real one was 597px, so the drawer stopped less than halfway.
			// Layout sizes are transform-immune.
			const boxW = img.clientWidth;
			const boxH = img.clientHeight;
			if (!nw || !nh || !boxW || !boxH) return;
			// Read the ratio from the cascade rather than hardcoding it, so the
			// measurement can't drift from `object-position` in the stylesheet.
			// Percentages are what we author; a px offset would need real length
			// resolution, so centre is the safe fallback there.
			const rawY =
				getComputedStyle(img).objectPosition.split(/\s+/)[1] ?? "50%";
			const parsedY = rawY.endsWith("%") ? Number.parseFloat(rawY) / 100 : 0.5;
			const yFraction = Number.isFinite(parsedY) ? parsedY : 0.5;

			const scale = Math.min(boxW / nw, boxH / nh);
			const paintedHeight = nh * scale;
			// Distance from the photo's top edge down to the bottom of the media
			// box, which is exactly where the drawer is anchored.
			const budget = boxH - (boxH - paintedHeight) * yFraction;
			page.style.setProperty(
				"--post-drawer-max",
				`${Math.max(0, Math.round(budget))}px`,
			);
		};

		apply();
		// Set up once: the carousel swaps `src` on the SAME element, so `load`
		// already re-fires per image, and the observer catches every box change
		// (rotation, drawer toggle, breakpoint). Keying this on `activeImage`
		// would only rebuild identical listeners.
		img.addEventListener("load", apply);
		const observer = new ResizeObserver(apply);
		observer.observe(img);
		return () => {
			img.removeEventListener("load", apply);
			observer.disconnect();
		};
	}, []);

	const toggleLike = () => {
		intentRef.current = "like";
		authGate(() => {
			const next = !liked;
			onLikeChange?.(post.id, next);
			void togglePostLike();
		});
	};
	const handleComment = () => {
		intentRef.current = "comment";
		authGate(() => {
			onRequestComments?.(post.id);
		});
	};
	const handleShare = async () => {
		const shareUrl =
			typeof location !== "undefined"
				? `${location.origin}/feed#${post.id}`
				: "";
		// Guests hitting the authed share endpoint get a 401 that the global
		// authed client broadcasts as AUTH_EXPIRED_EVENT — that force-redirects
		// them off the page (lib/auth/context.tsx). Anonymous shares aren't
		// counted, so skip the API call entirely instead of hardening the 401 path.
		if (isLoggedIn) {
			sharePost(post.id);
		}
		if (typeof navigator !== "undefined" && navigator.share) {
			try {
				await navigator.share({
					title: post.sellerName,
					text: post.content,
					url: shareUrl,
				});
				return;
			} catch {
				// User cancelled or share rejected — fall through to clipboard
			}
		}
		if (typeof navigator !== "undefined" && navigator.clipboard && shareUrl) {
			try {
				await navigator.clipboard.writeText(shareUrl);
				setNotice("連結已複製，可貼到任何聊天室分享");
				return;
			} catch {
				// fallthrough
			}
		}
		setNotice("分享連結尚未支援，請手動複製網址列");
	};

	const hasMedia = images.length > 0 && Boolean(images[activeImage]);
	// Still images use the ratio-preserving detail derivative. GIFs use
	// original so no legacy browser or intermediate encoder can flatten their
	// frames in this full-screen view. The ambient backdrop below reuses this
	// exact URL, so it costs no second request.
	const mediaSrc = hasMedia
		? isGifMedia(images[activeImage])
			? buildImageUrl(images[activeImage], "original")
			: ossImageUrl(images[activeImage], "productDetail")
		: "";

	return (
		<article
			className="post-viewer__page"
			data-active={isActive}
			data-no-media={!hasMedia}
			ref={pageRef}
		>
			{hasMedia && (
				<div className="post-viewer__media">
					{/* Ambient fill for the letterbox `contain` must leave somewhere.
					    Same treatment as the seller cover's `ambient-contain` mode: an
					    over-scaled, blurred copy of the same photo, held translucent so
					    the void reads as that photo's own light instead of a flat slab.
					    Decorative — hidden from AT, and it drops out silently if the
					    media 404s (the foreground image owns the fallback). */}
					{/* biome-ignore lint/performance/noImgElement: shares the foreground image's cached URL; next/image would request a second derivative. */}
					<img
						className="post-viewer__media-ambient"
						src={mediaSrc}
						alt=""
						aria-hidden="true"
						loading={isActive ? "eager" : "lazy"}
						draggable={false}
						onError={(e) => {
							e.currentTarget.style.visibility = "hidden";
						}}
					/>
					{/* biome-ignore lint/performance/noImgElement: feed viewer needs direct drag/error control for CDN media. */}
					<img
						ref={mediaImgRef}
						className="post-viewer__media-photo"
						src={mediaSrc}
						alt={post.content?.slice(0, 30) || post.sellerName}
						// Neighbouring pages stay mounted for the snap transition —
						// don't pull their media until they scroll into view.
						loading={isActive ? "eager" : "lazy"}
						// Disable native image drag (the browser's "drag to save"
						// gesture) so it doesn't intercept our pointer events on
						// the way to the snap scroller. CSS in feed.css also
						// blocks `-webkit-user-drag` for older WebKits.
						draggable={false}
						onDragStart={(e) => e.preventDefault()}
						onError={(e) => {
							const img = e.currentTarget;
							if (img.dataset.fallbackApplied === "true") return;
							img.dataset.fallbackApplied = "true";
							img.src = ossImageUrl(FALLBACK_POST_IMAGE, "productDetail");
						}}
					/>
					{images.length > 1 && (
						<div className="post-viewer__dots" role="tablist" aria-label="圖片">
							{images.map((image, i) => (
								<button
									key={image}
									type="button"
									data-storefront-post-preview-control="true"
									role="tab"
									data-active={i === activeImage}
									aria-selected={i === activeImage}
									onClick={() => setActiveImage(i)}
									className="post-viewer__dot"
									aria-label={`第 ${i + 1} 張`}
								/>
							))}
						</div>
					)}
					{(onPrev || onNext) && (
						<div className="post-viewer__media-nav" aria-hidden="false">
							<button
								type="button"
								data-storefront-post-preview-control="true"
								className="post-viewer__nav post-viewer__nav--prev"
								onClick={onPrev}
								disabled={!canPrev}
								aria-label="上一篇"
							>
								<ChevronUpIcon />
							</button>
							<button
								type="button"
								data-storefront-post-preview-control="true"
								className="post-viewer__nav post-viewer__nav--next"
								onClick={onNext}
								disabled={!canNext}
								aria-label="下一篇"
							>
								<span
									style={{
										transform: "rotate(180deg)",
										display: "inline-flex",
									}}
								>
									<ChevronUpIcon />
								</span>
							</button>
						</div>
					)}
				</div>
			)}

			{/* Tap-anywhere-but-the-text to close, mirroring the app's
			    `IgnorePointer(ignoring: !captionExpanded)` catcher. Rendered
			    BEFORE the reader so the reader paints above it and its own text,
			    links and action buttons stay live. Pointer events still bubble to
			    the pager surface, so a swipe from here keeps flipping posts. */}
			{captionExpanded && (
				<button
					type="button"
					data-storefront-post-preview-control="true"
					className="post-viewer__caption-scrim"
					aria-label="收合貼文說明"
					onClick={() => setCaptionExpanded(false)}
				/>
			)}

			<aside
				className="post-viewer__aside"
				data-caption={captionExpanded ? "expanded" : "collapsed"}
			>
				<header className="post-viewer__seller">
					<Avatar
						src={post.logoKey}
						name={post.sellerName}
						alt=""
						entity="seller"
						fallback="empty"
					/>
					<div className="post-viewer__seller-text">
						<div className="post-viewer__seller-name">{post.sellerName}</div>
						<div className="post-viewer__seller-meta">
							{post.viewCount.toLocaleString()} 次瀏覽
						</div>
					</div>
					{/*
					 * One argument, no sellerId fallback: on a post an empty
					 * `storeCode` means the gateway could not find that seller at all
					 * (see post.go's field comment), so `/sellers/{store_id}` 404s
					 * rather than 308ing to the canonical path. Withhold the link
					 * instead of offering a dead one — the name and avatar above still
					 * render from social-service's snapshot.
					 */}
					{sellerPath(post.storeCode) ? (
						<Link
							href={sellerPath(post.storeCode)}
							// `.post-viewer__follow` is a filled `--hkui-primary` pill whose
							// label is `--hkui-primary-foreground` (0-1-0). Without this marker
							// `[data-theme="dark"] body a:not([data-hkui-button])` (0-2-2)
							// repaints that label olive `--color-ink-200` on the green surface.
							data-hkui-button=""
							className="post-viewer__follow"
						>
							查看商家
						</Link>
					) : null}
				</header>

				<div
					className="post-viewer__article-scroll"
					data-post-viewer-scroll
					id={`post-viewer-caption-${post.id}`}
					ref={articleScrollRef}
				>
					{content ? (
						<HashtagText
							as="div"
							className="post-viewer__content"
							content={content}
						/>
					) : null}
					<PostTagChips
						tags={tags}
						className="post-viewer__hashtags"
						chipClassName="post-viewer__hashtag"
					/>

					{products.length > 0 && (
						<section className="post-viewer__products">
							<div className="post-viewer__products-head">
								<h3 className="post-viewer__section-title">推薦商品</h3>
								<span className="post-viewer__products-count">
									{products.length} 件商品
								</span>
							</div>
							{products.map((p) => (
								<Link
									key={p.id}
									href={`/products/${p.id}`}
									className="post-viewer__product"
								>
									{p.imageKey ? (
										/* biome-ignore lint/performance/noImgElement: product thumbnails come from mixed CDN/API URLs. */
										<img
											className="post-viewer__product-thumb"
											src={ossImageUrl(p.imageKey, "productThumb")}
											alt={p.name}
										/>
									) : (
										<span
											className="post-viewer__product-thumb post-viewer__product-thumb--empty"
											aria-hidden="true"
										/>
									)}
									<div className="post-viewer__product-info">
										<div className="post-viewer__product-name">{p.name}</div>
										<div className="post-viewer__product-price">
											{formatPostProductPrice(p.price)}
										</div>
									</div>
									<span className="post-viewer__product-cta">查看商品</span>
								</Link>
							))}
						</section>
					)}
				</div>

				{captionToggleable ? (
					<button
						type="button"
						data-storefront-post-preview-control="true"
						className="post-viewer__caption-toggle"
						onClick={() => setCaptionExpanded((prev) => !prev)}
						aria-expanded={captionExpanded}
						aria-controls={`post-viewer-caption-${post.id}`}
					>
						{/* Both labels are deliberately four characters. The pill sits in
						    an auto-width grid column, so a shorter "收合" would shrink the
						    box and shift its left edge — the control has to stay at the
						    exact same spot for a quick open/close, and equal-length copy
						    gets that with no min-width guesswork. */}
						<span>{captionExpanded ? "收合說明" : "查看更多"}</span>
						<AppIcon
							name={captionExpanded ? "chevron-down" : "chevron-up"}
							size={16}
						/>
					</button>
				) : null}

				<footer className="post-viewer__actions">
					{/*
					 * A plain <button>, structurally identical to its two siblings — NOT
					 * the `Button` primitive. `.post-viewer__action` stacks icon over
					 * count with `flex-direction: column`, but `Button` always wraps its
					 * children in `<span class="grid"><span class="inline-flex">`, so the
					 * column never reached them: the heart and its count rendered side by
					 * side, one row centred in a 58px box, while 留言/分享 kept their
					 * counts underneath (icon at y=160 vs y=150 — visibly off).
					 *
					 * Nothing was lost by dropping it. `.post-viewer__action` already
					 * owns every property `Button` would set here — background, border,
					 * padding, colour, font-size — which is why the call site needed
					 * `isIconOnly h-auto w-auto` just to undo the primitive's geometry,
					 * and the press feedback comes from `.post-viewer__action:active`.
					 *
					 * No pending state either: `usePostInteractions` flips `liked` +
					 * `likeCount` optimistically and rolls back on failure, so the
					 * heart's fill-and-radiate animation IS the feedback. Overlapping
					 * taps are safe — the hook's `togglePendingRef` drops a second
					 * toggle while one is in flight.
					 */}
					<button
						type="button"
						className={`post-viewer__action${liked ? " is-liked" : ""}`}
						onClick={toggleLike}
						aria-pressed={liked}
						disabled={authWaiting}
						aria-busy={likeBusy || authWaiting || undefined}
						aria-label={liked ? "取消按讚" : "按讚"}
					>
						<AppIcon name="heart" fill={liked} size={26} />
						<span>{formatCount(likeCount)}</span>
					</button>
					<button
						type="button"
						className="post-viewer__action"
						onClick={handleComment}
						aria-label={`留言 ${post.commentCount} 則`}
					>
						<AppIcon name="chat" size={26} />
						<span>{formatCount(post.commentCount)}</span>
					</button>
					<button
						type="button"
						className="post-viewer__action"
						onClick={handleShare}
						aria-label="分享"
					>
						<AppIcon name="share" size={26} />
					</button>
				</footer>
				{notice ? (
					<p className="post-viewer__notice" role="status">
						{notice}
					</p>
				) : null}
				{showInlineComments ? (
					<div className="post-viewer__inline-comments">
						<PostComments postId={post.id} active={commentsActive} />
					</div>
				) : null}
			</aside>
			<AuthGateWaitModal open={authWaiting} />
		</article>
	);
}

// Apple-style snap easing — same cubic-bezier used by iOS scroll/paging
// transitions. Quick start, long tail-out so motion feels weighty without
// dragging on. See: cubic-bezier.com (0.32, 0.72, 0, 1).
const SNAP_EASING = "cubic-bezier(0.32, 0.72, 0, 1)";
const SNAP_DURATION_MS = 520;

// Open/close surface zoom — ease-out, no overshoot.
const SCALE_EASING = "cubic-bezier(0.22, 1, 0.36, 1)";

// Run before paint on the client (so the open frame is already card-sized, no
// full-size flash) but fall back to useEffect on the server to avoid the SSR
// useLayoutEffect warning.
const useIsomorphicLayoutEffect =
	typeof window !== "undefined" ? React.useLayoutEffect : React.useEffect;

// Scale clamp for the shared zoom math (lib/animation/zoom.ts): tighter than
// PageTransition's card↔page zoom because the viewer shrinks toward a feed
// thumbnail rather than a full product card.
const ZOOM_SCALE_BOUNDS = { min: 0.08, max: 0.72 };
// Snap commits if drag exceeds ~18% of page height OR flick velocity
// exceeds 0.45 px/ms (whichever fires first). Matches the feel of iOS's
// page-snap pager.
const SNAP_DISTANCE_RATIO = 0.18;
const SNAP_VELOCITY = 0.45;
// Rubber-band resistance when over-dragging past first/last post.
const RUBBER_BAND = 0.32;
// Wheel needs to accumulate this many pixels of delta before committing
// a page switch. After commit, ignore further wheel for SNAP_DURATION_MS
// so a single trackpad gesture can't skip multiple pages.
const WHEEL_THRESHOLD = 60;
// After the aside hits a scroll boundary, we ignore wheel-into-pager for
// this long so trackpad momentum / accidental over-scroll doesn't flip
// pages while the user is reading comments.
const BOUNDARY_COOLDOWN_MS = 500;
// Escape hatch: a single wheel event with |deltaY| ≥ this is treated as
// an intentional flick (typical of a discrete mouse wheel click or a
// hard trackpad gesture) and bypasses the boundary cooldown.
const WHEEL_ESCAPE_DELTA = 80;

const isInteractiveTarget = (target: EventTarget | null): boolean => {
	if (!(target instanceof HTMLElement)) return false;
	return Boolean(target.closest("button, a, input, textarea, [role='button']"));
};
// Everything that can take keyboard focus inside the dialog (close button,
// pager nav, action buttons, product links, comment inputs in the aside).
const FOCUSABLE_SELECTOR =
	"a[href], button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex='-1'])";
// Keep Tab / Shift+Tab cycling inside the dialog. Focusables inside
// aria-hidden subtrees (the inactive pager pages) are skipped so Tab only
// walks the visible post. Falls back to the dialog container itself when
// nothing tabbable exists.
const trapTabKey = (e: KeyboardEvent, dialog: HTMLElement) => {
	const focusables = Array.from(
		dialog.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR),
	).filter((el) => !el.closest("[aria-hidden='true']"));
	if (focusables.length === 0) {
		e.preventDefault();
		dialog.focus({ preventScroll: true });
		return;
	}
	const first = focusables[0];
	const last = focusables[focusables.length - 1];
	const active = document.activeElement;
	const inside = active instanceof HTMLElement && dialog.contains(active);
	if (e.shiftKey) {
		if (!inside || active === first) {
			e.preventDefault();
			last.focus();
		}
		return;
	}
	if (!inside || active === last) {
		e.preventDefault();
		first.focus();
	}
};
const scrollAreaOf = (target: EventTarget | null): HTMLElement | null => {
	if (!(target instanceof HTMLElement)) return null;
	return (
		target.closest<HTMLElement>("[data-post-viewer-scroll]") ??
		target.closest<HTMLElement>(".post-viewer__aside")
	);
};
// dy convention: positive = finger moved down = wants prev post = aside
// would need to scroll *up* (scrollTop > 0). Negative is the opposite.
const asideCanScroll = (aside: HTMLElement, dy: number): boolean => {
	if (dy > 0) return aside.scrollTop > 0;
	if (dy < 0)
		return aside.scrollTop + aside.clientHeight < aside.scrollHeight - 1;
	return false;
};

export default function PostDetailViewer({
	posts,
	initialIndex,
	onClose,
	onNeedMore,
	hasMore,
	openRect,
}: PostDetailViewerProps) {
	const open = initialIndex !== null;
	const surfaceRef = React.useRef<HTMLDivElement>(null);
	const trackRef = React.useRef<HTMLDivElement>(null);
	const dialogRef = React.useRef<HTMLDivElement>(null);
	const closeButtonRef = React.useRef<HTMLButtonElement>(null);
	// Element that had focus before the viewer opened — restored on close.
	const lastFocusedRef = React.useRef<HTMLElement | null>(null);
	const [activeIndex, setActiveIndex] = React.useState<number>(
		initialIndex ?? 0,
	);
	const [dragOffset, setDragOffset] = React.useState(0);
	const [isDragging, setIsDragging] = React.useState(false);
	// Set briefly while opening at a new initialIndex so the long jump
	// from the previously-shown post happens without an easing animation.
	const [suppressTransition, setSuppressTransition] = React.useState(false);
	const activeIndexRef = React.useRef(activeIndex);
	activeIndexRef.current = activeIndex;

	// Whole-surface zoom: the detail page scales out of the tapped card on open
	// and back into the active card on close. Pure tween (no spring → no
	// bounce), uniform scale (no distortion).
	const [reduceMotion, setReduceMotion] = React.useState(false);
	React.useEffect(() => {
		if (typeof window === "undefined") return;
		const m = window.matchMedia("(prefers-reduced-motion: reduce)");
		const sync = () => setReduceMotion(m.matches);
		sync();
		m.addEventListener("change", sync);
		return () => m.removeEventListener("change", sync);
	}, []);
	const [scalePhase, setScalePhase] = React.useState<
		"opening" | "open" | "closing"
	>("open");
	const closeRectRef = React.useRef<OpenOriginRect | null>(null);

	// Paint card-sized for one frame, then release to full size so the tween
	// has two keyframes. Plain fade (no transform) when there's no origin rect
	// or the user prefers reduced motion.
	useIsomorphicLayoutEffect(() => {
		if (!open) return;
		if (reduceMotion || !openRect) {
			setScalePhase("open");
			return;
		}
		setScalePhase("opening");
		const id = requestAnimationFrame(() => setScalePhase("open"));
		return () => cancelAnimationFrame(id);
	}, [open, openRect, reduceMotion]);

	// Run the close shrink toward the *currently* active post's grid card, then
	// hand control back to the parent (which unmounts the viewer).
	const requestClose = React.useCallback(() => {
		if (reduceMotion) {
			onClose();
			return;
		}
		const activePost = posts[activeIndexRef.current];
		const cardEl = activePost
			? document.querySelector<HTMLElement>(
					`[data-feed-card-id="${activePost.id}"]`,
				)
			: null;
		const r = cardEl?.getBoundingClientRect();
		const target: OpenOriginRect | null | undefined = r
			? { top: r.top, left: r.left, width: r.width, height: r.height }
			: openRect;
		if (!target) {
			onClose();
			return;
		}
		closeRectRef.current = target;
		setScalePhase("closing");
	}, [reduceMotion, posts, openRect, onClose]);

	// Mobile breakpoint mirrors `.post-viewer__page` grid switch at 900px
	// (feed-components.css). On mobile the comment button opens a bottom
	// sheet; on desktop it scrolls the aside in-place.
	const [isMobile, setIsMobile] = React.useState(
		() =>
			typeof window !== "undefined" &&
			window.matchMedia("(max-width: 900px)").matches,
	);
	React.useEffect(() => {
		if (typeof window === "undefined") return;
		const mql = window.matchMedia("(max-width: 900px)");
		const sync = () => setIsMobile(mql.matches);
		sync();
		mql.addEventListener("change", sync);
		return () => mql.removeEventListener("change", sync);
	}, []);
	const [sheetPostId, setSheetPostId] = React.useState<number | string | null>(
		null,
	);
	const sheetPostIdRef = React.useRef(sheetPostId);
	sheetPostIdRef.current = sheetPostId;

	const goTo = React.useCallback(
		(next: number) => {
			const clamped = Math.max(0, Math.min(posts.length - 1, next));
			setActiveIndex(clamped);
			setDragOffset(0);
			// Switching posts always reopens the destination from the top
			// (matches Reels/IG — each post starts fresh, no lingering
			// comment-scroll from a previous visit).
			const page = surfaceRef.current?.querySelector<HTMLElement>(
				`.post-viewer__page-wrap[data-idx="${clamped}"] .post-viewer__page`,
			);
			const scrollArea = page?.querySelector<HTMLElement>(
				"[data-post-viewer-scroll]",
			);
			const aside = page?.querySelector<HTMLElement>(".post-viewer__aside");
			(scrollArea ?? aside)?.scrollTo({ top: 0 });
		},
		[posts.length],
	);

	const handleRequestComments = React.useCallback(
		(postId: number | string) => {
			if (isMobile) {
				setSheetPostId(postId);
				return;
			}
			if (typeof document === "undefined") return;
			const page = surfaceRef.current?.querySelector<HTMLElement>(
				`.post-viewer__page-wrap[data-idx="${activeIndexRef.current}"] .post-viewer__page`,
			);
			if (!page) return;
			const aside = page.querySelector<HTMLElement>(".post-viewer__aside");
			const comments = page.querySelector<HTMLElement>(".post-comments");
			if (!aside || !comments) return;
			// Scroll ONLY the aside — avoid `scrollIntoView`, which would walk up
			// the scrolling-box chain and disturb the pager surface.
			const asideTop = aside.getBoundingClientRect().top;
			const commentsTop = comments.getBoundingClientRect().top;
			aside.scrollTo({
				top: aside.scrollTop + (commentsTop - asideTop),
				behavior: "smooth",
			});
			const textarea = comments.querySelector<HTMLTextAreaElement>("textarea");
			// `preventScroll` so the browser doesn't re-anchor scroll on focus.
			textarea?.focus({ preventScroll: true });
		},
		[isMobile],
	);

	// Open / reset to chosen index whenever caller changes initialIndex.
	// Suppress the transition for one frame so the (possibly long) jump
	// from the previous position lands instantly — feels like opening
	// fresh rather than scrolling across N posts.
	React.useEffect(() => {
		if (!open) return;
		setSuppressTransition(true);
		setActiveIndex(initialIndex ?? 0);
		setDragOffset(0);
		const id = requestAnimationFrame(() => setSuppressTransition(false));
		return () => cancelAnimationFrame(id);
	}, [open, initialIndex]);

	// ESC + arrow keys + body scroll lock
	React.useEffect(() => {
		if (!open) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") {
				// ESC dismisses the comments sheet first; only close the
				// whole viewer once no sheet is open.
				if (sheetPostIdRef.current !== null) {
					setSheetPostId(null);
					return;
				}
				requestClose();
				return;
			}
			// Trap Tab inside the dialog — the page behind is inert while the
			// viewer is open. The comments sheet and login modal portal to
			// document.body (outside this dialog), so when focus lives in one
			// of those stacked dialogs we leave Tab to them instead of
			// yanking focus back into this layer.
			if (e.key === "Tab") {
				const dialog = dialogRef.current;
				if (!dialog) return;
				const target = e.target instanceof HTMLElement ? e.target : null;
				if (
					target &&
					!dialog.contains(target) &&
					target.closest("[role='dialog']")
				)
					return;
				trapTabKey(e, dialog);
				return;
			}
			// Arrow / paging keys are pager controls — suppress while the
			// sheet is open so the user can scroll comments without the
			// pager flipping pages underneath.
			if (sheetPostIdRef.current !== null) return;
			// Desktop 點留言會 focus 留言 textarea 但不開 sheet，這時打 j/k 該是
			// 輸入字元，不該切貼文；同理 input / contentEditable 也跳過
			const target = e.target as HTMLElement | null;
			const tag = target?.tagName;
			const isEditable =
				tag === "INPUT" ||
				tag === "TEXTAREA" ||
				target?.isContentEditable === true;
			if (isEditable) return;
			if (e.key === "ArrowDown" || e.key === "PageDown" || e.key === "j") {
				e.preventDefault();
				goTo(activeIndexRef.current + 1);
			}
			if (e.key === "ArrowUp" || e.key === "PageUp" || e.key === "k") {
				e.preventDefault();
				goTo(activeIndexRef.current - 1);
			}
		};
		document.addEventListener("keydown", onKey);
		return () => {
			document.removeEventListener("keydown", onKey);
		};
	}, [open, requestClose, goTo]);

	// Position-preserving scroll lock. The previous lock only toggled
	// `body { overflow: hidden }`, which clamps the document scroll to the top
	// while locked and never restores it on unlock — so closing the viewer
	// dropped the feed list back to the top. (The market list dodges this by
	// being a real route: it unmounts and re-asserts `scrollY` on remount via
	// its snapshot. This viewer is an in-page overlay that never remounts, so
	// nothing runs to restore scroll.) Pin the body at `top: -scrollY` so the
	// page holds its place under the overlay, then put the exact offset back on
	// close. `html { scrollbar-gutter: stable }` already reserves the scrollbar
	// track, so dropping the scrollbar here causes no horizontal shift. Keyed on
	// `open` alone so a mid-open re-render can't re-capture scrollY as 0.
	React.useEffect(() => {
		if (!open) return;
		return acquireBodyScrollLock({ preserveScrollPosition: true });
	}, [open]);

	// Trigger load more when nearing the end
	React.useEffect(() => {
		if (!open || !onNeedMore || !hasMore) return;
		if (activeIndex >= posts.length - 2) {
			onNeedMore();
		}
	}, [open, onNeedMore, hasMore, activeIndex, posts.length]);

	// Avoid SSR mismatch — only portal after mount
	const [mounted, setMounted] = React.useState(false);
	React.useEffect(() => {
		setMounted(true);
	}, []);

	// Focus management: remember whatever was focused when the viewer
	// opened (usually the tapped grid card), move focus onto the close
	// button (or the dialog itself as fallback), and hand focus back on
	// close/unmount — but only if the opener is still in the DOM.
	// `mounted` is in the deps so a deep-link open (open before the portal
	// exists) still grabs focus once the dialog actually renders.
	React.useEffect(() => {
		if (!open || !mounted) return;
		lastFocusedRef.current =
			document.activeElement instanceof HTMLElement
				? document.activeElement
				: null;
		// `preventScroll` so grabbing focus doesn't disturb the open zoom.
		const initial = closeButtonRef.current ?? dialogRef.current;
		initial?.focus({ preventScroll: true });
		return () => {
			const previous = lastFocusedRef.current;
			lastFocusedRef.current = null;
			if (previous?.isConnected) previous.focus({ preventScroll: true });
		};
	}, [open, mounted]);

	// One-shot scroll hint shown on first open per browser session. It
	// stays visible until the user interacts, with a max lifetime so it
	// never hangs around if they pause on the screen.
	const [showHint, setShowHint] = React.useState(false);
	const dismissHint = React.useCallback(() => setShowHint(false), []);
	React.useEffect(() => {
		if (!open) {
			setShowHint(false);
			return;
		}
		if (typeof window === "undefined") return;
		try {
			if (sessionStorage.getItem("post-viewer-scroll-hint-seen")) {
				setShowHint(false);
				return;
			}
			sessionStorage.setItem("post-viewer-scroll-hint-seen", "1");
		} catch {
			// sessionStorage unavailable (privacy mode); still show the hint
			// once per page load.
		}
		setShowHint(true);
		const t = window.setTimeout(() => setShowHint(false), 6000);
		return () => window.clearTimeout(t);
	}, [open]);

	React.useEffect(() => {
		if (!open || !showHint) return;
		const surface = surfaceRef.current;
		const onDismiss = () => dismissHint();
		surface?.addEventListener("pointerdown", onDismiss, { passive: true });
		surface?.addEventListener("wheel", onDismiss, { passive: true });
		document.addEventListener("keydown", onDismiss);
		return () => {
			surface?.removeEventListener("pointerdown", onDismiss);
			surface?.removeEventListener("wheel", onDismiss);
			document.removeEventListener("keydown", onDismiss);
		};
	}, [dismissHint, open, showHint]);

	// === Gesture pager =================================================
	// Replaces native scroll-snap with a translateY transform so swipe
	// motion is 1:1 with the finger and the release animation uses an
	// Apple-style easing rather than the browser's stock snap.
	const gestureRef = React.useRef<{
		pointerId: number;
		// Reference Y for computing the pager translation. While the
		// gesture lives in aside-scroll mode this equals `initialStartY`;
		// the moment the pager engages we reset it to that point so
		// `dragOffset` starts cleanly at 0 (no visual jump).
		startY: number;
		initialStartY: number;
		startTime: number;
		lastY: number;
		lastTime: number;
		// Snapshot of velocity points so we can compute a recent (not
		// average) velocity at release — better matches user intent.
		samples: Array<{ y: number; t: number }>;
		captured: boolean;
		// Aside the gesture began inside (null when started on the
		// media/elsewhere). When non-null, the move handler drives the
		// aside's scroll directly until it hits a boundary, then flips
		// into pager mode for the remainder of the same gesture.
		aside: HTMLElement | null;
		pagerEngaged: boolean;
	} | null>(null);

	const pageHeight = () =>
		surfaceRef.current?.clientHeight || window.innerHeight || 1;

	const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
		// Only react to primary pointer — middle/right clicks bypass.
		if (e.button !== 0 && e.button !== undefined) return;
		if (isInteractiveTarget(e.target)) return;
		const startedAside = scrollAreaOf(e.target);
		// The mobile article reader owns the gesture for its whole fixed-height
		// pane — it never hands an end-of-content swipe to the pager. With no
		// hand-off to arrange there is nothing for JS to drive, so leave the
		// gesture to the browser: native touch scrolling runs on the compositor
		// and carries momentum, which a per-`pointermove` `scrollTop` write
		// cannot. `touch-action: pan-y` on that element re-opens the pan the
		// pager surface's `touch-action: none` would otherwise swallow, and its
		// `overscroll-behavior: contain` keeps the boundary from chaining out.
		if (startedAside?.matches("[data-post-viewer-scroll]")) {
			gestureRef.current = null;
			return;
		}
		gestureRef.current = {
			pointerId: e.pointerId,
			startY: e.clientY,
			initialStartY: e.clientY,
			startTime: performance.now(),
			lastY: e.clientY,
			lastTime: performance.now(),
			samples: [{ y: e.clientY, t: performance.now() }],
			captured: false,
			aside: startedAside,
			// When the gesture starts outside the aside, the pager is the
			// only mode — engage it up front. Otherwise the first move
			// scrolls the aside, then flips to the pager at the boundary.
			pagerEngaged: !startedAside,
		};
	};

	const onPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
		const g = gestureRef.current;
		if (!g) return;
		if (e.pointerId !== g.pointerId) return;
		const totalDy = e.clientY - g.initialStartY;
		const deltaSinceLast = e.clientY - g.lastY;
		const aside = g.aside;

		if (!g.captured) {
			if (Math.abs(totalDy) < 6) {
				g.lastY = e.clientY;
				g.lastTime = performance.now();
				return;
			}
			g.captured = true;
			setIsDragging(true);
			try {
				e.currentTarget.setPointerCapture(g.pointerId);
			} catch {
				// Older Safari may refuse capture for touch — proceed anyway.
			}
		}

		const now = performance.now();
		const recordSample = () => {
			g.lastY = e.clientY;
			g.lastTime = now;
			// Keep ~80ms of recent samples for velocity estimate.
			g.samples.push({ y: e.clientY, t: now });
			while (g.samples.length > 1 && now - g.samples[0].t > 80) {
				g.samples.shift();
			}
		};

		// While the gesture lives in the aside and still has scroll
		// headroom in the swipe direction, drive aside scroll from JS.
		// Once the aside hits its boundary the same gesture flips into
		// pager mode — so a single continuous swipe can finish reading
		// comments AND switch to the next post.
		if (aside && !g.pagerEngaged) {
			if (asideCanScroll(aside, deltaSinceLast)) {
				aside.scrollTop -= deltaSinceLast;
				recordSample();
				return;
			}
			// Aside boundary reached — engage pager from the current
			// finger position so dragOffset starts at 0 (no visual jump)
			// and the velocity sample window only contains pager motion.
			g.pagerEngaged = true;
			g.startY = e.clientY;
			g.samples = [{ y: e.clientY, t: now }];
		}

		// Pager mode: translate the track and rubber-band at the ends.
		const pagerDy = e.clientY - g.startY;
		const atFirst = activeIndexRef.current === 0;
		const atLast = activeIndexRef.current === posts.length - 1 && !hasMore;
		let offset = pagerDy;
		if ((atFirst && pagerDy > 0) || (atLast && pagerDy < 0)) {
			offset = pagerDy * RUBBER_BAND;
		}
		setDragOffset(offset);
		recordSample();
	};

	const onPointerEnd = (e: React.PointerEvent<HTMLDivElement>) => {
		const g = gestureRef.current;
		if (!g) return;
		if (e.pointerId !== g.pointerId) return;
		gestureRef.current = null;
		if (!g.captured) {
			setIsDragging(false);
			setDragOffset(0);
			return;
		}
		try {
			if (e.currentTarget.hasPointerCapture(g.pointerId)) {
				e.currentTarget.releasePointerCapture(g.pointerId);
			}
		} catch {
			// noop
		}
		setIsDragging(false);
		// Gesture lived entirely in aside-scroll mode — leave the aside
		// where the user dragged it and don't trigger a page switch.
		if (!g.pagerEngaged) {
			setDragOffset(0);
			return;
		}
		// Recent-window velocity in px/ms (negative = swiped up = next).
		const first = g.samples[0];
		const last = g.samples[g.samples.length - 1];
		const dt = Math.max(last.t - first.t, 1);
		const velocity = (last.y - first.y) / dt;
		const dy = e.clientY - g.startY;
		const distance = Math.abs(dy);
		const threshold = pageHeight() * SNAP_DISTANCE_RATIO;

		let target = activeIndexRef.current;
		const goingNext = dy < 0;
		const farEnough = distance > threshold;
		const fastEnough = Math.abs(velocity) > SNAP_VELOCITY;
		if (farEnough || fastEnough) {
			// Direction is dominated by velocity when fast, by displacement
			// when slow — so a quick flick wins even if finger didn't
			// travel far, while a slow long drag still commits.
			const dir = fastEnough ? (velocity < 0 ? 1 : -1) : goingNext ? 1 : -1;
			target = activeIndexRef.current + dir;
		}
		goTo(target);
	};

	// === Wheel handling ================================================
	// Native scroll-snap is gone, so we drive the pager from wheel
	// deltas ourselves. Aside passthrough mirrors the touch logic:
	// if the cursor is over the comments aside and the aside has room
	// to scroll in the wheel direction, the wheel goes to the aside.
	const wheelAccumRef = React.useRef(0);
	const wheelCooldownRef = React.useRef(0);
	const wheelResetTimerRef = React.useRef<number | null>(null);
	// Set when the previous wheel event was consumed by aside scrolling so
	// we can detect the transition into "aside boundary hit" on this event.
	const prevWheelScrolledAsideRef = React.useRef(false);
	// Timestamp until which boundary cooldown is active.
	const boundaryCooldownRef = React.useRef(0);

	React.useEffect(() => {
		if (!open) return;
		const surface = surfaceRef.current;
		if (!surface) return;
		const onWheel = (e: WheelEvent) => {
			const now = performance.now();
			const aside = scrollAreaOf(e.target);
			if (aside) {
				const dir = e.deltaY > 0 ? -1 : 1; // mirror touch convention
				if (asideCanScroll(aside, dir)) {
					// Aside is still scrolling — track for boundary detection.
					prevWheelScrolledAsideRef.current = true;
					return;
				}
				if (aside.matches("[data-post-viewer-scroll]")) return;
				// Aside hit its boundary. If the previous event was still
				// scrolling inside the aside, treat this as a fresh
				// boundary-overshoot and arm the cooldown so trackpad
				// momentum / accidental over-scroll doesn't flip pages.
				if (prevWheelScrolledAsideRef.current) {
					boundaryCooldownRef.current = now + BOUNDARY_COOLDOWN_MS;
				}
			}
			prevWheelScrolledAsideRef.current = false;
			e.preventDefault();
			if (now < wheelCooldownRef.current) return;
			// Boundary cooldown swallows small wheel events while the cursor
			// is STILL over the aside (the user is reading comments and
			// over-scrolled). Moving the cursor onto the media area is a
			// deliberate context switch and regains immediate pager control.
			// A large single delta (mouse-wheel click / hard flick) bypasses
			// the cooldown so intentional "skip-to-next" gestures still work.
			if (
				aside !== null &&
				now < boundaryCooldownRef.current &&
				Math.abs(e.deltaY) < WHEEL_ESCAPE_DELTA
			) {
				wheelAccumRef.current = 0;
				return;
			}
			wheelAccumRef.current += e.deltaY;
			if (Math.abs(wheelAccumRef.current) >= WHEEL_THRESHOLD) {
				const dir = wheelAccumRef.current > 0 ? 1 : -1;
				goTo(activeIndexRef.current + dir);
				wheelAccumRef.current = 0;
				wheelCooldownRef.current = now + SNAP_DURATION_MS;
				boundaryCooldownRef.current = 0;
			}
			// Decay accumulator if user pauses — avoids stale half-deltas
			// triggering a switch on the next unrelated wheel event.
			if (wheelResetTimerRef.current !== null) {
				window.clearTimeout(wheelResetTimerRef.current);
			}
			wheelResetTimerRef.current = window.setTimeout(() => {
				wheelAccumRef.current = 0;
			}, 180);
		};
		surface.addEventListener("wheel", onWheel, { passive: false });
		return () => {
			surface.removeEventListener("wheel", onWheel);
			if (wheelResetTimerRef.current !== null) {
				window.clearTimeout(wheelResetTimerRef.current);
				wheelResetTimerRef.current = null;
			}
		};
	}, [open, goTo]);

	if (!open || !mounted) return null;

	const goPrev = () => goTo(activeIndex - 1);
	const goNext = () => goTo(activeIndex + 1);

	// translateY combines the page offset (index * 100%) with the live
	// drag delta in px. Wrapping in calc() keeps the percentage part
	// resolution-independent while the drag delta tracks the pointer 1:1.
	const trackStyle: React.CSSProperties = {
		transform: `translate3d(0, calc(${-activeIndex * 100}% + ${dragOffset}px), 0)`,
		transition:
			isDragging || suppressTransition
				? "none"
				: `transform ${SNAP_DURATION_MS}ms ${SNAP_EASING}`,
	};

	// Whole-surface zoom style. `opening` paints card-sized with no transition;
	// the next frame ("open") releases to full size, animating the grow. "closing"
	// shrinks back toward the active card. Easing is a plain ease-out — no spring.
	const SCALE_TWEEN = `transform 300ms ${SCALE_EASING}, opacity 300ms ${SCALE_EASING}`;
	let viewerStyle: React.CSSProperties | undefined;
	if (scalePhase === "opening" && openRect) {
		viewerStyle = {
			...zoomShrinkStyle(openRect, ZOOM_SCALE_BOUNDS),
			transition: "none",
		};
	} else if (scalePhase === "closing" && closeRectRef.current) {
		viewerStyle = {
			...zoomShrinkStyle(closeRectRef.current, ZOOM_SCALE_BOUNDS),
			transition: SCALE_TWEEN,
		};
	} else if (!reduceMotion) {
		viewerStyle = {
			transformOrigin: openRect ? zoomOriginCenter(openRect) : undefined,
			transform: "scale(1)",
			opacity: 1,
			transition: SCALE_TWEEN,
		};
	}

	const node = (
		<div
			className="post-viewer"
			role="dialog"
			aria-modal="true"
			aria-label="貼文詳情"
			ref={dialogRef}
			// Focusable fallback for the trap when no tabbable element exists.
			tabIndex={-1}
			style={viewerStyle}
			onTransitionEnd={(e) => {
				if (scalePhase === "closing" && e.propertyName === "transform") {
					onClose();
				}
			}}
		>
			<button
				type="button"
				data-storefront-post-preview-control="true"
				className="post-viewer__close"
				ref={closeButtonRef}
				onClick={requestClose}
				aria-label="關閉"
			>
				<CloseIcon />
			</button>

			<div className="post-viewer__counter" aria-live="polite">
				{activeIndex + 1} / {posts.length}
				{hasMore && <span className="post-viewer__counter-more">+</span>}
			</div>

			<div
				className="post-viewer__surface"
				ref={surfaceRef}
				data-dragging={isDragging || undefined}
				onPointerDown={onPointerDown}
				onPointerMove={onPointerMove}
				onPointerUp={onPointerEnd}
				onPointerCancel={onPointerEnd}
			>
				<div className="post-viewer__track" ref={trackRef} style={trackStyle}>
					{posts.map((post, idx) => (
						<section
							key={post.id}
							data-idx={idx}
							className="post-viewer__page-wrap"
							aria-hidden={idx !== activeIndex}
						>
							{/* Window the track to activeIndex ± 1: the empty section
							    keeps its slot (each wrap is one viewport tall), so the
							    percentage transform stays valid, but far-away posts
							    don't mount — opening the viewer after 4 pages of
							    infinite scroll used to fire ~48 comment fetches + 48
							    full-size image loads at once. */}
							{Math.abs(idx - activeIndex) <= 1 ? (
								<PostPage
									post={post}
									isActive={idx === activeIndex}
									commentsActive={idx === activeIndex && sheetPostId === null}
									showInlineComments={!isMobile}
									onRequestComments={handleRequestComments}
									onPrev={goPrev}
									onNext={goNext}
									canPrev={idx > 0}
									canNext={idx < posts.length - 1 || hasMore}
								/>
							) : null}
						</section>
					))}
				</div>
				{hasMore && activeIndex >= posts.length - 1 && (
					<div className="post-viewer__loading">載入更多…</div>
				)}
			</div>

			{showHint && <PostViewerScrollHint />}
			<CommentsBottomSheet
				postId={sheetPostId}
				open={sheetPostId !== null}
				onClose={() => setSheetPostId(null)}
			/>
		</div>
	);

	return createPortal(node, document.body);
}

/**
 * One-shot scroll/drag teaching cue shown on first open per session.
 * Parent unmounts it after ~1.5s.
 */
function PostViewerScrollHint() {
	return (
		<div className="post-viewer__scroll-hint" aria-hidden="true">
			<div className="post-viewer__scroll-hint-glyph">
				<AppIcon
					name="arrow-down"
					size={96}
					className="animate-pulse motion-reduce:animate-none"
				/>
			</div>
			<span className="post-viewer__scroll-hint-label">上下滑動切換貼文</span>
		</div>
	);
}
