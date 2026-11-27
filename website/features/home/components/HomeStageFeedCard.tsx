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
	const { authGate, authWaiting } = useAuthGate(onRequireLogin);
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

			<AuthGateWaitModal open={authWaiting} />

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

				{imageSrc ? (
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
									className="scale-[3.4] object-cover object-center opacity-[0.38] max-[720px]:scale-[1.9]"
								/>
							</div>
						</div>

						<div className="absolute inset-0 bg-gradient-to-r from-transparent via-hk-surface-card/3 to-hk-surface-card/12 max-[720px]:bg-gradient-to-b" />
					</div>
				) : null}
				<div className="relative min-w-0 w-full aspect-[4/3] h-full overflow-hidden max-[1080px]:min-h-[220px] max-[720px]:h-auto max-[720px]:min-h-0">
					{imageSrc ? (
						<div className="absolute inset-3 flex items-center justify-center">
							<AppImage
								src={imageSrc}
								alt={post.sellerName}
								width={1600}
								height={1200}
								preset={postCardPresetForMedia(imageSrc, "detail")}
								sizes="(max-width: 720px) 100vw, (max-width: 1080px) 48vw, (max-width: 1488px) 26vw, 380px"
								loading={imagePriority ? "eager" : imageLoading}
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
						className="mt-4 line-clamp-4 font-sans text-base font-bold leading-relaxed text-hk-heading"
					/>

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
