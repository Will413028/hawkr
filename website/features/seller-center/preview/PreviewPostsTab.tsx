"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import * as React from "react";
import Button from "@/components/ui/Button";
import PostCardV2 from "@/features/feed/components/PostCardV2";
import PostDetailViewer, {
	type OpenOriginRect,
} from "@/features/feed/components/PostDetailViewer";
import type { PostListItem } from "@/lib/api/openapi-types";
import { fetchPreviewPosts } from "@/lib/seller/preview-posts";
import { useSellerDataScope } from "@/lib/seller-data/scope";
import { usePreviewStore } from "./PreviewStoreContext";

function PostsEmpty() {
	return (
		<div className="seller-posts-empty">
			<h3>還沒有貼文</h3>
			<p>等賣家分享新的採收、出貨或食材故事後，這裡會自動更新。</p>
		</div>
	);
}

/**
 * 貼文分頁（唯讀）：讀店主端 `ListMyPosts` 的已發布貼文。卡片與詳情以 `readOnly` 渲染，
 * 不含按讚、留言、分享、編輯、刪除或封鎖控制。
 */
export function PreviewPostsGrid({
	posts,
	hasMore,
	loadingMore,
	onLoadMore,
}: {
	posts: PostListItem[];
	hasMore: boolean;
	loadingMore: boolean;
	onLoadMore: () => void;
}) {
	const [openId, setOpenId] = React.useState<string | null>(null);
	const [origin, setOrigin] = React.useState<OpenOriginRect | null>(null);
	const openIndex = openId
		? posts.findIndex((post) => String(post.id) === openId)
		: -1;

	return (
		<section
			className="seller-posts-board"
			data-hk-page-content-ready="seller-storefront"
		>
			{posts.length === 0 ? (
				<PostsEmpty />
			) : (
				<div className="hkui-feed-grid">
					{posts.map((post, index) => (
						<PostCardV2
							key={post.id}
							post={post}
							imageLoading="eager"
							imagePriority={index === 0}
							readOnly
							onOpen={(opened, rect) => {
								setOpenId(String(opened.id));
								setOrigin(
									rect
										? {
												top: rect.top,
												left: rect.left,
												width: rect.width,
												height: rect.height,
											}
										: null,
								);
							}}
						/>
					))}
				</div>
			)}
			{hasMore ? (
				<div className="hkui-feed-sentinel">
					<Button
						variant="bordered"
						color="primary"
						radius="full"
						isLoading={loadingMore}
						loadingLabel="載入動態中…"
						onClick={onLoadMore}
					>
						載入更多動態
					</Button>
				</div>
			) : null}
			<PostDetailViewer
				readOnly
				posts={posts}
				initialIndex={openIndex >= 0 ? openIndex : null}
				onClose={() => {
					setOpenId(null);
					setOrigin(null);
				}}
				hasMore={false}
				openRect={origin}
			/>
		</section>
	);
}

export default function PreviewPostsTab() {
	const { view } = usePreviewStore();
	const scope = useSellerDataScope();
	const query = useInfiniteQuery({
		queryKey: scope.keys.posts(view.storeId, { preview: "published" }),
		queryFn: ({ pageParam }) => fetchPreviewPosts(pageParam),
		initialPageParam: "",
		getNextPageParam: (last) => last.nextPageToken || undefined,
		staleTime: 30_000,
	});
	const posts = React.useMemo(
		() => query.data?.pages.flatMap((page) => page.items) ?? [],
		[query.data],
	);

	if (!query.data) {
		return query.isError ? (
			<p className="seller-profile__error" role="alert">
				貼文暫時載入不到，請稍後再試。
			</p>
		) : (
			<p role="status" aria-busy="true">
				正在載入貼文…
			</p>
		);
	}
	return (
		<PreviewPostsGrid
			posts={posts}
			hasMore={query.hasNextPage}
			loadingMore={query.isFetchingNextPage}
			onLoadMore={() => void query.fetchNextPage()}
		/>
	);
}
