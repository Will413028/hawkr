"use client";

import { useQuery } from "@tanstack/react-query";
import AppIcon from "@/components/shared/AppIcon";
import FeedClientShell from "@/features/feed/components/FeedClientShell";
import type { StoryItem } from "@/features/feed/components/StoryRail";
import type { PostDetail, PostListItem } from "@/lib/api/openapi-types";
import {
	FALLBACK_POST_IMAGE,
	fetchPostDetail,
	normalizePost,
} from "@/lib/api/posts";
import { useUrlSearchParams } from "@/lib/hooks/useUrlSearchParams";
import { normalizePostTag } from "@/lib/post-tags/repo";

const TOPIC_LABELS: Record<string, string> = {
	following: "追蹤店家",
	story: "店家故事",
	new: "新品上架",
	deal: "優惠活動",
};

/**
 * Flatten a PostDetail into a PostListItem so it can join the InfiniteFeed
 * grid. Both shapes carry `imageKeys[]`; only the product chip narrows (detail
 * has `products[]`, the list carries one), so pick the first of those.
 */
function postDetailToListItem(detail: PostDetail): PostListItem {
	return {
		id: detail.id,
		sellerId: detail.sellerId,
		// `/public/posts/{id}` does NOT carry storeCode — unlike the list, it
		// makes no seller lookup to lift the segment off, so filling it would
		// cost this endpoint an RPC it does not currently make (#2098 phase 3).
		// Empty on purpose: sellerPath then links the store_id and eats one 308,
		// which is correct-but-slower rather than broken.
		storeCode: "",
		sellerName: detail.sellerName?.trim() || "Hawkr 選品",
		logoKey: detail.logoKey?.trim() ?? "",
		content: detail.content ?? "",
		imageKeys: detail.imageKeys?.[0]?.trim()
			? [detail.imageKeys[0].trim()]
			: [FALLBACK_POST_IMAGE],
		product: detail.products?.[0]
			? {
					...detail.products[0],
					imageKey: detail.products[0].imageKey?.trim() || FALLBACK_POST_IMAGE,
					name: detail.products[0].name?.trim() || "商品",
					price: detail.products[0].price?.trim() || "",
				}
			: null,
		commentCount: detail.commentCount,
		likeCount: detail.likeCount,
		shareCount: detail.shareCount,
		viewCount: detail.viewCount,
		isLiked: detail.isLiked,
		createdAt: detail.createdAt,
		tags: detail.tags ?? [],
	};
}

interface FeedParamControllerProps {
	/** Canonical (unscoped) page-1 posts, server-rendered into the ISR HTML. */
	initialPosts: PostListItem[];
	stories: StoryItem[];
	pageSize: number;
	/** The canonical server seed timed out; let the client query recover. */
	initialSeedFailed: boolean;
}

/**
 * Client owner of `/feed`'s URL params. The route is ISR — the server renders
 * the canonical page-1 feed (the SEO body) with no request-time input, and this
 * component reads `?seller` / `?hashtag` / `?topic` / `?post` here (via
 * `useUrlSearchParams`, not `useSearchParams`, so the route stays statically
 * renderable) and applies them client-side:
 *   - seller / hashtag → re-scope the feed query (FeedClientShell → useFeedPosts)
 *   - topic            → the topic banner label
 *   - post             → fetch the post detail and open the viewer over the feed
 */
export default function FeedParamController({
	initialPosts,
	stories,
	pageSize,
	initialSeedFailed,
}: FeedParamControllerProps) {
	const params = useUrlSearchParams();
	const sellerId = params.get("sellerId") || params.get("seller") || "";
	const topic = params.get("topic") || "";
	const hashtag = normalizePostTag(params.get("hashtag") || "");
	const postId = params.get("post") || "";
	const topicLabel = topic ? TOPIC_LABELS[topic] : undefined;

	const deepLinkQuery = useQuery<PostListItem | null>({
		queryKey: ["feed", "deep-link-post", postId],
		queryFn: async () => {
			const detail = await fetchPostDetail(postId);
			return detail ? normalizePost(postDetailToListItem(detail)) : null;
		},
		enabled: Boolean(postId),
		staleTime: 60_000,
		gcTime: 5 * 60 * 1000,
	});

	return (
		<>
			{topicLabel && (
				<div
					className="flex items-center gap-2.5 py-3 px-4 mb-3 rounded-[14px] bg-[rgb(var(--color-brand-primary-strong-rgb)/0.08)] border border-[rgb(var(--color-brand-primary-strong-rgb)/0.22)] text-[length:var(--text-sm)] text-[color:var(--hkui-foreground)] flex-wrap"
					role="status"
				>
					<AppIcon
						name="pin"
						size={16}
						className="text-[color:var(--hkui-primary)]"
					/>
					<strong className="font-semibold text-[color:var(--hkui-primary)]">
						主題：{topicLabel}
					</strong>
					<small className="text-[color:var(--hkui-foreground-soft)] text-[length:var(--text-xs)]">
						已套用主題篩選 — 點選導覽列「推薦動態」可清除
					</small>
				</div>
			)}
			<FeedClientShell
				stories={stories}
				initialPosts={initialPosts}
				initialPage={1}
				pageSize={pageSize}
				sellerId={sellerId || undefined}
				hashtag={hashtag || undefined}
				hasActiveFilter={Boolean(sellerId || topic || hashtag)}
				initialSeedFailed={initialSeedFailed}
				initialPostId={postId || undefined}
				deepLinkPost={deepLinkQuery.data ?? undefined}
			/>
		</>
	);
}
