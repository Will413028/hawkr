"use client";

import { SellerCapability } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { useSellerIdentity } from "@/components/shared/useSellerIdentity";
import Button from "@/components/ui/Button";
import { PostImageIcon } from "@/features/feed/components/PostMetricIcons";
import {
	SellerEmptyState,
	SellerPostsSkeleton,
} from "@/features/seller-center";
import {
	SellerMobileDisclosureIcon,
	SellerMobileDisclosureList,
} from "@/features/seller-center/components/SellerMobileDisclosureList";
import { SellerMobileDisclosureSkeleton } from "@/features/seller-center/components/SellerPageSkeletons";
import SellerPostDetailModal from "@/features/seller-center/posts/SellerPostDetailModal";
import type { MyPostDetail, PostListItem } from "@/lib/api/openapi-types";
import {
	POST_PUBLISH_STATUS_LABEL,
	PostPublishStatus,
} from "@/lib/codemeta/enums";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useSellerSelf } from "@/lib/hooks/useSellerSelf";
import { postCardPresetForMedia } from "@/lib/media/cdn";
import { capabilityStatus, storeIdOf } from "@/lib/seller/store";
import { useSellerDataScope } from "@/lib/seller-data/scope";
import {
	PostDraftBadge,
	PostPublishStatusBadge,
	PostUnpublishedBadge,
} from "./PostDraftBadge";
import { PostHiddenBadge } from "./PostHiddenBadge";
import PostPublishControl from "./PostPublishControl";
import { isPostHidden } from "./postHidden";
import {
	type PostPublishTarget,
	postPublishConfirmText,
} from "./postPublishActions";
import { useSellerPostsViewport } from "./useSellerPostsViewport";

const PlusIcon = () => <AppIcon name="plus" size={17} />;

function formatCount(value?: number | null): string {
	const n = Number(value ?? 0);
	if (n >= 10000) return `${(n / 10000).toFixed(1)}萬`;
	if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
	return n.toLocaleString();
}

/** MyPostDetail → PostListItem so a deep-linked post the list page didn't return
 *  (older than the global 50, or a TSID JSON.parse rounded) can still open the
 *  modal. The id comes back through the owner's `/app/sellers/posts/{id}` path,
 *  so it's exact. */
function detailToListItem(d: MyPostDetail): PostListItem {
	return {
		id: d.id,
		sellerId: d.sellerId,
		// The detail endpoint carries no storeCode. Harmless here — this is the
		// seller's own post list in seller-center and it renders no storefront
		// link at all.
		storeCode: "",
		sellerName: d.sellerName,
		logoKey: d.logoKey,
		content: d.content ?? "",
		imageKeys: d.imageKeys ?? [],
		tags: d.tags ?? [],
		product: d.products?.[0] ?? null,
		commentCount: d.commentCount,
		likeCount: d.likeCount,
		shareCount: d.shareCount,
		viewCount: d.viewCount,
		isLiked: d.isLiked,
		createdAt: d.createdAt,
		hiddenAt: d.hiddenAt,
		hiddenReason: d.hiddenReason,
		publishStatus: d.publishStatus,
	};
}

function PostStat({
	icon,
	label,
	value,
}: {
	icon: React.ReactNode;
	label: string;
	value?: number | null;
}) {
	return (
		<span
			className="seller-posts__stat"
			title={`${label} ${formatCount(value)}`}
		>
			<span className="sr-only">{label}</span>
			<span className="seller-posts__stat-icon">{icon}</span>
			<span>{formatCount(value)}</span>
		</span>
	);
}

function postKey(post: PostListItem): string {
	return String(post.id);
}

function postFirstLine(post: PostListItem): string {
	return post.content?.trim().split(/\r?\n/, 1)[0]?.trim() || "(無文字)";
}

function postInteractionMeta(post: PostListItem, delegated: boolean): string {
	if (delegated) return formatTaipeiDate(post.createdAt) || "";
	const engagement =
		Number(post.likeCount || 0) +
		Number(post.commentCount || 0) +
		Number(post.shareCount || 0);
	return `${formatCount(engagement)} 互動`;
}

function MobilePostThumbnail({
	post,
	enabled,
}: {
	post: PostListItem;
	enabled: boolean;
}) {
	const imageKey = post.imageKeys[0];
	if (!imageKey || !enabled) {
		return <SellerMobileDisclosureIcon name="image" />;
	}

	return (
		<AppImage
			src={imageKey}
			preset={postCardPresetForMedia(imageKey)}
			alt=""
			width={32}
			height={32}
			sizes="32px"
			loading="eager"
			className="size-full object-cover"
			fallbackLabel="動態圖片無法顯示"
		/>
	);
}

