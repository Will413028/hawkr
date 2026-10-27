export type MainTabFreshnessKey = "home" | "feed" | "market" | "search";

/**
 * Display data may stay on screen while these windows expire; expiry only
 * starts a background check. Transactional actions still reconcile against
 * the backend (SKU resolution, cart mutation and checkout validation).
 */
export const MAIN_TAB_STALE_MS = {
	home: 60_000,
	feed: 15_000,
	market: 30_000,
	search: 5 * 60_000,
} as const satisfies Record<MainTabFreshnessKey, number>;

export const MAIN_TAB_REFRESH_RETRY_COOLDOWN_MS = 10_000;
/** Rebuild every loaded page occasionally; fast head checks alone cannot
 * remove an unpublished page-2 item or reconcile its price/stock forever. */
export const MAIN_TAB_FULL_RECONCILE_MS = 5 * 60_000;

interface AutomaticRefreshState {
	now: number;
	freshAt: number;
	lastAttemptAt: number;
	refreshing: boolean;
	baseStaleMs: number;
	online: boolean;
	visible: boolean;
	saveData?: boolean;
	effectiveType?: string;
}

export function effectiveMainTabStaleMs(
	baseStaleMs: number,
	effectiveType?: string,
): number {
	return effectiveType?.toLowerCase() === "3g" ? baseStaleMs * 2 : baseStaleMs;
}

/**
 * Pure decision function shared by the activation hook and its fake-timer
 * tests. Save-Data/offline/hidden/2G are hard stops; 3G keeps the feature but
 * halves its polling frequency.
 */
export function shouldStartAutomaticMainTabRefresh({
	now,
	freshAt,
	lastAttemptAt,
	refreshing,
	baseStaleMs,
	online,
	visible,
	saveData,
	effectiveType,
}: AutomaticRefreshState): boolean {
	if (!online || !visible || saveData || refreshing) return false;
	const network = effectiveType?.toLowerCase();
	if (network === "slow-2g" || network === "2g") return false;
	if (
		lastAttemptAt > 0 &&
		now - lastAttemptAt < MAIN_TAB_REFRESH_RETRY_COOLDOWN_MS
	) {
		return false;
	}
	return now - freshAt >= effectiveMainTabStaleMs(baseStaleMs, effectiveType);
}
