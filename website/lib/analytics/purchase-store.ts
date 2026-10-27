/**
 * Carries the just-confirmed orderNos across the PayUni redirect (same-tab
 * sessionStorage survives the cross-origin round-trip) so the success page can
 * fire one purchase per order, and dedups so a refresh of the success page
 * does not re-fire.
 */

const PENDING_KEY = "hawkr:pending_purchase";
const TRACKED_PREFIX = "hawkr:purchase_tracked:";

export function setPendingPurchase(orderNos: string[]): void {
	if (typeof sessionStorage === "undefined" || orderNos.length === 0) return;
	try {
		sessionStorage.setItem(PENDING_KEY, JSON.stringify(orderNos));
	} catch {
		// storage blocked — purchase falls back to the URL order param
	}
}

export function readPendingPurchase(): string[] {
	if (typeof sessionStorage === "undefined") return [];
	try {
		const raw = sessionStorage.getItem(PENDING_KEY);
		const parsed = raw ? (JSON.parse(raw) as unknown) : null;
		return Array.isArray(parsed) ? parsed.map(String) : [];
	} catch {
		return [];
	}
}

export function clearPendingPurchase(): void {
	if (typeof sessionStorage === "undefined") return;
	try {
		sessionStorage.removeItem(PENDING_KEY);
	} catch {
		/* ignore */
	}
}

/** Returns true the first time an orderNo is seen; false on refresh/re-entry. */
export function markPurchaseTracked(orderNo: string): boolean {
	if (typeof sessionStorage === "undefined") return true;
	const key = `${TRACKED_PREFIX}${orderNo}`;
	try {
		if (sessionStorage.getItem(key)) return false;
		sessionStorage.setItem(key, "1");
		return true;
	} catch {
		return true;
	}
}
