import { beforeEach, describe, expect, it, vi } from "vitest";

const cached = vi.hoisted(() => ({
	getCachedSeller: vi.fn(),
	getCachedSellerReviewStats: vi.fn(),
	getCachedSellerReviews: vi.fn(),
	getCachedStorePosts: vi.fn(),
}));
vi.mock("@/lib/api/cached", () => cached);
vi.mock("@/lib/api/static-params", () => ({ sellerStaticParams: vi.fn() }));
vi.mock("@/features/feed/components/InfiniteFeed", () => ({
	default: () => null,
}));
vi.mock("./reviews/ReviewsClient", () => ({ default: () => null }));
vi.mock("./reviews/ReviewList", () => ({
	StarsSvg: () => null,
}));
vi.mock("@/components/seo/JsonLd", () => ({ default: () => null }));
vi.mock("./card/SellerCardPage", () => ({ default: () => null }));

import { generateMetadata as card } from "../../app/[locale]/(public)/sellers/[id]/card/page";
import { generateMetadata as posts } from "./posts/SellerPostsPage";
import { generateMetadata as reviews } from "./reviews/SellerReviewsPage";
import { generateMetadata as storefront } from "./storefront/metadata";

const pages = { card, posts, reviews, storefront };
const cases = [
	{ locale: "zh-TW", missing: "找不到賣家" },
	{ locale: "en", missing: "Seller not found" },
];

beforeEach(() => cached.getCachedSeller.mockResolvedValue(undefined));

describe.each(Object.entries(pages))("%s not-found title", (_n, gen) => {
	it.each(cases)("$locale", async (c) => {
		const params = Promise.resolve({ locale: c.locale, id: "shop" });
		expect((await gen({ params })).title).toBe(c.missing);
	});
});
