"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import Avatar from "@/components/shared/Avatar";
import HashtagText from "@/features/feed/components/HashtagText";
import PostComments from "@/features/feed/components/PostComments";
import { EmptyState } from "@/features/seller-center/components/EmptyState";
import type { PostListItem } from "@/lib/api/openapi-types";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import { postCardPresetForMedia } from "@/lib/media/cdn";
import { PostPublishStatusBadge } from "./PostDraftBadge";
import {
	POST_COMMENTS_UNAVAILABLE_TEXT,
	postCommentsAvailability,
} from "./postCommentsAvailability";
import { isPostHidden } from "./postHidden";

interface Props {
	open: boolean;
	post: PostListItem | null;
	/** 店家的 `PUBLIC_LISTING` 是否啟用；不知道（代行、尚未載入）時是 `undefined`。 */
	storePublic: boolean | undefined;
	onClose: () => void;
}

const CloseIcon = () => <AppIcon name="close" size={20} />;

function formatCount(n: number | null | undefined): string {
	const v = Number(n ?? 0);
	if (v >= 10000) return `${(v / 10000).toFixed(1)}萬`;
	if (v >= 1000) return `${(v / 1000).toFixed(1)}k`;
	return v.toLocaleString();
}

/**
 * Seller-side popup that shows a single post in an overlay (matches the
 * LoginRequiredModal pattern — portal + backdrop + Escape + scroll-lock)
 * so the seller can read engagement and reply to buyer comments without
 * leaving the seller-center posts page. Reuses the buyer-facing
 * `<PostComments>` for the comment thread; any logged-in user (including
 * the seller themselves) can post a reply through its composer. A post that is
 * not public right now (store not listed, draft or unpublished, platform-hidden)
 * has no readable public thread, so the thread is replaced by the reason.
 */
export default function SellerPostDetailModal({
	open,
	post,
	storePublic,
	onClose,
}: Props) {
	const [mounted, setMounted] = React.useState(false);

	const sellerAvatar = post?.logoKey;
	const sellerName = post?.sellerName || "";

	React.useEffect(() => {
		setMounted(true);
	}, []);

	React.useEffect(() => {
		if (!open) return;
		const onKey = (event: KeyboardEvent) => {
			if (event.key === "Escape") onClose();
		};
		const releaseScrollLock = acquireBodyScrollLock();
		document.addEventListener("keydown", onKey);
		return () => {
			document.removeEventListener("keydown", onKey);
			releaseScrollLock();
		};
	}, [open, onClose]);

	if (!open || !mounted || !post) return null;

	const imageKey = post.imageKeys[0] ?? "";
	const commentsAvailability = postCommentsAvailability(post, storePublic);

	return createPortal(
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="seller-post-modal-title"
			className="fixed inset-0 z-[1000] flex items-center justify-center p-4 max-md:p-0"
		>
			<button
				type="button"
				aria-label="關閉"
				onClick={onClose}
				className="absolute inset-0 border-0 cursor-default bg-black/55 backdrop-blur-sm"
			/>
			<section className="seller-post-modal__surface relative z-[1] flex w-full max-w-[1080px] max-h-[88vh] overflow-hidden rounded-[24px] bg-content1 shadow-2xl max-md:h-full max-md:max-h-full max-md:rounded-none max-md:flex-col">
				<button
					type="button"
					onClick={onClose}
					aria-label="關閉"
					className="absolute top-2 right-2 z-[2] grid size-11 place-items-center rounded-full bg-content2/90 backdrop-blur text-foreground border-0 cursor-pointer transition-colors hover:bg-content2 md:top-3 md:right-3"
				>
					<CloseIcon />
				</button>

				{/* Media — left on desktop, top on mobile */}
				<div className="seller-post-modal__media relative grid flex-1 min-w-0 place-items-center bg-default-100 min-h-[320px] max-md:flex-none max-md:aspect-[4/3]">
					{imageKey ? (
						<AppImage
							src={imageKey}
							preset={postCardPresetForMedia(imageKey)}
							alt={post.content?.slice(0, 40) ?? ""}
							fill
							sizes="(max-width: 767px) 100vw, 660px"
							className="seller-post-modal__image max-h-[88vh] max-w-full object-contain max-md:max-h-full"
							fallbackLabel="動態圖片無法顯示"
						/>
					) : (
						<span className="text-foreground-soft text-sm">無圖片</span>
					)}
				</div>

				{/* Aside — right on desktop, bottom on mobile */}
				<aside className="seller-post-modal__aside flex w-[420px] max-w-full shrink-0 flex-col overflow-y-auto border-l border-divider max-md:w-full max-md:border-l-0 max-md:border-t">
					<header
						id="seller-post-modal-title"
						className="flex items-center gap-3 p-4 border-b border-divider"
					>
						<Avatar
							src={sellerAvatar}
							name={sellerName}
							alt=""
							entity="seller"
							className="size-10 shrink-0 rounded-full object-cover"
							fallbackClassName="grid place-items-center bg-default-200 text-sm font-bold text-foreground"
							fallback="empty"
						/>
						<div className="min-w-0 flex-1">
							<div className="font-semibold truncate text-foreground">
								{sellerName}
							</div>
							<div className="mt-0.5 flex items-center gap-3 text-xs text-foreground-soft">
								<span className="inline-flex items-center gap-1">
									<AppIcon name="heart" size={16} />
									{formatCount(post.likeCount)}
								</span>
								<span className="inline-flex items-center gap-1">
									<AppIcon name="chat" size={16} />
									{formatCount(post.commentCount)}
								</span>
								<span className="inline-flex items-center gap-1">
									<AppIcon name="show" size={16} />
									{formatCount(post.viewCount)}
								</span>
							</div>
						</div>
					</header>

					{/* 列表上標了、點進來卻沒標，等於把剛建立的判斷又拆掉一次 —— 代行者
					    正是從列表點進這裡確認內容的。 */}
					{!isPostHidden(post) && (
						<PostPublishStatusBadge
							publishStatus={post.publishStatus}
							scheduledPublishAt={post.scheduledPublishAt}
							className="mx-4 mt-3 flex items-center gap-2"
						/>
					)}

					{isPostHidden(post) && (
						<div className="mx-4 mt-3 rounded-xl bg-danger-50 p-3 text-danger-700 dark:bg-[#b45644]/20 dark:text-[#e6b8a6]">
							<p className="text-sm font-semibold">此貼文已被平台下架</p>
							<p className="mt-1 text-sm">
								{post.hiddenReason && post.hiddenReason.length > 0
									? post.hiddenReason
									: "此貼文因違反平台規範已被下架，如有疑問請聯繫客服。"}
							</p>
						</div>
					)}

					{post.content ? (
						<HashtagText
							as="div"
							content={post.content}
							className="px-4 py-3 text-sm leading-relaxed whitespace-pre-wrap break-words text-foreground"
						/>
					) : (
						<p className="px-4 py-3 text-sm text-foreground-soft">(無文字)</p>
					)}

					{/* canModerate=true: this surface is only opened from the seller's
					    own posts list, so the viewer is by definition the post owner
					    and the backend grants DELETE on any comment under their posts
					    (own comment OR post owner). Lets the seller remove abusive /
					    off-topic buyer comments and replies. */}
					{commentsAvailability === "available" ? (
						<PostComments postId={post.id} canModerate />
					) : (
						<EmptyState>
							{POST_COMMENTS_UNAVAILABLE_TEXT[commentsAvailability]}
						</EmptyState>
					)}
				</aside>
			</section>
		</div>,
		document.body,
	);
}
