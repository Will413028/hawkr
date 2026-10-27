import type { PostListItem } from "@/lib/api/openapi-types";
import { LOCAL_PRIVATE_KEYS } from "@/lib/auth/local-private-keys";

export type FeedListSnapshotTab = "recommend" | "follow" | "nearby";
export type FeedListSnapshotFilterMode = "all" | "followed";

export interface FeedListSnapshot {
	/** pathname + search of the feed page this snapshot belongs to. */
	key: string;
	posts: PostListItem[];
	page: number;
	done: boolean;
	scrollY: number;
	activeTab: FeedListSnapshotTab;
	filterMode: FeedListSnapshotFilterMode;
	/** Epoch ms stamped by `saveFeedListSnapshot`. */
	savedAt?: number;
}

const STORAGE_KEY = LOCAL_PRIVATE_KEYS.feedListSnapshot.key;
const RESTORE_WINDOW_MS = 5 * 60 * 1000;

function readStoredFeedListSnapshot(key: string): FeedListSnapshot | null {
	try {
		const raw = window.sessionStorage.getItem(STORAGE_KEY);
		if (!raw) return null;
		const snapshot = JSON.parse(raw) as FeedListSnapshot;
		if (snapshot.key !== key) return null;
		if (Date.now() - (snapshot.savedAt ?? 0) > RESTORE_WINDOW_MS) return null;
		if (!Array.isArray(snapshot.posts) || snapshot.posts.length === 0) {
			return null;
		}
		if (!["recommend", "follow", "nearby"].includes(snapshot.activeTab)) {
			return null;
		}
		if (!["all", "followed"].includes(snapshot.filterMode)) return null;
		return snapshot;
	} catch {
		return null;
	}
}

export function saveFeedListSnapshot(snapshot: FeedListSnapshot): void {
	try {
		window.sessionStorage.setItem(
			STORAGE_KEY,
			JSON.stringify({ ...snapshot, savedAt: Date.now() }),
		);
	} catch {
		// Quota / privacy mode: feed back-restore degrades to a fresh first page.
	}
}

/**
 * Read without consuming. Client-only navigation uses this during its initial
 * render, then consumes the entry after commit. Keeping the read non-mutating
 * makes it safe under React Strict Mode and concurrent render retries.
 */
export function peekFeedListSnapshot(key: string): FeedListSnapshot | null {
	return readStoredFeedListSnapshot(key);
}

/**
 * Browser state may seed a component that mounts after hydration, but it must
 * never change the first client tree that hydrates server HTML.
 */
export function feedListSnapshotForInitialRender(
	hydrationSafeClientRender: boolean,
	readSnapshot: () => FeedListSnapshot | null,
): FeedListSnapshot | null {
	return hydrationSafeClientRender ? readSnapshot() : null;
}

/**
 * One-shot restore for feed list navigation. Matching is intentionally scoped
 * to the exact pathname + search so `/feed`, `/feed?post=...`, and hashtag
 * filtered feeds never restore into each other.
 */
export function takeFeedListSnapshot(key: string): FeedListSnapshot | null {
	const snapshot = readStoredFeedListSnapshot(key);
	try {
		window.sessionStorage.removeItem(STORAGE_KEY);
	} catch {
		// Storage may be blocked. The validated in-memory value is still usable.
	}
	return snapshot;
}
