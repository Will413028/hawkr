import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * Every seller route must self-canonicalize on the seller's store_code (#365).
 *
 * Middleware 308s a store_id URL to the store_code, but the redirect and the
 * canonical tag are two independent signals and the three tabs used to emit
 * neither: none of them set `alternates`, so a crawler that reached a tab URL
 * without following a redirect had nothing to consolidate on.
 *
 * ⚠️ THIS FILE WAS ITSELF THE REASON THE DEFECT SURVIVED (2026-08-06). The
 * cases below have always used a store_id ≠ store_code pair, which is the right
 * fixture — but they FED the seller as `store_code`, the pre-backend#2235 name.
 * The routes read `store_code` too, so mock and route agreed with each other
 * and disagreed with the wire: green suite, and every real request quietly
 * falling through to the id. A mock is a claim about the wire, and this one had
 * stopped being true. The payload below is now the camelCase shape
 * `/public/sellers/{id}` actually returns (verified against dev, 2026-08-06),
 * and a companion case asserts the retired spelling is NOT honoured — without
 * that second case, re-adding a `store_code` fallback would go unnoticed again.
 */

const cachedMocks = vi.hoisted(() => ({ getCachedSeller: vi.fn() }));

vi.mock("@/lib/api/cached", () => ({
	getCachedSeller: cachedMocks.getCachedSeller,
	getCachedSellerReviews: vi.fn(),
	getCachedSellerReviewStats: vi.fn(),
	CACHE_TTL: {
		COMMERCE_LIVE: 300,
		SELLER_PROFILE: 1800,
		DISCOVERY: 900,
		REVIEWS: 900,
		TAXONOMY: 21600,
	},
}));

const STORE_ID = "7487768729113814280";
const STORE_CODE = "2662192214";

const tabs = [
	{
		name: "reviews",
		tail: "/reviews",
		load: () =>
			import("@/features/sellers/reviews/SellerReviewsPage").then(
				(m) => m.generateMetadata,
			),
	},
	{
		name: "posts",
		tail: "/posts",
		load: () =>
			import("@/features/sellers/posts/SellerPostsPage").then(
				(m) => m.generateMetadata,
			),
	},
	{
		name: "messages",
		tail: "/messages",
		load: () =>
			import("@/features/sellers/messages/SellerMessagesPage").then(
				(m) => m.generateMetadata,
			),
	},
	// The storefront index. It was excluded from this file on the grounds that
	// it "already did this" — and it was the first route to break, because
	// "already does this" was never asserted anywhere.
	{
		name: "storefront index",
		tail: "",
		load: () =>
			import("@/features/sellers/storefront/metadata").then(
				(m) => m.generateMetadata,
			),
	},
];

beforeEach(() => {
	cachedMocks.getCachedSeller.mockReset();
});

/**
 * `load()` dynamically imports a Next page module, so the FIRST assertion in
 * each block pays that module graph's transform cost inside the test's own
 * timeout. Under the full parallel suite that regularly exceeds vitest's 5s
 * default and the test times out with nothing actually wrong — reproduced on a
 * clean HEAD worktree, 2026-08-05. Budget for the compile; the assertions
 * themselves are synchronous.
 */
const IMPORT_BUDGET_MS = 30_000;

describe.each(tabs)("$name metadata", ({ tail, load }) => {
	it("canonicalizes on the store_code even when entered by store_id", {
		timeout: IMPORT_BUDGET_MS,
	}, async () => {
		cachedMocks.getCachedSeller.mockResolvedValue({
			// camelCase — `PublicSellerInfo.storeCode` (backend#2235). `fetchSeller`
			// passes it through unrenamed, so this is verbatim what the route sees.
			name: "測試",
			storeCode: STORE_CODE,
			rating: 5,
		});
		const generateMetadata = await load();
		const metadata = await generateMetadata({
			params: Promise.resolve({ locale: "zh-TW", id: STORE_ID }),
		});

		// SELF-canonical: the tab's own URL on the canonical id, not the storefront
		// index. Dropping the suffix would additionally ask search engines to
		// de-index this tab, which is a separate decision from consolidating the
		// two ids.
		expect(metadata.alternates?.canonical).toBe(
			`https://web-dev.example.com/sellers/${STORE_CODE}${tail}`,
		);
		expect(metadata.openGraph?.url).toBe(
			`https://web-dev.example.com/sellers/${STORE_CODE}${tail}`,
		);
	});

	it("falls back to the requested id when the seller has no store_code", {
		timeout: IMPORT_BUDGET_MS,
	}, async () => {
		cachedMocks.getCachedSeller.mockResolvedValue({
			name: "測試",
			rating: 5,
		});
		const generateMetadata = await load();
		const metadata = await generateMetadata({
			params: Promise.resolve({ locale: "zh-TW", id: STORE_ID }),
		});

		expect(metadata.alternates?.canonical).toBe(
			`https://web-dev.example.com/sellers/${STORE_ID}${tail}`,
		);
	});

	it("does not honour the retired store_code spelling", {
		timeout: IMPORT_BUDGET_MS,
	}, async () => {
		// A payload carrying ONLY the pre-backend#2235 name must be treated as
		// carrying no code at all. Without this the fix is unpinnable: a route
		// that reads BOTH names satisfies the first case while still depending on
		// a key the gateway stopped sending.
		cachedMocks.getCachedSeller.mockResolvedValue({
			name: "測試",
			store_code: STORE_CODE,
			rating: 5,
		});
		const generateMetadata = await load();
		const metadata = await generateMetadata({
			params: Promise.resolve({ locale: "zh-TW", id: STORE_ID }),
		});

		expect(metadata.alternates?.canonical).toBe(
			`https://web-dev.example.com/sellers/${STORE_ID}${tail}`,
		);
	});
});

describe.each(tabs)("$name metadata in the en locale", ({ tail, load }) => {
	it("is canonical on the /en URL of the same store_code tab", {
		timeout: IMPORT_BUDGET_MS,
	}, async () => {
		cachedMocks.getCachedSeller.mockResolvedValue({
			name: "測試",
			storeCode: STORE_CODE,
			rating: 5,
		});
		const generateMetadata = await load();
		const metadata = await generateMetadata({
			params: Promise.resolve({ locale: "en", id: STORE_ID }),
		});
		const canonical = String(metadata.alternates?.canonical);
		expect(new URL(canonical).pathname).toBe(
			`/en/sellers/${STORE_CODE}${tail}`,
		);
		expect(metadata.openGraph?.url).toBe(canonical);
	});
});
