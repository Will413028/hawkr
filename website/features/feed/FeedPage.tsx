import type { StoryItem } from "@/features/feed/components/StoryRail";
import FeedParamController from "@/features/feed/FeedParamController";
import type { PostListItem } from "@/lib/api/openapi-types";
import { fetchPublicPostsTyped, normalizePost } from "@/lib/api/posts";

const PAGE_SIZE = 12;
const FEED_INITIAL_DEADLINE_MS = 3_000;

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
 * SEO body — from the shared public transport, reading no request-time input so
 * the route stays statically renderable. All URL params (`?seller` / `?hashtag`
 * / `?topic` / `?post`) are read on the client by `FeedParamController`, which
 * re-scopes the feed, shows the topic banner, and opens deep-linked posts — no
 * per-request dynamic server render.
 */
export default async function FeedPage() {
	// Keep this canonical seed off the OpenNext incremental cache boundary via a
	// bounded deadline: an intermittent foreground gateway read can otherwise
	// hold a Flight body open after its 200 headers. A failed seed is explicitly
	// distinguished from a valid empty feed so the client query retries instead
	// of persisting a false empty state.
	const signal = AbortSignal.timeout(FEED_INITIAL_DEADLINE_MS);
	let initialPosts: PostListItem[];
	let initialSeedFailed = false;
	try {
		const { records } = await fetchPublicPostsTyped({
			page: 1,
			pageSize: PAGE_SIZE,
			signal,
			strict: true,
		});
		initialPosts = records.map(normalizePost);
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
					stories={stories}
					pageSize={PAGE_SIZE}
					initialSeedFailed={initialSeedFailed}
				/>
			</section>
		</div>
	);
}
