/**
 * Per-topic Last-Event-ID cache used by SSE hooks to resume a stream
 * after the consumer mounts/unmounts within the same SPA session
 * (e.g. swiping back to a previously-viewed post). 5-minute TTL matches
 * the design book §5.3 / §13.1 — long enough that a quick "swipe away,
 * swipe back" within a feed session resumes from the same offset, short
 * enough that stale ids don't trigger a "buffer miss" replay storm.
 *
 * Storage is plain in-memory: surviving a full page refresh is not the
 * goal (the user has fresh REST state at that point); surviving a hook
 * re-mount within the same SPA session is.
 */

interface CachedEntry {
	id: string;
	expireAt: number;
}

const TTL_MS = 5 * 60 * 1000;

const cache = new Map<string, CachedEntry>();

/** Resolve cached Last-Event-ID for a topic; null if absent / expired. */
export function getLastEventId(key: string): string | null {
	const entry = cache.get(key);
	if (!entry) return null;
	if (Date.now() > entry.expireAt) {
		cache.delete(key);
		return null;
	}
	return entry.id;
}

/** Refresh the cached id + bump the TTL window. Empty ids are ignored. */
export function setLastEventId(key: string, id: string | null | undefined) {
	if (!id) return;
	cache.set(key, { id, expireAt: Date.now() + TTL_MS });
}

/** Drop a cached id (e.g. on logout / explicit user action). */
export function clearLastEventId(key: string) {
	cache.delete(key);
}

/** Test-only helper to nuke the entire cache between specs. */
export function __resetLastEventIdCacheForTest() {
	cache.clear();
}
