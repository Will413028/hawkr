import { timingSafeEqual } from "node:crypto";
import { PUBLIC_CACHE_TAGS } from "@/lib/seo/tags";

export type RevalidateParseResult =
	| {
			ok: true;
			status: 200;
			tags: string[];
			eventId?: string;
	  }
	| {
			ok: false;
			status: 401 | 400;
			error:
				| "unauthorized"
				| "no tags"
				| "too many tags"
				| "invalid tag"
				| "invalid event id";
	  };

/**
 * Two profiles, picked from the tag's shape — never from the caller. A producer
 * knows which record changed; it cannot know which cache entries embed it, so
 * it must not be the one choosing how hard they expire.
 *
 * `{expire: 0}` makes the adapter write the tag row with `expire = now`, and
 * OpenNext's `hasBeenRevalidated` (`expire <= now && expire > lastModified`)
 * then reports every entry generated before the bust as a miss — the next
 * reader renders fresh content. `"max"` instead writes `expire = now + 1y` and
 * only opens a stale window: the next reader is still handed the pre-mutation
 * HTML while the refresh runs behind it.
 *
 * Entity tags key the single-record views a writer looks at straight after
 * saving. Serving those stale shows someone their own edit as if it never
 * happened, which reads as a failed save — so they expire immediately.
 *
 * Aggregate tags key the catalogue, home, market and feed. Expiring those on
 * the spot costs far more than it buys. OpenNext only routes a regeneration
 * through its deduplicating queue on the STALE path (`revalidateIfRequired`
 * returns early unless the response was STALE), so a hard expiry has every
 * concurrent reader render inline instead of one render serving all of them —
 * and because `expandAggregateTags` turns a single edit into `products:list` +
 * `posts:list` + `sellers:list`, one price change would do that across the
 * whole catalogue at once. That is the amplification that pushed the prod
 * Workers past their memory ceiling and blanked the site (#338). Deferring
 * also keeps the stale-if-error fallback that serves the last good page while
 * the backend is degraded. The readers of an aggregate are not the writer, and
 * one request's lag is invisible to them.
 */
export const IMMEDIATE_PROFILE = { expire: 0 } as const;
export const DEFERRED_PROFILE = "max" as const;

export type RevalidationProfile =
	| typeof IMMEDIATE_PROFILE
	| typeof DEFERRED_PROFILE;

export const MAX_REVALIDATION_TAGS = 128;
export const MAX_REVALIDATION_TAG_LENGTH = 256;
export const MAX_REVALIDATION_BODY_BYTES = 64 * 1024;

