/**
 * Client helpers to update `?searchParams` on a statically-rendered (ISR) route
 * without `useSearchParams` / the Next router's server round trip.
 *
 * `history.pushState` is patched by `components/shared/PageTransition.tsx`, which
 * fires `LOCATION_CHANGE_EVENT` after committing the new URL — so a call here
 * both updates the address bar and wakes every `useUrlSearchParams` subscriber
 * to re-read and re-filter in place. This keeps the market filters reactive
 * while the route stays static (no SEO body loss, no per-request render).
 */

/**
 * Merge `updates` into the current URL query (a `null` value removes the key),
 * preserving every other param, and push/replace it. `pathname` is kept as-is.
 */
export function updateUrlParams(
	updates: Record<string, string | null | undefined>,
	options?: { replace?: boolean },
): void {
	if (typeof window === "undefined") return;
	const params = new URLSearchParams(window.location.search);
	for (const [key, value] of Object.entries(updates)) {
		if (value == null || value === "") params.delete(key);
		else params.set(key, value);
	}
	const qs = params.toString();
	const url = qs
		? `${window.location.pathname}?${qs}`
		: window.location.pathname;
	if (options?.replace) window.history.replaceState(null, "", url);
	else window.history.pushState(null, "", url);
}
