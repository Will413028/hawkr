import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

async function source(relativePath: string): Promise<string> {
	return readFile(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

test("seller storefront keeps critical media and server-owned content", async () => {
	const [products, posts, feed, feedShell, reviews, messages, showcaseWarmer] =
		await Promise.all([
			source("features/sellers/storefront/StorefrontProductGrid.tsx"),
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

test("the seller posts tab keeps the page one its own server render fetched", async () => {
	const [postsPage, feed, hook] = await Promise.all([
		source("features/sellers/posts/SellerPostsPage.tsx"),
		source("features/feed/components/InfiniteFeed.tsx"),
		source("lib/hooks/useFeedPosts.ts"),
	]);

	assert.match(postsPage, /getCachedStorePosts\(\s*storeId,\s*POSTS_PAGE_SIZE/);
	assert.match(postsPage, /initialPostsMatchQuery/);
	assert.match(feed, /initialPostsMatchQuery,/);
	assert.match(
		hook,
		/const seedMatchesQuery = isCanonical \|\| initialPostsMatchQuery;/,
	);
	assert.match(
		hook,
		/hasInitialData =\s*restored !== null \|\|\s*\(!initialSeedFailed && seedMatchesQuery && !viewer\);/,
	);
	assert.match(hook, /initialLoading: query\.isPending/);
});
