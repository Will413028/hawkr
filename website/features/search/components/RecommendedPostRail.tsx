"use client";

import * as React from "react";
import Link from "@/components/shared/Link";
import PostCardV2 from "@/features/feed/components/PostCardV2";
import type { PostListItem } from "@/lib/api/openapi-types";
import { useDragScroll } from "@/lib/hooks/useDragScroll";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

interface RecommendedPostRailProps {
	posts: PostListItem[];
}

export default function RecommendedPostRail({
	posts,
}: RecommendedPostRailProps) {
	const router = useObservedRouter("recommended_post_rail");
	const railRef = React.useRef<HTMLUListElement | null>(null);

	useDragScroll(railRef);

	const openPost = React.useCallback(
		(post: PostListItem) => {
			router.push(`/feed?post=${encodeURIComponent(String(post.id))}`);
		},
		[router],
	);

	if (!posts.length) return null;

	return (
		<section className="flex flex-col gap-3.5" aria-label="推薦動態">
			<header className="flex items-end justify-between gap-3">
				<div>
					<strong className="block text-lg text-[var(--ink-heading)] mb-0.5 dark:text-[#f5e9c3]">
						推薦動態
					</strong>
					<small className="text-[0.8125rem] text-[var(--ink-soft)]">
						看看大家最近在分享什麼
					</small>
				</div>
				<Link
					className="text-[0.8125rem] text-[var(--action-primary)] no-underline font-medium whitespace-nowrap hover:underline"
					href="/feed"
				>
					看全部 →
				</Link>
			</header>
			<ul
				ref={railRef}
				className="flex gap-4 overflow-x-auto py-0.5 px-1 pb-1 -mx-1 list-none cursor-grab select-none [scrollbar-width:none] [overscroll-behavior-x:contain] [scroll-behavior:auto] [&::-webkit-scrollbar]:hidden [&>*]:flex-none [&>*]:list-none [&_a]:[-webkit-user-drag:none] [&_img]:[-webkit-user-drag:none]"
			>
				{posts.map((post) => (
					<li
						key={`feed-${post.id}`}
						className="w-[240px] max-[680px]:w-[200px] [&_.hkui-feed-card]:h-full"
					>
						<PostCardV2 post={post} imageLoading="lazy" onOpen={openPost} />
					</li>
				))}
			</ul>
		</section>
	);
}
