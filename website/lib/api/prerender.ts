/**
 * NOT dead code — load-bearing during `next build`. 13+ callers depend on it;
 * deleting it makes any fetcher's upstream 5xx (e.g. SELLER_NOT_FOUND / 9233)
 * fail the whole export. #339 D3 misclassified this as dead code — do not delete.
 *
 * Prerender-safe error handler for `unstable_cache` data fetches called during
 * render. Replaces the scattered
 *   .catch(err => { if (process.env.NODE_ENV === "production") throw err; return fallback })
 * pattern, which re-threw during `next build` (NODE_ENV is "production" in BOTH
 * build and runtime) and so let a single upstream 5xx kill the whole export.
 *
 * Policy:
 *  - build phase (`next build` prerender): soft-fail to `fallback` — a transient
 *    backend blip must never be FATAL to the build. Sentinel/cold pages render
 *    a shell; ISR fills real data at runtime.
 *  - definitive 4xx (ApiClientError, status < 500 AND not in RETRY_LATER below —
 *    malformed/invalid id, bad request): soft-fail always, including runtime
 *    production. It will never succeed on retry, client.ts's own
 *    reportApiFailure already excludes 4xx from Sentry (status < 500 → no-op),
 *    and generateMetadata has no error boundary to "reach" — rethrowing there
 *    only turns it into an unhandled exception instead of falling through to
 *    its own `if (!s) return { title: "找不到賣家" }` fallback.
 *  - runtime production (remaining cases — 5xx / network / RETRY_LATER):
 *    rethrow. With a warm cache Next serves stale (stale-while-revalidate); on
 *    cold cache the error reaches the route error boundary.
 *  - dev: soft-fail for DX (unchanged from the old pattern).
 *
 * `NEXT_PHASE === "phase-production-build"` (next/constants PHASE_PRODUCTION_BUILD)
 * is set by Next only during `next build`, not `next start`.
 */
import { ApiClientError } from "./client";

/**
 * 4xx statuses that mean "not now", never "not ever".
 *
 * The soft-fail branch above is justified entirely by "it will never succeed on
 * retry" — so a status whose whole meaning is *retry later* does not belong in
 * it. Left in, it turns a momentary condition into a durable lie, and ISR then
 * stores that lie:
 *
 *   gateway 429 (the anonymous public read limit is 120 req / 60 s per IP —
 *   backend gateway/internal/config/config.go RATE_LIMIT_*)
 *     → ApiClientError(429) from client.ts (softFailNotFound only absorbs 404)
 *     → this handler returns null
 *     → app/[locale]/(public)/sellers/[id]/(storefront)/layout.tsx: `if (!seller) notFound()`
 *     → Next caches that 404 render as the route's ISR entry, and the route
 *       declares `revalidate = 1800`, so a *live* storefront serves
 *       "找不到賣家" to every visitor for the next 30 minutes and Google sees a
 *       404 on a canonical URL. One unlucky request, half an hour of damage.
 *
 * Rethrowing instead puts them on the 5xx path, which is what they behave like:
 * a warm cache keeps serving stale, and a failed render is NOT committed to the
 * cache, so the next request simply tries again.
 *
 * 408 / 425 are here for the same reason, not because they have been observed:
 * both are defined as retryable, so classifying them as definitive would be
 * wrong for the identical reason 429 was.
 */
const RETRY_LATER = new Set([408, 425, 429]);

export function prerenderSafe<T>(fallback: T): (err: unknown) => T {
	return (err: unknown): T => {
		if (process.env.NEXT_PHASE === "phase-production-build") {
			console.warn("[prerenderSafe] soft-fail during build:", err);
			return fallback;
		}
		if (
			err instanceof ApiClientError &&
			err.status < 500 &&
			!RETRY_LATER.has(err.status)
		) {
			return fallback;
		}
		if (process.env.NODE_ENV !== "production") return fallback;
		throw err;
	};
}
