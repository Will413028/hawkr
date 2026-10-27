import { LOCAL_PRIVATE_KEYS } from "@/lib/auth/local-private-keys";
import type { MarketProductItem } from "./types";

/**
 * Back-navigation snapshot for the infinite-scrolled market list.
 *
 * The list is a TanStack `useInfiniteQuery` seeded from SSR page 1 and grown by
 * client-appended pages. A card→detail navigation unmounts it; TanStack keeps
 * the accumulated pages cached for `gcTime`, but the cache can be cold (gc'd or
 * a different key) and TanStack never tracks scroll — so "back" used to land the
 * buyer on a fresh page-1 list at the top, nowhere near the card. On
 * click-through the card saves a snapshot (accumulated items + pagination cursor
 * + scrollY); the remounting list seeds the query from it and re-asserts scroll.
 *
 * Gating is by save recency, NOT by popstate: Next's router remounts the list
 * and reads the snapshot synchronously inside its own popstate handler, before
 * any of our popstate listeners (capture or bubble) reliably run — so a
 * popstate-timestamp gate always read stale and the restore silently never
 * fired. A short recency window is race-free and, with the one-shot read +
 * key match below, scoped to "the buyer just came back to this exact list".
 */

export interface MarketListSnapshot {
	/** pathname + search of the list page the snapshot belongs to. */
	key: string;
	products: MarketProductItem[];
	page: number;
	done: boolean;
	scrollY: number;
	/** Epoch ms the snapshot was written (stamped in `saveMarketListSnapshot`). */
	savedAt?: number;
}

const STORAGE_KEY = LOCAL_PRIVATE_KEYS.marketListSnapshot.key;
/**
 * A remount with a snapshot newer than this counts as a back-navigation. Long
 * enough to read a product detail (reviews, description) and return; the read
 * is one-shot + key-matched so a stale snapshot can't leak into a later visit.
 */
const RESTORE_WINDOW_MS = 5 * 60 * 1000;

function readStoredMarketListSnapshot(key: string): MarketListSnapshot | null {
	try {
		const raw = window.sessionStorage.getItem(STORAGE_KEY);
		if (!raw) return null;
		const snapshot = JSON.parse(raw) as MarketListSnapshot;
		if (snapshot.key !== key) return null;
		if (Date.now() - (snapshot.savedAt ?? 0) > RESTORE_WINDOW_MS) return null;
		if (!Array.isArray(snapshot.products) || snapshot.products.length === 0) {
			return null;
		}
		return snapshot;
	} catch {
		return null;
	}
}

export function saveMarketListSnapshot(snapshot: MarketListSnapshot): void {
	try {
		window.sessionStorage.setItem(
			STORAGE_KEY,
			JSON.stringify({ ...snapshot, savedAt: Date.now() }),
		);
	} catch {
		// Quota / privacy mode — back-restore just degrades to a fresh list.
	}
}

/**
 * Read without consuming. A client-only back-navigation mount can seed its
 * first render synchronously, then remove the entry after commit. Keeping the
 * render-time read non-mutating makes it safe under concurrent render retries.
 */
export function peekMarketListSnapshot(key: string): MarketListSnapshot | null {
	return readStoredMarketListSnapshot(key);
}

/**
 * Browser state may seed a component mounted by client navigation, but it must
 * never change the first client tree that hydrates server HTML.
 */
export function marketListSnapshotForInitialRender(
	hydrationSafeClientRender: boolean,
	readSnapshot: () => MarketListSnapshot | null,
): MarketListSnapshot | null {
	return hydrationSafeClientRender ? readSnapshot() : null;
}

/**
 * One-shot read: returns the snapshot when `key` matches and it was saved
 * recently, clearing it either way so a stale snapshot can never restore into a
 * later, unrelated visit.
 */
export function takeMarketListSnapshot(key: string): MarketListSnapshot | null {
	const snapshot = readStoredMarketListSnapshot(key);
	try {
		window.sessionStorage.removeItem(STORAGE_KEY);
	} catch {
		// Storage may be blocked. The validated in-memory value is still usable.
	}
	return snapshot;
}
