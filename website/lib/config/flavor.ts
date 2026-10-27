/**
 * Flavor — environment selector for the Next.js web app.
 *
 * Mirrors the Flutter `Flavor` enum at
 * `hawkr_frontend/hawkr_seller/lib/config/flavor.dart` so dev / prod
 * stay aligned across mobile + web.
 *
 * Resolution
 * ──────────
 * `currentFlavor()` reads `NEXT_PUBLIC_FLAVOR` (must be one of "dev" | "prod").
 * Default is `"dev"` so accidental missing config never silently resolves to
 * production.
 *
 * The deployment injects `NEXT_PUBLIC_FLAVOR` at build time; local dev reads
 * it from `.env.local`. Hosts and origins are not tied to the flavor: they come
 * from `API_BASE_URL`, `NEXT_PUBLIC_WEB_ORIGIN` and friends (lib/config/env.ts).
 */

export type Flavor = "dev" | "prod";

const FALLBACK: Flavor = "dev";

function readFlavor(): Flavor {
	const raw = process.env.NEXT_PUBLIC_FLAVOR?.trim().toLowerCase();
	if (raw === "dev" || raw === "prod") return raw;
	return FALLBACK;
}

let _cached: Flavor | null = null;

/**
 * Resolves the active flavor. Cached after first read so flavor lookups are
 * effectively free.
 */
export function currentFlavor(): Flavor {
	if (_cached !== null) return _cached;
	_cached = readFlavor();
	return _cached;
}

/** Convenience predicate. */
export const isDevFlavor = () => currentFlavor() === "dev";

/** Test-only override. Avoid in app code. */
export function __setFlavorForTest(flavor: Flavor | null) {
	_cached = flavor;
}
