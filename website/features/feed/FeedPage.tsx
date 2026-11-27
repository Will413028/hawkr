import type { StoryItem } from "@/features/feed/components/StoryRail";
import FeedParamController from "@/features/feed/FeedParamController";
import { getCachedPublicPosts } from "@/lib/api/cached";
import type { PostListItem } from "@/lib/api/openapi-types";

const PAGE_SIZE = 12;

function buildStories(posts: PostListItem[]): StoryItem[] {
	const seen = new Set<string>();
	const stories: StoryItem[] = [];
	for (const p of posts) {
		const sellerKey = String(p.sellerId);
		if (seen.has(sellerKey)) continue;
		seen.add(sellerKey);
		stories.push({
			id: p.sellerId,
			storeCode: p.storeCode || undefined,
			name: p.sellerName,
			avatar: p.logoKey,
		});
		if (stories.length >= 12) break;
	}
	return stories;
}

/**
 * `/feed` (ISR). The server renders the canonical, unscoped page-1 feed — the
 * SEO body — from the anonymous cached reader (`getCachedPublicPosts`, tagged
 * `posts:list` so social's invalidation reaches it), reading no request-time
 * input so the route stays statically renderable. A signed-in viewer's list
 * depends on the viewer (blocked stores, `viewer_*`), so `useFeedPosts` drops
 * this seed and reads page one itself with the viewer's cookie. All URL params (`?seller` / `?hashtag`
 * / `?topic` / `?post`) are read on the client by `FeedParamController`, which
 * re-scopes the feed, shows the topic banner, and opens deep-linked posts — no
 * per-request dynamic server render.
 */
export default async function FeedPage() {
	let initialPosts: PostListItem[];
	let initialNextPageToken = "";
	let initialSeedFailed = false;
	try {
		const page = await getCachedPublicPosts(PAGE_SIZE);
		initialPosts = page.records;
		initialNextPageToken = page.nextPageToken;
	} catch {
		initialPosts = [];
		initialSeedFailed = true;
	}
	const stories = buildStories(initialPosts);

	return (
		<div className="hkui-feed-page" data-hk-tab-content-ready="feed">
			<section className="hkui-feed-page__main">
				<FeedParamController
					initialPosts={initialPosts}
					initialNextPageToken={initialNextPageToken}
					stories={stories}
					pageSize={PAGE_SIZE}
					initialSeedFailed={initialSeedFailed}
				/>
			</section>
		</div>
	);
}
