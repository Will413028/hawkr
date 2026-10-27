"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import HashtagText from "@/features/feed/components/HashtagText";
import type { PostListItem } from "@/lib/api/openapi-types";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import { usePostInteractions } from "@/lib/hooks/useSocial";
import { postCardPresetForMedia } from "@/lib/media/cdn";
import { sellerPath } from "@/lib/seo/urls";

/**
 * Same glyphs the post detail viewer uses, at the card's compact scale — the
 * hand-rolled 15px SVGs that used to live here drew a different heart and a
 * different speech bubble from the ones the viewer opens into.
 */
const ACTION_ICON_SIZE = 20;

interface Props {
	post: PostListItem;
	imageSrc: string;
	imagePriority?: boolean;
	imageLoading?: "eager" | "lazy";
	/** Guest tapped like — let the stage show the shared login modal. */
	onRequireLogin: () => void;
	onPrewarmPostDetail: () => void;
	/** Opens the already-rendered home post immediately, without first waiting
	 * for the `/feed?post=` route's server payload. */
	onOpenPost: (
		post: PostListItem,
		originRect: DOMRect | undefined,
		opener: HTMLElement | null,
	) => void;
}

const POST_DETAIL_HOVER_PREWARM_MS = 140;

export default function HomeStageFeedCard({
	post,
	imageSrc,
	imagePriority = false,
	imageLoading = "lazy",
	onRequireLogin,
	onPrewarmPostDetail,
	onOpenPost,
}: Props) {
	const cardRef = React.useRef<HTMLDivElement>(null);
	const prewarmTimerRef = React.useRef<number | null>(null);
	const {
		liked,
		likeCount,
		likeBusy,
		toggleLike: togglePostLike,
	} = usePostInteractions(post.id, {
		liked: post.isLiked,
		likeCount: post.likeCount,
	});
	// Liking writes to the buyer's account; gate guests behind login rather
	// than firing an optimistic like that fails server-side. The gate also
	// absorbs clicks landing before the sign-in state has resolved.
	const { authGate, authWaiting } = useAuthGate(onRequireLogin);
	// Deliberately one argument — see the note at the render site below.
	const sellerHref = sellerPath(post.storeCode);
	const sellerIdentity = (
		<>
			<Avatar
				src={post.logoKey}
				name={post.sellerName}
				alt={post.sellerName}
				entity="seller"
				loading={imagePriority ? "eager" : imageLoading}
				fetchPriority="low"
				className="size-11 rounded-full object-cover"
				fallbackClassName="grid place-items-center bg-hk-primary-subtle text-base font-black text-hk-accent-on-tint"
				fallback="empty"
			/>
			<div>
				<div className="font-sans font-black text-hk-heading transition-colors group-hover:text-hk-primary">
					{post.sellerName}
				</div>
				<div className="text-xs font-bold text-hk-muted">
					最新動態 · Hawkr 市集
				</div>
			</div>
		</>
	);
	const cancelPostDetailPrewarm = React.useCallback(() => {
		if (prewarmTimerRef.current === null) return;
		window.clearTimeout(prewarmTimerRef.current);
		prewarmTimerRef.current = null;
	}, []);
	const prewarmPostDetailNow = React.useCallback(() => {
		cancelPostDetailPrewarm();
		onPrewarmPostDetail();
	}, [cancelPostDetailPrewarm, onPrewarmPostDetail]);
	const schedulePostDetailPrewarm = React.useCallback(() => {
		cancelPostDetailPrewarm();
		prewarmTimerRef.current = window.setTimeout(() => {
			prewarmTimerRef.current = null;
			onPrewarmPostDetail();
		}, POST_DETAIL_HOVER_PREWARM_MS);
	}, [cancelPostDetailPrewarm, onPrewarmPostDetail]);
	React.useEffect(
		() => () => cancelPostDetailPrewarm(),
		[cancelPostDetailPrewarm],
	);
	const openPost = () => {
		prewarmPostDetailNow();
		const opener = cardRef.current;
		onOpenPost(post, opener?.getBoundingClientRect(), opener);
	};

	const toggleLike = (e: React.MouseEvent) => {
		e.stopPropagation();
		authGate(() => {
			void togglePostLike();
		});
	};

	return (
		<>
			{/* Portal events bubble through the React tree, so the waiting popup
			    must be a SIBLING of the card — inside it, a tap on the popup
			    would re-fire the card's openPost. */}
			<AuthGateWaitModal open={authWaiting} />
			{/* When this stage shares the row with editor picks, an auto media
			    track lets the 4:3 ratio consume about 60% of the compact card.
			    Bound it by both card width and the existing image-size ceiling so
			    the story metadata keeps the larger half without cropping media. */}
			{/* biome-ignore lint/a11y/useSemanticElements: card nests a seller link + like button, so a native <button> would create invalid interactive nesting. */}
			<div
				ref={cardRef}
				role="button"
				tabIndex={0}
				onPointerEnter={(event) => {
					const target = event.target;
					if (target instanceof Element && target.closest("a, button")) return;
					schedulePostDetailPrewarm();
				}}
				onPointerLeave={cancelPostDetailPrewarm}
				onPointerDown={(event) => {
					const target = event.target;
					if (target instanceof Element && target.closest("a, button")) return;
					prewarmPostDetailNow();
				}}
				onFocus={(event) => {
					if (event.target === event.currentTarget) prewarmPostDetailNow();
				}}
				onClick={openPost}
				onKeyDown={(e) => {
					if (e.target !== e.currentTarget) return;
					if (e.key === "Enter" || e.key === " ") {
						e.preventDefault();
						openPost();
					}
				}}
				className="group relative grid h-full cursor-pointer grid-cols-[minmax(0,min(48%,380px))_minmax(0,1fr)] overflow-hidden rounded-[22px] border border-hk-border-soft bg-hk-surface-card text-inherit no-underline max-[720px]:grid-cols-1"
			>
				{/* The post's own photo, magnified and faded into the surface colour,
				    bleeding out of the card and across BOTH columns. Its box is pinned
				    to the media column rather than the card, so the halo stays
				    concentric with the sharp copy inside it and reads as one continuous
				    picture instead of a second image laid over the first. Deliberately
				    unblurred: this is a magnification, not a frosted backdrop. */}
				{imageSrc ? (
					/* At rest this is an ordinary card; the whole backdrop only fades up
					   on hover. Keyboard users get it on focus too, and it is the pair of
					   layers that fades as one — animating them separately would let the
					   wash arrive before the photo it is there to soften. */
					<div className="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100 group-focus-visible:opacity-100 motion-reduce:transition-none">
						<div className="absolute inset-0">
							<div className="absolute inset-y-0 left-0 w-[min(48%,380px)] max-[720px]:w-full">
								<AppImage
									src={imageSrc}
									alt=""
									aria-hidden="true"
									fill
									preset={postCardPresetForMedia(imageSrc, "detail")}
									sizes="(max-width: 720px) 100vw, 1100px"
									loading={imagePriority ? "eager" : imageLoading}
									fetchPriority="low"
									/* object-cover, not contain: contain sizes the halo to the
									   photo's own aspect, so a tall image shrank to a narrow
									   column and left the far side of the card bare. Cover always
									   fills its box, so the halo reaches the card edge whatever
									   shape the seller uploaded. */
									className="scale-[3.4] object-cover object-center opacity-[0.38] max-[720px]:scale-[1.9]"
								/>
							</div>
						</div>
						{/* One uninterrupted wash over the whole card, never a panel
						    behind the text column: a column-bounded scrim draws a seam
						    down the middle and splits the card into blocks. Kept light so
						    the photo still reads through the copy, and ramped rather than
						    flat so it thickens only as far as the weakest text (the
						    byline and the counts). Stacked layout fades downwards. */}
						<div className="absolute inset-0 bg-gradient-to-r from-transparent via-hk-surface-card/3 to-hk-surface-card/12 max-[720px]:bg-gradient-to-b" />
					</div>
				) : null}
				<div className="relative min-w-0 w-full aspect-[4/3] h-full overflow-hidden max-[1080px]:min-h-[220px] max-[720px]:h-auto max-[720px]:min-h-0">
					{imageSrc ? (
						/* Concentric with the card: an inner corner stays parallel to the
						   outer one only when its radius is the outer radius minus the
						   gap, so the card's 22px over a 12px inset fixes the photo at
						   10px. A wider inset would drive that negative — which is why the
						   photo used to meet the card's curve with a square corner. */
						<div className="absolute inset-3 flex items-center justify-center">
							<AppImage
								src={imageSrc}
								alt={post.sellerName}
								/* Sized by the picture, not by a fixed box. `fill` +
								   object-contain letterboxes inside its box whenever the photo
								   and the box disagree on aspect, and a radius on that box
								   then curves the empty margin while the photo itself keeps
								   square corners. Letting the element take its intrinsic size
								   under max-w/max-h makes the element BE the photo, so the
								   radius and the shadow trace the real edges at any aspect.
								   New GIFs already carry their seller-authored transparent 4:3
								   frame; intrinsic sizing keeps legacy GIFs uncropped too. */
								width={1600}
								height={1200}
								preset={postCardPresetForMedia(imageSrc, "detail")}
								sizes="(max-width: 720px) 100vw, (max-width: 1080px) 48vw, (max-width: 1488px) 26vw, 380px"
								loading={imagePriority ? "eager" : imageLoading}
								// 第一張 LCP 候選維持 high；其餘圖片接近視口時原生 lazy 載入，
								// 切為目前卡片時改用 eager / low。
								fetchPriority={imagePriority ? "high" : "low"}
								className="h-auto max-h-full w-auto max-w-full rounded-[10px] object-contain drop-shadow-[0_6px_16px_rgba(0,0,0,0.22)]"
							/>
						</div>
					) : (
						<div className="grid size-full place-items-center bg-[linear-gradient(135deg,var(--color-ink-100),var(--color-ink-200))] text-7xl font-black text-white">
							{post.sellerName.trim().slice(0, 1) || "日"}
						</div>
					)}
				</div>
				<div className="relative flex min-w-0 flex-col px-6 py-5">
					{/*
					 * NO fallback to sellerId here, unlike most sellerPath() callers.
					 * On a post, an empty `storeCode` does not mean "this seller has
					 * no code yet" — the gateway sets it only when GetSellerBatch found
					 * the seller, so empty means NOT FOUND, and `/sellers/{store_id}`
					 * then 404s instead of 308ing. post.go's field comment says it
					 * outright: "此時前端必須不渲染賣家連結，而不是退回 sellerId".
					 *
					 * The identity still renders — name and avatar come from
					 * social-service's denormalised snapshot and are fine. Only the
					 * link is withheld.
					 */}
					{sellerHref ? (
						<Link
							href={sellerHref}
							onClick={(e) => e.stopPropagation()}
							className="group flex w-fit items-center gap-3 no-underline"
						>
							{sellerIdentity}
						</Link>
					) : (
						<div className="flex w-fit items-center gap-3">
							{sellerIdentity}
						</div>
					)}
					<HashtagText
						as="p"
						content={post.content}
						/* No fixed height: the carousel track already pins every card to
						   the same h-full box, so reserving 104px only pushed a short
						   post's copy away from its own byline and left a dead gap. */
						className="mt-4 line-clamp-4 font-sans text-base font-bold leading-relaxed text-hk-heading"
					/>
					{/* Same treatment as the post detail viewer's action bar
					    (`.post-viewer__action` in styles/features/feed-components.css):
					    `--hkui-foreground-muted` idle colour, and `--color-like-heart`
					    once liked. The glyphs take `currentColor`, so the counts and
					    icons share these tokens instead of the home page's `hk-*` browns. */}
					<div className="mt-auto flex flex-wrap items-center gap-4 border-t border-hk-border-soft pt-4 text-xs font-bold text-[var(--hkui-foreground-muted)]">
						<Button
							type="button"
							variant="light"
							color="default"
							size="sm"
							isIconOnly
							onClick={toggleLike}
							disabled={authWaiting}
							aria-pressed={liked}
							aria-busy={likeBusy || authWaiting || undefined}
							aria-label={liked ? "取消按讚" : "按讚"}
							/* `variant="light" color="default"` hard-paints
							   `text-[var(--hkui-foreground)]` — a near-black that made the
							   heart read as a different colour family from the comment icon
							   beside it. Restating the colour lets tailwind-merge drop it. */
							className={`h-auto w-auto inline-flex items-center gap-1.5 transition-colors ${
								liked
									? "text-hk-like"
									: "text-[var(--hkui-foreground-muted)] hover:text-[var(--hkui-foreground)]"
							}`}
						>
							<AppIcon name="heart" fill={liked} size={ACTION_ICON_SIZE} />
							{likeCount}
						</Button>
						<span className="inline-flex items-center gap-1.5">
							<AppIcon name="chat" size={ACTION_ICON_SIZE} />
							{post.commentCount}
						</span>
						<span className="ml-auto text-hk-primary" data-home-feed-open>
							看貼文 →
						</span>
					</div>
				</div>
			</div>
		</>
	);
}