/**
 * The desktop list intentionally suppresses the routine published state, but a
 * 58px phone summary needs an explicit status to stay scannable. The label still
 * comes from CodeMeta; a missing upstream value remains blank rather than being
 * guessed as published.
 */
function MobilePostStatus({ post }: { post: PostListItem }) {
	if (isPostHidden(post)) return <PostHiddenBadge />;
	if (!post.publishStatus) return null;
	if (post.publishStatus === PostPublishStatus.DRAFT) return <PostDraftBadge />;
	if (post.publishStatus === PostPublishStatus.UNPUBLISHED)
		return <PostUnpublishedBadge />;
	if (post.publishStatus !== PostPublishStatus.PUBLISHED) return null;

	return (
		<span className="inline-flex items-center whitespace-nowrap rounded-full bg-hkui-primary/12 px-2 py-0.5 text-[0.6875rem] font-bold text-hkui-primary">
			{POST_PUBLISH_STATUS_LABEL[post.publishStatus]}
		</span>
	);
}

/** Seller posts list backed by the authenticated `/app/sellers/me/posts`. */
export default function SellerPostsClient() {
	const queryClient = useQueryClient();
	const scope = useSellerDataScope();
	const delegated = scope.mode === "delegated";
	const { data: seller, loading: sellerLoading } = useSellerSelf();
	// 停權判斷走 `useSellerIdentity`（與 SellerCenterShell 同一個述詞、同一份
	// seller-self 快取），不在這裡自己比對 lifecycleStatus —— 兩處各寫一份的話，
	// 46 §3.6 那條 `suspended_governance` 只會被補進其中一處。
	const { isSuspended } = useSellerIdentity();
	// 代行下 `useSellerSelf()` 是店面那一支（scope 已換），但工作區的側欄不掛它 ——
	// 這裡的店家鍵一律優先取工作階段下發的 `ownerStoreId`，才不會因為 storefront.read
	// 沒核發就整頁不載入。
	const sellerId = scope.ownerStoreId ?? storeIdOf(seller);
	const composeHref = delegated
		? `/adm/${encodeURIComponent(scope.ownerStoreId ?? "")}/posts/new`
		: "/seller-center/posts/new";
	const [deleteError, setDeleteError] = React.useState<string | null>(null);
	const [publishError, setPublishError] = React.useState<string | null>(null);
	const [activePostId, setActivePostId] = React.useState<
		number | string | null
	>(null);
	const imageViewport = useSellerPostsViewport();
	const showDesktopMedia = imageViewport === "desktop";
	const showMobileMedia = imageViewport === "mobile";

	// `/app/sellers/me/posts` returns the authenticated seller's own posts
	// (including platform-hidden ones) — no client-side seller filtering
	// needed; the cache entry is still keyed by sellerId.
	const postsKey = scope.keys.posts(sellerId ?? "none", { pageSize: 50 });
	const query = useQuery({
		queryKey: postsKey,
		queryFn: () =>
			scope.repos.posts.fetchSellerPosts(sellerId, { pageSize: 50 }),
		enabled: scope.canRead("posts", Boolean(sellerId)),
	});
	const items = query.data ?? [];
	// 代行下 `useSellerSelf()` 可能根本沒掛（storefront.read 未核發），它的 loading
	// 不能綁住貼文列表。
	const loading = (delegated ? false : sellerLoading) || query.isLoading;
	const error =
		deleteError ??
		publishError ??
		(query.error instanceof Error ? query.error.message : null);

	// Deep-link: a 賣家被留言/被回覆 notification lands on `?post=<id>` (and
	// `&comment=<id>`, which the modal's <PostComments> reads on its own). Open
	// the post's modal one time so the seller jumps straight to the thread.
	// Prefer the in-list copy; if it isn't on the loaded page (older than the
	// global 50, or a rounded TSID), fetch its detail by the exact id. Runs once
	// per mount — manual close stays closed.
	const [deepLinkPost, setDeepLinkPost] = React.useState<PostListItem | null>(
		null,
	);
	const deepLinkDoneRef = React.useRef(false);
	React.useEffect(() => {
		if (deepLinkDoneRef.current || typeof window === "undefined") return;
		if (query.isLoading) return;
		const targetPost = new URLSearchParams(window.location.search).get("post");
		if (!targetPost) {
			deepLinkDoneRef.current = true;
			return;
		}
		deepLinkDoneRef.current = true;
		const match = (query.data ?? []).find((p) => String(p.id) === targetPost);
		if (match) {
			setActivePostId(match.id);
			return;
		}
		let cancelled = false;
		scope.repos.posts
			.fetchSellerPostDetail(targetPost)
			.then((detail) => {
				if (!cancelled) setDeepLinkPost(detailToListItem(detail));
			})
			.catch(() => {});
		return () => {
			cancelled = true;
		};
	}, [query.isLoading, query.data, scope.repos.posts]);

	// Optimistically drop the row from the cache; roll back if the delete fails.
	const deleteMutation = useMutation({
		mutationFn: (id: number | string) => scope.repos.posts.deleteSellerPost(id),
		onMutate: async (id: number | string) => {
			setDeleteError(null);
			await queryClient.cancelQueries({ queryKey: postsKey });
			const before = queryClient.getQueryData<PostListItem[]>(postsKey);
			queryClient.setQueryData<PostListItem[]>(postsKey, (old) =>
				(old ?? []).filter((p) => p.id !== id),
			);
			return { before };
		},
		onError: (_e, _id, ctx) => {
			if (ctx?.before) queryClient.setQueryData(postsKey, ctx.before);
			setDeleteError("刪除失敗");
		},
	});

	const handleDelete = (id: number | string) => {
		if (!window.confirm("刪除這則動態？")) return;
		deleteMutation.mutate(id);
	};

	/**
	 * 發布／下架。**不做樂觀更新**：`publish_status` 是後端算出來的（下架不清
	 * `published_at`、排程會被一併結束），先在快取裡改一個猜的值，回滾時就得把
	 * 那些一起猜回去。改成成功後 invalidate 讓列表重抓權威值 —— 一次往返換掉一個
	 * 會說謊的樂觀值。刪除那一支可以樂觀是因為它的目標狀態只有「這一列消失」。
	 */
	const publishMutation = useMutation({
		mutationFn: ({
			id,
			target,
		}: {
			id: number | string;
			target: PostPublishTarget;
		}) => scope.repos.posts.setSellerPostPublishStatus(id, target),
		onMutate: () => setPublishError(null),
		onSuccess: () => queryClient.invalidateQueries({ queryKey: postsKey }),
		onError: (e) =>
			setPublishError(e instanceof Error ? e.message : "操作失敗，請稍後再試"),
	});

	const handleTogglePublish = (
		id: number | string,
		target: PostPublishTarget,
		scheduledPublishAt?: string,
	) => {
		if (!window.confirm(postPublishConfirmText(target, !!scheduledPublishAt)))
			return;
		publishMutation.mutate({ id, target });
	};
	const activePost =
		activePostId === null
			? (deepLinkPost ?? null)
			: (items.find((post) => String(post.id) === String(activePostId)) ??
				deepLinkPost ??
				null);

	return (
		<SellerPageShell
			className="seller-posts"
			title="動態管理"
			// 代行的列表刻意含草稿與排程中的貼文（那正是要處理的那一批），所以這裡
			// 不能沿用賣家端「已發布 N 則」——賣家自建的貼文一律直接發布，那句在
			// 賣家路徑上為真，在代行路徑上會把草稿一起宣告成已發布。
			subtitle={
				delegated
					? `共 ${items.length} 則動態`
					: `已發布 ${items.length} 則動態`
			}
			actions={
				<Button href={composeHref}>
					<PlusIcon />
					<span>發布動態</span>
				</Button>
			}
		>
			{error && (
				<p className="seller-posts__error" role="alert">
					{error}
				</p>
			)}

			<div className="seller-card seller-posts__card seller-mobile-disclosure-desktop">
				{loading && items.length === 0 ? (
					<SellerPostsSkeleton count={4} delegated={delegated} />
				) : items.length === 0 ? (
					<div className="seller-posts__empty">
						<SellerEmptyState variant="posts" seamless />
						<Button href={composeHref}>發布第一篇</Button>
					</div>
				) : (
					<ul className="seller-posts__list">
						{items.map((p) => (
							<li key={p.id} className="seller-posts__row">
								{/* Cover + main share one click target via a button with
								    `display: contents`, so the row's CSS grid
								    (132px / 1fr / auto) stays intact and the 刪除 button
								    in the third column is unaffected by the click. Opens
								    the in-page modal (same engagement surface buyers see,
								    plus the comments composer the seller can reply with). */}
								<button
									type="button"
									className="contents cursor-pointer border-0 bg-transparent p-0 text-left font-inherit text-inherit"
									onClick={() => setActivePostId(p.id)}
									aria-label={`查看動態：${p.content?.slice(0, 30) || "未命名動態"}`}
								>
									<div className="seller-posts__cover">
										{showDesktopMedia && p.imageKeys[0] ? (
											<AppImage
												src={p.imageKeys[0]}
												preset={postCardPresetForMedia(p.imageKeys[0])}
												alt=""
												width={132}
												height={99}
												sizes="132px"
												loading="eager"
												className="size-full object-contain"
												fallbackLabel="動態圖片無法顯示"
											/>
										) : (
											<span className="seller-posts__cover-placeholder">
												<PostImageIcon />
											</span>
										)}
									</div>
									<div className="seller-posts__main">
										{isPostHidden(p) && (
											<div className="mb-1.5 flex items-center gap-2">
												<PostHiddenBadge />
												{p.hiddenReason ? (
													<span className="truncate text-xs text-danger-700/80 dark:text-[#e6b8a6]/80">
														{p.hiddenReason}
													</span>
												) : null}
											</div>
										)}
										{/* 平台下架與發布狀態互斥呈現：一則貼文若被平台下架，那是更重要的
										    資訊。發布狀態缺值時什麼都不畫 —— 賣家自己那條路徑的上游不帶
										    publishStatus，推定成「已發布」會是一個沒有根據的宣稱。 */}
										{!isPostHidden(p) && (
											<PostPublishStatusBadge
												publishStatus={p.publishStatus}
												scheduledPublishAt={p.scheduledPublishAt}
												className="mb-1.5 flex items-center gap-2"
											/>
										)}
										<p className="seller-posts__excerpt whitespace-pre-wrap">
											{p.content?.slice(0, 120) || "(無文字)"}
										</p>
										{/* 代行的貼文 DTO 沒有互動計數：畫 0 是一個宣稱（「這則貼文有 0 個讚」），
										    而拿不到權威數字時正解是不畫那個數字，不是編一個。 */}
										{!delegated && (
											<div className="seller-posts__stats">
												<PostStat
													icon={<AppIcon name="heart" size={18} />}
													label="按讚"
													value={p.likeCount}
												/>
												<PostStat
													icon={<AppIcon name="chat" size={18} />}
													label="留言"
													value={p.commentCount}
												/>
												<PostStat
													icon={<AppIcon name="share" size={18} />}
													label="分享"
													value={p.shareCount}
												/>
												<PostStat
													icon={<AppIcon name="show" size={18} />}
													label="瀏覽"
													value={p.viewCount}
												/>
											</div>
										)}
									</div>
								</button>
								{/* 代行不渲染這兩顆：貼文編輯頁尚未接線，而「刪除貼文」在代行的端點
								    總表裡根本沒有對應路徑（能做的是下架，走 /unpublish）。 */}
								{!delegated && (
									<div className="seller-posts__actions">
										{/* 發布／下架擺在動作欄的第一顆：它改的是「這則貼文買家看不看得到」，
										    比編輯內容更接近賣家在這一頁要做的判斷。方向由狀態決定，停權時
										    發布方向整顆不渲染（下架留著＝收縮型自清）—— 判準全在
										    PostPublishControl，這裡不再自行分支。 */}
										<PostPublishControl
											publishStatus={p.publishStatus}
											hidden={isPostHidden(p)}
											suspended={isSuspended}
											hasImage={Boolean(p.imageKeys[0])}
											busy={
												publishMutation.isPending &&
												publishMutation.variables?.id === p.id
											}
											onToggle={(target) =>
												handleTogglePublish(p.id, target, p.scheduledPublishAt)
											}
										/>
										<Button
											variant="bordered"
											size="sm"
											href={`/seller-center/posts/${p.id}/edit`}
										>
											編輯
										</Button>
										<Button
											variant="bordered"
											color="danger"
											size="sm"
											onClick={() => void handleDelete(p.id)}
										>
											刪除
										</Button>
									</div>
								)}
							</li>
						))}
					</ul>
				)}
			</div>

			{loading && items.length === 0 ? (
				<SellerMobileDisclosureSkeleton count={4} />
			) : items.length === 0 ? (
				<div className="seller-mobile-disclosure-list">
					<div className="seller-card seller-posts__empty">
						<SellerEmptyState variant="posts" seamless />
						<Button href={composeHref}>發布第一篇</Button>
					</div>
				</div>
			) : (
				<SellerMobileDisclosureList
					items={items}
					getKey={postKey}
					ariaLabel="動態清單"
					resetKey={String(sellerId ?? "")}
					getSummary={(post) => ({
						label: `動態：${postFirstLine(post).slice(0, 30)}`,
						leading: (
							<MobilePostThumbnail post={post} enabled={showMobileMedia} />
						),
						title: postFirstLine(post),
						meta: postInteractionMeta(post, delegated),
						status: <MobilePostStatus post={post} />,
					})}
					deferDetailsUntilExpanded
					renderDetails={(post) => (
						<>
							<div className="seller-mobile-disclosure__body">
								{showMobileMedia && post.imageKeys[0] ? (
									<div className="relative aspect-[4/3] w-full overflow-hidden rounded-xl bg-content2">
										<AppImage
											src={post.imageKeys[0]}
											preset={postCardPresetForMedia(post.imageKeys[0])}
											alt=""
											fill
											sizes="(max-width: 767px) 100vw, 0px"
											loading="eager"
											className="size-full object-contain"
											fallbackLabel="動態圖片無法顯示"
										/>
									</div>
								) : null}

								{isPostHidden(post) ? (
									<div className="flex min-w-0 items-center gap-2">
										<PostHiddenBadge />
										{post.hiddenReason ? (
											<span className="truncate text-xs text-danger-700/80 dark:text-[#e6b8a6]/80">
												{post.hiddenReason}
											</span>
										) : null}
									</div>
								) : (
									<PostPublishStatusBadge
										publishStatus={post.publishStatus}
										scheduledPublishAt={post.scheduledPublishAt}
										className="flex items-center gap-2"
									/>
								)}

								<p className="m-0 whitespace-pre-wrap text-sm font-semibold leading-6 text-foreground">
									{post.content?.slice(0, 240) || "(無文字)"}
								</p>

								<div className="seller-mobile-disclosure__detail-row">
									<span>建立日期</span>
									<time>{formatTaipeiDate(post.createdAt) || "—"}</time>
								</div>

								{!delegated ? (
									<div className="seller-mobile-disclosure__facts">
										<span className="seller-mobile-disclosure__fact inline-flex items-center gap-1.5">
											<AppIcon name="heart" size={16} />
											按讚 {formatCount(post.likeCount)}
										</span>
										<span className="seller-mobile-disclosure__fact inline-flex items-center gap-1.5">
											<AppIcon name="chat" size={16} />
											留言 {formatCount(post.commentCount)}
										</span>
										<span className="seller-mobile-disclosure__fact inline-flex items-center gap-1.5">
											<AppIcon name="share" size={16} />
											分享 {formatCount(post.shareCount)}
										</span>
										<span className="seller-mobile-disclosure__fact inline-flex items-center gap-1.5">
											<AppIcon name="show" size={16} />
											瀏覽 {formatCount(post.viewCount)}
										</span>
									</div>
								) : null}
							</div>

							<div className="seller-mobile-disclosure__actions">
								<Button
									type="button"
									variant="bordered"
									size="sm"
									onClick={() => setActivePostId(post.id)}
								>
									查看完整動態
								</Button>
								{!delegated ? (
									<>
										<PostPublishControl
											publishStatus={post.publishStatus}
											hidden={isPostHidden(post)}
											suspended={isSuspended}
											hasImage={Boolean(post.imageKeys[0])}
											busy={
												publishMutation.isPending &&
												publishMutation.variables?.id === post.id
											}
											onToggle={(target) =>
												handleTogglePublish(
													post.id,
													target,
													post.scheduledPublishAt,
												)
											}
										/>
										<Button
											variant="bordered"
											size="sm"
											href={`/seller-center/posts/${post.id}/edit`}
										>
											編輯
										</Button>
										<Button
											variant="bordered"
											color="danger"
											size="sm"
											onClick={() => void handleDelete(post.id)}
										>
											刪除
										</Button>
									</>
								) : null}
							</div>
						</>
					)}
				/>
			)}
			<SellerPostDetailModal
				open={activePost !== null}
				post={activePost}
				storePublic={
					capabilityStatus(seller, SellerCapability.PUBLIC_LISTING)?.active
				}
				onClose={() => {
					setActivePostId(null);
					setDeepLinkPost(null);
				}}
			/>
		</SellerPageShell>
	);
}
