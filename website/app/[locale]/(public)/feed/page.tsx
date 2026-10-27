import FeedPage from "@/features/feed/FeedPage";
import { pageMetadata } from "@/lib/seo/metadata";

// Keep the already-rendered tab tree in Next's in-memory router cache so a tab
// revisit paints its last snapshot without another RSC flight (live data
// refreshes in the background on activation).
export const unstable_dynamicStaleTime = 300;

// ISR (#338): the server renders the canonical unscoped feed only; `?seller` /
// `?hashtag` / `?topic` / `?post` are read on the client (FeedParamController)
// so the route no longer server-renders per request. Post data comes from the
// bounded public transport; `revalidate` is the static-shell TTL safety net,
// aligned with the feed data window.
export const revalidate = 60;

export const generateMetadata = pageMetadata({
	title: { absolute: "Hawkr 動態 — 食材故事與店家貼文" },
	description:
		"在 Hawkr 動態追蹤你喜愛店家的最新貼文與消息，看見食材背後的故事、季節新品與限時優惠活動。跟著在地小農與職人的日常分享，認識每一天的產地餐桌，發現更多值得收藏的生鮮店家與當季好物，讓採買不只是交易，更是一段與生產者互動、認識食物來源的溫暖旅程與生活靈感。",
	path: "/feed",
});

export default function Page() {
	return (
		<>
			<h1 className="sr-only">Hawkr 動態</h1>
			<FeedPage />
		</>
	);
}
