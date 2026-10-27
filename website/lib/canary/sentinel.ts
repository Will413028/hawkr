/**
 * Cross-PoP freshness canary — the measured object.
 *
 * `open-next.config.ts` keeps the regional incremental cache at `short-lived`
 * (a hard-coded 60s ceiling) with a comment that refuses to relax it until a
 * canary proves on-demand invalidation actually reaches every PoP under load.
 * This module is that canary's target: a tiny `unstable_cache` entry that goes
 * through the exact same incremental-cache + tag-cache path as every product /
 * seller / discovery fetcher (lib/api/cached.ts), but carries no upstream call,
 * so what it measures is the cache layer itself rather than backend latency.
 *
 * The served payload stamps the moment the entry was regenerated. A prober
 * anywhere on the internet can therefore answer "has the PoP serving me picked
 * up the last purge yet, and how long did that take" with one GET — no shared
 * state, no storage, no privileged access.
 *
 * Deliberately NOT part of `PUBLIC_CACHE_TAGS`: the canary tag must never be
 * accepted by POST /api/revalidate, because that boundary expands entity tags
 * into aggregate tags. A canary write has to invalidate exactly one entry and
 * nothing a real visitor can see (tests/canary-sentinel.test.ts locks this).
 */

/** Sole tag on the sentinel entry; purged by POST /sentinel. */
export const CANARY_TAG = "canary:freshness";

/** Public path of the sentinel route (`app/(canary)/sentinel/route.ts`). */
export const CANARY_PATH = "/sentinel";

/** `unstable_cache` key parts — one shared entry, no per-request variance. */
export const CANARY_CACHE_KEY = ["canary", "sentinel"];

/**
 * TTL safety net only. Freshness here is meant to be driven by the tag purge,
 * so the window is far longer than the probe cycle (5 min): within a cycle any
 * regeneration observed is attributable to the purge, not to expiry. An hour
 * still bounds the damage if the purge path breaks entirely — the canary
 * reports that as a miss long before the TTL hides it.
 */
export const CANARY_TTL_SECONDS = 3_600;

/** Stored in the cache entry — everything here survives until the next purge. */
export type SentinelEntry = {
	/** Epoch ms at which this entry was (re)generated. */
	generatedAt: number;
	/** Build/deploy namespace the entry belongs to (regional keys include it). */
	buildId: string;
};

/** Sent to the prober: the stored entry plus per-request, uncached timing. */
export type SentinelBody = SentinelEntry & {
	tag: string;
	/** Epoch ms at which this response was produced (never cached). */
	servedAt: number;
	/** How stale the served entry is, from the serving PoP's point of view. */
	ageMs: number;
};

export function buildSentinelEntry(
	generatedAt: number,
	buildId: string | undefined,
): SentinelEntry {
	return { generatedAt, buildId: buildId || "unknown" };
}

export function buildSentinelBody(
	entry: SentinelEntry,
	servedAt: number,
): SentinelBody {
	return {
		...entry,
		tag: CANARY_TAG,
		servedAt,
		ageMs: Math.max(0, servedAt - entry.generatedAt),
	};
}

/**
 * `x-robots-tag` is the operative de-indexing signal: robots.txt Disallow stops
 * a crawl but not an index entry sourced from an external link, and a route
 * handler has no <meta robots> to carry. `no-store` keeps the response out of
 * every response-level cache so a probe always measures the entry, not a copy
 * of an earlier probe's response.
 */
export const SENTINEL_RESPONSE_HEADERS = {
	"cache-control": "private, no-store",
	"x-robots-tag": "noindex, nofollow",
	"x-content-type-options": "nosniff",
} as const;