const STATIC_PUBLIC_TAGS = new Set<string>([
	...Object.values(PUBLIC_CACHE_TAGS),
]);
const ENTITY_TAG =
	/^(?:product|seller|post|category|hashtag):[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/;
const EVENT_ID = /^[\x21-\x7E]{1,128}$/;

// A product's mutable card fields (name/price/image/stock/rating) are embedded
// by the product lists and by the posts feed's linked-product block — nowhere
// else, so nothing else is expanded.
const PRODUCT_AGGREGATE_TAGS = [
	PUBLIC_CACHE_TAGS.products,
	PUBLIC_CACHE_TAGS.posts,
] as const;
// Seller name + logo ride along on every product card (lib/market/listFilters
// mapProduct) and post card, so a seller edit has to reach the product and post
// lists as well as the recommended-seller rail.
const SELLER_AGGREGATE_TAGS = [
	PUBLIC_CACHE_TAGS.sellers,
	PUBLIC_CACHE_TAGS.products,
	PUBLIC_CACHE_TAGS.posts,
] as const;

/** Constant-time string compare. timingSafeEqual requires equal-length
 * buffers, so the length check short-circuits unequal lengths (itself a safe
 * pre-condition, not a secret-dependent branch on content). */
function secretsMatch(a: string, b: string): boolean {
	const ba = Buffer.from(a);
	const bb = Buffer.from(b);
	return ba.length === bb.length && timingSafeEqual(ba, bb);
}

export function isRevalidationSecretAuthorized(
	secretHeader: string | null,
	expectedSecret: string | undefined,
): boolean {
	return Boolean(
		expectedSecret &&
			secretHeader &&
			secretsMatch(secretHeader, expectedSecret),
	);
}

export function isAllowedRevalidationTag(tag: string): boolean {
	return (
		(tag.length <= MAX_REVALIDATION_TAG_LENGTH &&
			STATIC_PUBLIC_TAGS.has(tag)) ||
		ENTITY_TAG.test(tag)
	);
}

/**
 * Producers send entity tags only; they cannot know which aggregate embeds the
 * record. Expand once at the trusted web boundary, strictly to the data domains
 * that actually embed the entity — an aggregate that does not carry the entity's
 * fields is deliberately left cached. Explicit caller tags stay first for stable
 * observability.
 */
function expandAggregateTags(tags: string[]): string[] {
	const expanded = [...tags];
	const seen = new Set(tags);
	const append = (values: readonly string[]) => {
		for (const value of values) {
			if (seen.has(value)) continue;
			seen.add(value);
			expanded.push(value);
		}
	};
	for (const tag of tags) {
		if (tag.startsWith("product:")) {
			append(PRODUCT_AGGREGATE_TAGS);
		} else if (tag.startsWith("seller:")) {
			append(SELLER_AGGREGATE_TAGS);
		} else if (tag.startsWith("post:")) {
			append([PUBLIC_CACHE_TAGS.posts]);
		} else if (tag.startsWith("hashtag:")) {
			append([PUBLIC_CACHE_TAGS.hashtags]);
		} else if (tag.startsWith("category:")) {
			append([PUBLIC_CACHE_TAGS.categories]);
		}
	}
	return expanded;
}

/**
 * Pure auth + tag-validation for POST /api/revalidate. Kept out of the route
 * handler so it is unit-testable without the Next runtime. Fails closed: a
 * missing configured secret is treated as unauthorized, never as "open".
 */
export function parseRevalidateRequest(
	secretHeader: string | null,
	body: unknown,
	expectedSecret: string | undefined,
): RevalidateParseResult {
	if (!isRevalidationSecretAuthorized(secretHeader, expectedSecret)) {
		return { ok: false, status: 401, error: "unauthorized" };
	}
	// A payload may still carry the retired `mode` field; it is read by nobody
	// and must stay a tolerated unknown, because rejecting it would 400 the whole
	// request and drop the valid tags travelling with it — the invalidation would
	// simply vanish.
	const payload = body as {
		tags?: unknown;
		eventId?: unknown;
		event_id?: unknown;
	} | null;
	const raw = payload?.tags;
	if (!Array.isArray(raw) || raw.length === 0) {
		return { ok: false, status: 400, error: "no tags" };
	}
	if (raw.length > MAX_REVALIDATION_TAGS) {
		return { ok: false, status: 400, error: "too many tags" };
	}

	const tags: string[] = [];
	const seen = new Set<string>();
	for (const candidate of raw) {
		if (
			typeof candidate !== "string" ||
			candidate.length === 0 ||
			!isAllowedRevalidationTag(candidate)
		) {
			return { ok: false, status: 400, error: "invalid tag" };
		}
		if (!seen.has(candidate)) {
			seen.add(candidate);
			tags.push(candidate);
		}
	}

	const camelEventId = payload?.eventId;
	const snakeEventId = payload?.event_id;
	if (
		camelEventId !== undefined &&
		snakeEventId !== undefined &&
		camelEventId !== snakeEventId
	) {
		return { ok: false, status: 400, error: "invalid event id" };
	}
	const eventId = camelEventId ?? snakeEventId;
	if (
		eventId !== undefined &&
		(typeof eventId !== "string" || !EVENT_ID.test(eventId))
	) {
		return { ok: false, status: 400, error: "invalid event id" };
	}

	return {
		ok: true,
		status: 200,
		tags: expandAggregateTags(tags),
		...(eventId === undefined ? {} : { eventId }),
	};
}

/**
 * An entity tag names one record, so its readers include the person who just
 * changed it. Anything else is a list, whose readers are strangers to the edit.
 */
export function revalidationProfileFor(tag: string): RevalidationProfile {
	return ENTITY_TAG.test(tag) ? IMMEDIATE_PROFILE : DEFERRED_PROFILE;
}

/**
 * Invalidates every tag under the profile its shape earns. Split out of the
 * route handler because `revalidateTag` needs a request scope: keeping the loop
 * here makes the tag-shape-to-profile mapping assertable in a plain unit test
 * instead of only in a deployed environment.
 */
export function applyRevalidation(
	tags: string[],
	revalidate: (tag: string, profile: RevalidationProfile) => void,
): void {
	for (const tag of tags) {
		revalidate(tag, revalidationProfileFor(tag));
	}
}
