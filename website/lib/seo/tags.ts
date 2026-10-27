/**
 * Cache-tag string builders — the single source of truth for the tags emitted
 * by the `unstable_cache` entity wrappers (lib/api/cached.ts) AND consumed by the
 * on-demand invalidation route (app/api/revalidate) + the Java write-side that
 * calls it. Keep these in sync with the documented webhook contract.
 */
export function productTag(id: string | number): string {
	return `product:${id}`;
}

export function sellerTag(id: string | number): string {
	return `seller:${id}`;
}

/**
 * Domain-collection tags — the second (and only other) layer of the taxonomy.
 *
 * Entity tags (`product:<id>`, `seller:<id>`, …) invalidate one record; these
 * invalidate "an aggregate that embeds this data domain". A fetcher tags what
 * it *embeds*, never what page it renders on, so a mutation cannot shotgun
 * unrelated landings. Producers never send these directly — the revalidate
 * boundary expands entity → domain (lib/api/revalidate-request.ts).
 *
 * `marketplace` and `tab:*` used to be listed here as inert legacy layers.
 * They are gone: a triple check (git pickaxe across every producer path, a
 * Loki sweep of every real POST to /api/revalidate, and design book 53
 * §4.1 stating the backend never emitted them) found no caller. Keeping a tag
 * the allow-list accepts but no fetcher reads is worse than removing it — the
 * endpoint answers 200 "invalidated" while doing nothing, so a producer that
 * starts emitting one gets silence instead of the 400 that would tell it the
 * tag is meaningless.
 *
 * This comment used to quote "165 real POSTs over seven days" as the size of
 * that Loki sweep. Dropped 2026-08-04 (backend#2192): unsampled Cloudflare
 * analytics for prod put the real write-side invalidation rate at 0-3 POSTs
 * PER DAY (08-01 = 0, 08-02 = 1, 08-03 = 3), an order of magnitude below that
 * figure, so the number was not a number anyone should plan against. The
 * conclusion it supported is unaffected — zero of those POSTs carried
 * `marketplace` or `tab:*`, whatever the total was.
 */
export const PUBLIC_CACHE_TAGS = {
	products: "products:list",
	posts: "posts:list",
	sellers: "sellers:list",
	hashtags: "hashtags:list",
	categories: "categories",
} as const;
