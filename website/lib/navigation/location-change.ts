/**
 * Soft-navigation location signal.
 *
 * Scheduled on `window` whenever a client-side navigation commits a new URL
 * (Next's router calls `history.pushState` / `replaceState` at route commit —
 * see `components/shared/PageTransition.tsx`, which owns the single history
 * patch and fires this event from it).
 *
 * Client code subscribes to this (plus `popstate` for back/forward) to read
 * `?searchParams` reactively **without** `useSearchParams()`. `useSearchParams`
 * opts a statically-rendered route out of prerendering
 * (BAILOUT_TO_CLIENT_SIDE_RENDERING), dropping the route's server HTML body and
 * killing SEO — the exact regression `/feed` and `/market/products` must avoid
 * while staying reactive to filter/deep-link params. See `useUrlSearchParams`.
 */
export const LOCATION_CHANGE_EVENT = "hk:locationchange";

const queuedLocationChangeTargets = new WeakSet<EventTarget>();

/**
 * Broadcasts the committed URL after the current React commit callstack.
 *
 * Next's App Router writes history from its `useInsertionEffect`. Dispatching
 * our event synchronously from the patched `pushState` / `replaceState` lets a
 * React subscriber call `setState` while that insertion effect is still
 * running, which React explicitly forbids. A microtask keeps the URL commit
 * synchronous while moving subscribers outside the insertion-effect phase.
 * Multiple history writes in one tick collapse to one notification; readers
 * always inspect the final `window.location` when the event arrives.
 */
export function scheduleLocationChangeEvent(target: EventTarget): void {
	if (queuedLocationChangeTargets.has(target)) return;
	queuedLocationChangeTargets.add(target);
	queueMicrotask(() => {
		queuedLocationChangeTargets.delete(target);
		target.dispatchEvent(new Event(LOCATION_CHANGE_EVENT));
	});
}

/**
 * Marks a history entry whose query string is an in-page view state and whose
 * component owns scroll restoration. The pathname value prevents the marker
 * from leaking into a later, real route navigation if a router copies state.
 */
export const SCROLL_MANAGED_HISTORY_PATH_KEY = "__hkScrollManagedPath";

export function isScrollManagedHistoryState(
	state: unknown,
	pathname: string,
): boolean {
	if (!state || typeof state !== "object") return false;
	return (
		(state as Record<string, unknown>)[SCROLL_MANAGED_HISTORY_PATH_KEY] ===
		pathname
	);
}

export function isScrollManagedSamePathHistoryState(
	state: unknown,
	previousPathname: string | null,
	currentPathname: string,
): boolean {
	return (
		previousPathname === currentPathname &&
		isScrollManagedHistoryState(state, currentPathname)
	);
}
