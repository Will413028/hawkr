"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import AppIcon from "@/components/shared/AppIcon";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import CommentsBottomSheet from "@/features/feed/components/CommentsBottomSheet";
import PostComments from "@/features/feed/components/PostComments";
import PostViewTracker from "@/features/feed/components/PostViewTracker";
import { zoomOriginCenter, zoomShrinkStyle } from "@/lib/animation/zoom";
import type { PostDetail, PostListItem } from "@/lib/api/openapi-types";
import { useAuth } from "@/lib/auth/context";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import { usePostInteractions } from "@/lib/hooks/useSocial";
import { buildImageUrl, isGifMedia, ossImageUrl } from "@/lib/media/cdn";
import { FALLBACK_POST_IMAGE, sharePost } from "@/lib/posts/repo";
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
	/** 唯讀呈現：只顯示數字，不渲染按讚、留言、分享與留言區（賣家的店面預覽）。 */
	readOnly?: boolean;
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
	readOnly = false,
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
	readOnly?: boolean;
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
	const intentRef = React.useRef<"like" | "comment">("like");
	const openLoginRequired = useAppUiStore((state) => state.openLoginRequired);
	const { status } = useAuth();
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

	const [captionExpanded, setCaptionExpanded] = React.useState(false);
	const [captionOverflows, setCaptionOverflows] = React.useState(false);
	const articleScrollRef = React.useRef<HTMLDivElement>(null);

	React.useEffect(() => {
		if (isActive) return;
		setCaptionExpanded(false);
	}, [isActive]);

	React.useEffect(() => {
		if (captionExpanded) return;
		const host = articleScrollRef.current;
		const caption = host?.querySelector<HTMLElement>(".post-viewer__content");
		if (!content || !caption) {
			setCaptionOverflows(false);
			return;
		}
		const measure = () => {
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

	const captionToggleable =
		captionOverflows || tags.length > 0 || products.length > 0;

	const pageRef = React.useRef<HTMLElement>(null);
	const mediaImgRef = React.useRef<HTMLImageElement>(null);

	React.useEffect(() => {
		const img = mediaImgRef.current;
		const page = pageRef.current;
		if (!img || !page) return;

		const apply = () => {
			const { naturalWidth: nw, naturalHeight: nh } = img;
			const boxW = img.clientWidth;
			const boxH = img.clientHeight;
			if (!nw || !nh || !boxW || !boxH) return;
			const rawY =
				getComputedStyle(img).objectPosition.split(/\s+/)[1] ?? "50%";
			const parsedY = rawY.endsWith("%") ? Number.parseFloat(rawY) / 100 : 0.5;
			const yFraction = Number.isFinite(parsedY) ? parsedY : 0.5;

			const scale = Math.min(boxW / nw, boxH / nh);
			const paintedHeight = nh * scale;
			const budget = boxH - (boxH - paintedHeight) * yFraction;
			page.style.setProperty(
				"--post-drawer-max",
				`${Math.max(0, Math.round(budget))}px`,
			);
		};

		apply();
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
		if (status === "signedIn") {
			void sharePost(post.id).catch(() => undefined);
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
			}
		}
		if (typeof navigator !== "undefined" && navigator.clipboard && shareUrl) {
			try {
				await navigator.clipboard.writeText(shareUrl);
				setNotice("連結已複製，可貼到任何聊天室分享");
				return;
			} catch {
			}
		}
		setNotice("分享連結尚未支援，請手動複製網址列");
	};

	const hasMedia = images.length > 0 && Boolean(images[activeImage]);
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
			{isActive ? <PostViewTracker postId={String(post.id)} /> : null}
			{hasMedia && (
				<div className="post-viewer__media">

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
						loading={isActive ? "eager" : "lazy"}
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

					{sellerPath(post.storeCode) ? (
						<Link
							href={sellerPath(post.storeCode)}
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

						<span>{captionExpanded ? "收合說明" : "查看更多"}</span>
						<AppIcon
							name={captionExpanded ? "chevron-down" : "chevron-up"}
							size={16}
						/>
					</button>
				) : null}

				<footer className="post-viewer__actions">

					{readOnly ? (
						<>
							<div className={`post-viewer__action${liked ? " is-liked" : ""}`}>
								<AppIcon name="heart" fill={liked} size={26} />
								<span>{formatCount(likeCount)}</span>
							</div>
							<div className="post-viewer__action">
								<AppIcon name="chat" size={26} />
								<span>{formatCount(post.commentCount)}</span>
							</div>
						</>
					) : (
						<>
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
						</>
					)}
				</footer>
				{notice ? (
					<p className="post-viewer__notice" role="status">
						{notice}
					</p>
				) : null}
				{showInlineComments && !readOnly ? (
					<div className="post-viewer__inline-comments">
						<PostComments
							postId={post.id}
							viewer={post.viewer}
							active={commentsActive}
						/>
					</div>
				) : null}
			</aside>
			<AuthGateWaitModal open={authWaiting} />
		</article>
	);
}

const SNAP_EASING = "cubic-bezier(0.32, 0.72, 0, 1)";
const SNAP_DURATION_MS = 520;

const SCALE_EASING = "cubic-bezier(0.22, 1, 0.36, 1)";

const useIsomorphicLayoutEffect =
	typeof window !== "undefined" ? React.useLayoutEffect : React.useEffect;

const ZOOM_SCALE_BOUNDS = { min: 0.08, max: 0.72 };
const SNAP_DISTANCE_RATIO = 0.18;
const SNAP_VELOCITY = 0.45;
const RUBBER_BAND = 0.32;
const WHEEL_THRESHOLD = 60;
const BOUNDARY_COOLDOWN_MS = 500;
const WHEEL_ESCAPE_DELTA = 80;

const isInteractiveTarget = (target: EventTarget | null): boolean => {
	if (!(target instanceof HTMLElement)) return false;
	return Boolean(target.closest("button, a, input, textarea, [role='button']"));
};
const FOCUSABLE_SELECTOR =
	"a[href], button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex='-1'])";
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
	readOnly = false,
}: PostDetailViewerProps) {
	const open = initialIndex !== null;
	const surfaceRef = React.useRef<HTMLDivElement>(null);
	const trackRef = React.useRef<HTMLDivElement>(null);
	const dialogRef = React.useRef<HTMLDivElement>(null);
	const closeButtonRef = React.useRef<HTMLButtonElement>(null);
	const lastFocusedRef = React.useRef<HTMLElement | null>(null);
	const [activeIndex, setActiveIndex] = React.useState<number>(
		initialIndex ?? 0,
	);
	const [dragOffset, setDragOffset] = React.useState(0);
	const [isDragging, setIsDragging] = React.useState(false);
	const [suppressTransition, setSuppressTransition] = React.useState(false);
	const activeIndexRef = React.useRef(activeIndex);
	activeIndexRef.current = activeIndex;

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
	const [sheetPostId, setSheetPostId] = React.useState<string | null>(null);
	const sheetPostIdRef = React.useRef(sheetPostId);
	sheetPostIdRef.current = sheetPostId;

	const goTo = React.useCallback(
		(next: number) => {
			const clamped = Math.max(0, Math.min(posts.length - 1, next));
			setActiveIndex(clamped);
			setDragOffset(0);
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
				setSheetPostId(String(postId));
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
			const asideTop = aside.getBoundingClientRect().top;
			const commentsTop = comments.getBoundingClientRect().top;
			aside.scrollTo({
				top: aside.scrollTop + (commentsTop - asideTop),
				behavior: "smooth",
			});
			const textarea = comments.querySelector<HTMLTextAreaElement>("textarea");
			textarea?.focus({ preventScroll: true });
		},
		[isMobile],
	);

	React.useEffect(() => {
		if (!open) return;
		setSuppressTransition(true);
		setActiveIndex(initialIndex ?? 0);
		setDragOffset(0);
		const id = requestAnimationFrame(() => setSuppressTransition(false));
		return () => cancelAnimationFrame(id);
	}, [open, initialIndex]);

	React.useEffect(() => {
		if (!open) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") {
				if (sheetPostIdRef.current !== null) {
					setSheetPostId(null);
					return;
				}
				requestClose();
				return;
			}
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
			if (sheetPostIdRef.current !== null) return;
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

	React.useEffect(() => {
		if (!open) return;
		return acquireBodyScrollLock({ preserveScrollPosition: true });
	}, [open]);

	React.useEffect(() => {
		if (!open || !onNeedMore || !hasMore) return;
		if (activeIndex >= posts.length - 2) {
			onNeedMore();
		}
	}, [open, onNeedMore, hasMore, activeIndex, posts.length]);

	const [mounted, setMounted] = React.useState(false);
	React.useEffect(() => {
		setMounted(true);
	}, []);

	React.useEffect(() => {
		if (!open || !mounted) return;
		lastFocusedRef.current =
			document.activeElement instanceof HTMLElement
				? document.activeElement
				: null;
		const initial = closeButtonRef.current ?? dialogRef.current;
		initial?.focus({ preventScroll: true });
		return () => {
			const previous = lastFocusedRef.current;
			lastFocusedRef.current = null;
			if (previous?.isConnected) previous.focus({ preventScroll: true });
		};
	}, [open, mounted]);

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

	const gestureRef = React.useRef<{
		pointerId: number;
		startY: number;
		initialStartY: number;
		startTime: number;
		lastY: number;
		lastTime: number;
		samples: Array<{ y: number; t: number }>;
		captured: boolean;
		aside: HTMLElement | null;
		pagerEngaged: boolean;
	} | null>(null);

	const pageHeight = () =>
		surfaceRef.current?.clientHeight || window.innerHeight || 1;

	const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
		if (e.button !== 0 && e.button !== undefined) return;
		if (isInteractiveTarget(e.target)) return;
		const startedAside = scrollAreaOf(e.target);
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
			}
		}

		const now = performance.now();
		const recordSample = () => {
			g.lastY = e.clientY;
			g.lastTime = now;
			g.samples.push({ y: e.clientY, t: now });
			while (g.samples.length > 1 && now - g.samples[0].t > 80) {
				g.samples.shift();
			}
		};

		if (aside && !g.pagerEngaged) {
			if (asideCanScroll(aside, deltaSinceLast)) {
				aside.scrollTop -= deltaSinceLast;
				recordSample();
				return;
			}
			g.pagerEngaged = true;
			g.startY = e.clientY;
			g.samples = [{ y: e.clientY, t: now }];
		}

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
		}
		setIsDragging(false);
		if (!g.pagerEngaged) {
			setDragOffset(0);
			return;
		}
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
			const dir = fastEnough ? (velocity < 0 ? 1 : -1) : goingNext ? 1 : -1;
			target = activeIndexRef.current + dir;
		}
		goTo(target);
	};

	const wheelAccumRef = React.useRef(0);
	const wheelCooldownRef = React.useRef(0);
	const wheelResetTimerRef = React.useRef<number | null>(null);
	const prevWheelScrolledAsideRef = React.useRef(false);
	const boundaryCooldownRef = React.useRef(0);

	React.useEffect(() => {
		if (!open) return;
		const surface = surfaceRef.current;
		if (!surface) return;
		const onWheel = (e: WheelEvent) => {
			const now = performance.now();
			const aside = scrollAreaOf(e.target);
			if (aside) {
				const dir = e.deltaY > 0 ? -1 : 1;
				if (asideCanScroll(aside, dir)) {
					prevWheelScrolledAsideRef.current = true;
					return;
				}
				if (aside.matches("[data-post-viewer-scroll]")) return;
				if (prevWheelScrolledAsideRef.current) {
					boundaryCooldownRef.current = now + BOUNDARY_COOLDOWN_MS;
				}
			}
			prevWheelScrolledAsideRef.current = false;
			e.preventDefault();
			if (now < wheelCooldownRef.current) return;
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

	const trackStyle: React.CSSProperties = {
		transform: `translate3d(0, calc(${-activeIndex * 100}% + ${dragOffset}px), 0)`,
		transition:
			isDragging || suppressTransition
				? "none"
				: `transform ${SNAP_DURATION_MS}ms ${SNAP_EASING}`,
	};

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

							{Math.abs(idx - activeIndex) <= 1 ? (
								<PostPage
									post={post}
									isActive={idx === activeIndex}
									commentsActive={idx === activeIndex && sheetPostId === null}
									showInlineComments={!isMobile}
									readOnly={readOnly}
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
			{readOnly ? null : (
				<CommentsBottomSheet
					postId={sheetPostId}
					open={sheetPostId !== null}
					onClose={() => setSheetPostId(null)}
				/>
			)}
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
