import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

async function source(relativePath: string): Promise<string> {
	return readFile(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

test("seller storefront keeps critical media and server-owned content", async () => {
	const [products, posts, feed, feedShell, reviews, messages, showcaseWarmer] =
		await Promise.all([
			source("features/sellers/storefront/SellerProductsClient.tsx"),
			source("features/sellers/posts/SellerPostsPage.tsx"),
			source("features/feed/components/InfiniteFeed.tsx"),
			source("features/feed/components/FeedClientShell.tsx"),
			source("features/sellers/reviews/ReviewList.tsx"),
			source("features/sellers/components/SellerMessageTab.tsx"),
			source(
				"features/seller/landing/components/SellerShowcaseImageWarmer.tsx",
			),
		]);

	assert.match(products, /priority=\{index === 0\}/);
	assert.match(products, /imageLoading="eager"/);
	assert.doesNotMatch(products, /priority=\{index < \d+\}/);

	assert.match(posts, /<InfiniteFeed/);
	assert.match(feed, /progressiveImages = false/);
	assert.match(
		feed,
		/imageLoading=\{!progressiveImages \|\| index < 4 \? "eager" : "lazy"\}/,
	);
	assert.match(feedShell, /<InfiniteFeed\s+progressiveImages/);
	assert.doesNotMatch(posts, /progressiveImages/);
	assert.match(feed, /imagePriority=\{index === 0\}/);
	assert.doesNotMatch(feed, /IntersectionObserver/);

	assert.equal(
		reviews.match(/loading="eager"/g)?.length,
		2,
		"review product thumbnails and evidence photos must both be eager",
	);
	assert.doesNotMatch(reviews, /loading="lazy"/);

	assert.doesNotMatch(messages, /loading="lazy"|IntersectionObserver/);

	assert.match(showcaseWarmer, /useEffect\(\(\) =>/);
	assert.match(showcaseWarmer, /image\.fetchPriority = "low"/);
	assert.match(showcaseWarmer, /return null/);
	assert.doesNotMatch(showcaseWarmer, /<script|dangerouslySetInnerHTML/);
});

// Opening the 貼文 tab must show the posts the server already fetched. A scoped
// feed query that drops its seed stays pending, and a pending feed renders the
// full-page skeleton — the loading surface that replaces delivered content and
// that the soft-navigation E2E forbids.
test("the seller posts tab keeps the page one its own server render fetched", async () => {
	const [postsPage, feed, hook] = await Promise.all([
		source("features/sellers/posts/SellerPostsPage.tsx"),
		source("features/feed/components/InfiniteFeed.tsx"),
		source("lib/hooks/useFeedPosts.ts"),
	]);

	// The page fetches page one for this very seller, then declares that seed as
	// belonging to the feed's scoped query.
	assert.match(postsPage, /fetchPublicPostsTyped\(\{[\s\S]*?sellerId: id,/);
	assert.match(postsPage, /initialPostsMatchQuery/);
	assert.match(feed, /initialPostsMatchQuery,/);
	// Trusting a scoped seed stays opt-in: the canonical feed seeds itself, and
	// no other caller may seed from posts fetched for a different key.
	assert.match(
		hook,
		/const seedMatchesQuery = isCanonical \|\| initialPostsMatchQuery;/,
	);
	assert.match(
		hook,
		/hasInitialData =\s*restored !== null \|\| \(!initialSeedFailed && seedMatchesQuery\);/,
	);
	// The skeleton branch this protects is driven by the query's pending state.
	assert.match(hook, /initialLoading: query\.isPending/);
});
