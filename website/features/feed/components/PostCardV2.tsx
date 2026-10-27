"use client";

import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import Avatar from "@/components/shared/Avatar";
import Button from "@/components/ui/Button";
import type { PostListItem } from "@/lib/api/openapi-types";
import { FALLBACK_POST_IMAGE } from "@/lib/api/posts";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import { usePostInteractions } from "@/lib/hooks/useSocial";
import {
	buildImageUrl,
	postCardPresetForMedia,
	resolveCdnUrl,
} from "@/lib/media/cdn";
import { normalizePostTag } from "@/lib/post-tags/repo";
import { useLoginRequiredOverlay } from "@/lib/state/use-login-required-overlay";
import HashtagText from "./HashtagText";

const HASHTAG_PATTERN = /#[\p{L}\p{N}_]+/u;

function firstHashtag(content: string | null | undefined): string | null {
	if (!content) return null;
	const match = content.match(HASHTAG_PATTERN);
	if (!match) return null;
	return normalizePostTag(match[0]);
}

interface PostCardProps {
	post: PostListItem;
	/**
	 * Page-level lists reserve eager loading for a small visible row;
	 * lower and appended cards keep native lazy loading.
	 */
	imageLoading?: "eager" | "lazy";
	/** Reserve preload/high priority for the single card expected to be LCP. */
	imagePriority?: boolean;
	/** `mediaRect` is the on-screen box of the card's thumbnail at click time,
	 *  used to seed the open zoom animation (FeedHeroZoom). */
	onOpen?: (post: PostListItem, mediaRect?: DOMRect) => void;
	onLike?: (post: PostListItem, next: boolean) => void;
}

const HeartIcon = ({ filled }: { filled?: boolean }) => (
	<svg
		width="14"
		height="14"
		viewBox="0 0 24 24"
		fill={filled ? "currentColor" : "none"}
		stroke="currentColor"
		strokeWidth="2.2"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
	</svg>
);

function formatCount(n: number): string {
	if (n >= 10000) return `${(n / 10000).toFixed(1)}萬`;
	if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
	return String(n);
}

const PostCardV2 = React.memo(function PostCardV2({
	post,
	imageLoading = "lazy",
	imagePriority = false,
	onOpen,
	onLike,
}: PostCardProps) {
	const mediaRef = React.useRef<HTMLDivElement>(null);
	const {
		liked,
		likeCount,
		likeBusy,
		toggleLike: togglePostLike,
	} = usePostInteractions(post.id, {
		liked: post.isLiked,
		likeCount: post.likeCount,
	});
	const loginPayload = React.useMemo(
		() => ({
			title: "登入後即可按讚",
			description:
				"登入 Hawkr 後，就能幫喜歡的內容按讚，並追蹤商家的最新動態。",
			actionLabel: "登入 / 註冊",
		}),
		[],
	);
	const openLoginRequired = useLoginRequiredOverlay(loginPayload);
	const { authGate, authWaiting } = useAuthGate(openLoginRequired);
	// Stale wire data is common here: the DB keeps post image URLs whose OSS
	// files have since been purged, so the CDN 404s. Swap to the placeholder
	// via STATE (a new src through next/image), not by mutating img.src —
	// the optimizer's srcset outranks src, so a direct mutation leaves the
	// browser stuck on the dead srcset entries and the card stays broken.
	const [imageFailed, setImageFailed] = React.useState(false);
	const fallbackImage = buildImageUrl(FALLBACK_POST_IMAGE, "card");
	const imageSrc =
		!imageFailed && post.imageKeys[0]
			? resolveCdnUrl(post.imageKeys[0])
			: fallbackImage;

	const toggleLike = (e: React.MouseEvent) => {
		e.stopPropagation();
		// Liking writes to the buyer's account, so gate it behind login —
		// guests get the login prompt instead of an optimistic like that
		// would silently fail server-side.
		authGate(() => {
			const next = !liked;
			onLike?.(post, next);
			void togglePostLike();
		});
	};

	const open = () => onOpen?.(post, mediaRef.current?.getBoundingClientRect());

	return (
		<>
			{/* biome-ignore lint/a11y/useSemanticElements: feed cards include nested links and a like button, so a native button would create invalid interactive nesting. */}
			<div
				className="hkui-feed-card"
				data-storefront-post-preview-trigger="true"
				onClick={open}
				role="button"
				aria-haspopup="dialog"
				tabIndex={0}
				onKeyDown={(e) => {
					if (e.target !== e.currentTarget) return;
					if (e.key === "Enter" || e.key === " ") {
						e.preventDefault();
						open();
					}
				}}
			>
				<div
					className="hkui-feed-card__media"
					ref={mediaRef}
					data-feed-card-id={post.id}
				>
					<AppImage
						src={imageSrc}
						alt={post.content?.slice(0, 30) || post.sellerName}
						fill
						preset={postCardPresetForMedia(imageSrc)}
						priority={imagePriority}
						loading={imagePriority ? undefined : imageLoading}
						fetchPriority={imagePriority ? undefined : "low"}
						sizes="(max-width: 768px) 50vw, 320px"
						// Once we're already showing the placeholder, let AppImage's
						// built-in "圖片無法顯示" fallback take over on a second failure.
						onError={
							imageSrc === fallbackImage
								? undefined
								: () => setImageFailed(true)
						}
					/>
					{(() => {
						const tag = firstHashtag(post.content);
						if (tag) {
							return <span className="hkui-feed-card__badge">#{tag}</span>;
						}
						if (post.product) {
							return (
								<span className="hkui-feed-card__badge hkui-feed-card__badge--product">
									<span className="hkui-feed-card__badge-dot" />
									{post.product.name}
								</span>
							);
						}
						return <span className="hkui-feed-card__badge">動態</span>;
					})()}
					<Button
						type="button"
						variant="light"
						color="default"
						size="sm"
						isIconOnly
						className={`hkui-feed-card__like h-auto w-auto${liked ? " is-liked" : ""}`}
						onClick={toggleLike}
						/* Optimistic-only, same as the detail viewer — see the note there. */
						aria-pressed={liked}
						disabled={authWaiting}
						aria-busy={likeBusy || authWaiting || undefined}
						aria-label={liked ? "取消按讚" : "按讚"}
					>
						<span className="inline-flex items-center gap-px">
							<HeartIcon filled={liked} />
							{likeCount > 0 && <span>{formatCount(likeCount)}</span>}
						</span>
					</Button>
				</div>

				<div className="hkui-feed-card__body">
					{post.content && (
						<HashtagText
							as="div"
							className="hkui-feed-card__title"
							content={post.content}
						/>
					)}

					<div className="hkui-feed-card__author">
						<Avatar
							src={post.logoKey}
							name={post.sellerName}
							alt={post.sellerName || ""}
							entity="seller"
							className="hkui-feed-card__author-avatar"
							fallback="empty"
							loading={imageLoading}
							fetchPriority="low"
						/>
						<span className="hkui-feed-card__author-name">
							{post.sellerName}
						</span>
					</div>
				</div>
			</div>
			<AuthGateWaitModal open={authWaiting} />
		</>
	);
});

export default PostCardV2;
