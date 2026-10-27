/**
 * Chat timeline ordering + dedup helpers (pure, no React/network).
 *
 * Messages are ordered by `seqId` — the server's per-room monotonic sequence —
 * not by `timestamp`, so out-of-order delivery (sync replay after a reconnect)
 * settles into the authoritative order. `seqId` is a 64-bit value carried as a
 * decimal string (can exceed JS 2^53), so all comparison goes through BigInt;
 * `Number()` would silently collapse large neighbours into a tie.
 *
 * Optimistic (just-sent, not-yet-persisted) messages carry no seqId; they pin
 * to the bottom in send order until the server echo upgrades them with a seqId
 * (see `upsertMessage`) — the standard "your message settles into place" UX.
 */

import type { ChatMessage } from "./types";

function seqBig(seqId?: string): bigint | null {
	if (!seqId) return null;
	try {
		return BigInt(seqId);
	} catch {
		return null;
	}
}

/**
 * Total order over messages. Persisted (seqId-bearing) messages come first,
 * ordered by seqId; optimistic ones come last, ordered by send time. Grouping
 * by "has seqId" keeps this transitive — a naive "seqId else timestamp"
 * comparator is non-transitive and corrupts the sort.
 */
export function compareMessages(a: ChatMessage, b: ChatMessage): number {
	const sa = seqBig(a.seqId);
	const sb = seqBig(b.seqId);
	if (sa !== null && sb !== null) return sa < sb ? -1 : sa > sb ? 1 : 0;
	if (sa !== null) return -1; // a persisted, b optimistic → a first
	if (sb !== null) return 1; // b persisted → b first
	// both optimistic → send order; `|| 0` guards a NaN timestamp from a
	// malformed frame, which would otherwise make Array.sort non-deterministic.
	return (a.timestamp || 0) - (b.timestamp || 0);
}

export function sortTimeline(messages: ChatMessage[]): ChatMessage[] {
	return [...messages].sort(compareMessages);
}

/**
 * Insert `next` into a sorted timeline. Dedups by id; when the duplicate is the
 * server echo of a message we already hold seqId-less (our own sent message),
 * the echo's seqId is merged in so it sorts into place rather than staying
 * pinned to the bottom. Returns a new, sorted array.
 */
export function upsertMessage(
	current: ChatMessage[] | undefined,
	next: ChatMessage,
): ChatMessage[] {
	const list = current ?? [];
	const i = next.id ? list.findIndex((m) => m.id === next.id) : -1;
	if (i >= 0) {
		const existing = list[i];
		// Already present (e.g. the server echo of our own send). Fill in the
		// authoritative fields the REST response didn't carry — seqId (so it sorts
		// into place) and conversationId (the room id the sync gap-fill keys on) —
		// without clobbering the reconciled content/id. Nothing new ⇒ same ref.
		const seqId = existing.seqId ?? next.seqId;
		const conversationId = existing.conversationId ?? next.conversationId;
		if (
			seqId === existing.seqId &&
			conversationId === existing.conversationId
		) {
			return list;
		}
		const merged = [...list];
		merged[i] = { ...existing, seqId, conversationId };
		return sortTimeline(merged);
	}
	return sortTimeline([...list, next]);
}

/**
 * Reconcile an optimistic send against the REST response. If the WS echo of the
 * same message already inserted the real id (echo-first race), drop the
 * optimistic copy to avoid a duplicate bubble; otherwise promote the optimistic
 * copy to the real id (+ optional content/timestamp patch). Re-sorts.
 */
export function reconcileOptimistic(
	current: ChatMessage[] | undefined,
	optimisticId: string,
	real: { id: string; timestamp: number; content?: string; seqId?: string },
): ChatMessage[] {
	const list = current ?? [];
	if (real.id && list.some((m) => m.id === real.id)) {
		// Echo-first race: the WS echo already inserted the real id. Drop our
		// optimistic copy, but still apply the REST content patch to the survivor —
		// for images that's the permanent CDN url, which the echo frame may not
		// carry. (Text sends have no content patch, so this is a no-op there.)
		return sortTimeline(
			list
				.filter((m) => m.id !== optimisticId)
				.map((m) =>
					m.id === real.id && real.content != null
						? { ...m, content: real.content }
						: m,
				),
		);
	}
	return sortTimeline(
		list.map((m) =>
			m.id === optimisticId
				? {
						...m,
						id: real.id || m.id,
						timestamp: real.timestamp,
						content: real.content ?? m.content,
						// The sender gets no WS self-echo, so this REST seqId is the only
						// thing that lifts the message off the seqId-less bottom.
						seqId: real.seqId ?? m.seqId,
					}
				: m,
		),
	);
}
