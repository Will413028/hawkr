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
			// canonical 定址碼（#2098 階段三）。gateway 查不到賣家時回空字串，
			// 這裡轉成 undefined 讓 sellerPath 的型別語意一致。
			storeCode: p.storeCode || undefined,
			name: p.sellerName,
			avatar: p.logoKey,
			// 刻意不設 hasUnread：`!isLiked` 不是「有新動態」訊號（訪客未登入
			// 時恆為 true），整排都會亮著狀態環 → 環就失去語意，而且 /market
			// 的同一條 rail 沒有這個欄位，兩個頁面會長得不一樣。
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
	// A failed seed is explicitly distinguished from a valid empty feed so the
	// client query retries instead of persisting a false empty state. The cached
	// reader carries its own upstream timeout.
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
