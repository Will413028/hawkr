/**
 * Per-postId pub-sub for comment lifecycle events that need to reach
 * nested `<CommentItem>` instances (which hold their own reply state
 * via `useCommentReplies` + a local `pendingReplies` mirror).
 *
 * `usePostComments` already owns the top-level comments array, so
 * top-level `comment.created` / `comment.updated` / `comment.deleted`
 * / `comment.like_changed` events are handled in-hook without needing
 * the bus. The bus delivers ONE event today — `reply_arrived` — used
 * to push freshly-created replies into the right parent's thread so
 * "B sees A's reply in real time" works without a manual collapse +
 * re-expand.
 *
 * Why a module-level EventTarget and not a React context:
 *   - The publisher is `usePostComments` (one per post). The
 *     subscribers are deeply nested CommentItems, one per top-level
 *     comment. Wiring through context would require a context per
 *     post and explicit pass-through; the EventTarget keeps both sides
 *     loosely coupled and lazy.
 *   - The bus is scoped by postId so subscribers from other posts in
 *     the same SPA session don't see events they'd ignore anyway —
 *     keeps the per-dispatch cost O(subscribers-on-this-post).
 *
 * The future-compat shape is a discriminated union so adding
 * `reply_deleted` / `reply_updated` / `reply_like_changed` later
 * doesn't break existing subscribers — they just `switch` on `kind`.
 */

import type { CommentSnapshot } from "./useCommentStream";

export type CommentBusEvent = {
	kind: "reply_arrived";
	postId: string;
	parentCommentId: string;
	reply: CommentSnapshot;
	clientRequestId?: string;
};

const target = new EventTarget();

const channelName = (postId: string) => `comment-bus:${postId}`;

/** Publish a comment lifecycle event for one post. No-op if the bus has
 *  no subscribers for that post (cheap dispatch). */
export function publishCommentEvent(
	postId: string | number,
	event: CommentBusEvent,
): void {
	target.dispatchEvent(
		new CustomEvent(channelName(String(postId)), { detail: event }),
	);
}

/** Subscribe to comment lifecycle events for one post. Returns an
 *  unsubscribe function. `postId` of `undefined` returns a no-op
 *  unsub so callers don't have to branch — handy inside React effects
 *  before the postId resolves. */
export function subscribeCommentEvents(
	postId: string | number | undefined,
	handler: (event: CommentBusEvent) => void,
): () => void {
	if (postId == null || postId === "") return () => {};
	const name = channelName(String(postId));
	const listener = (e: Event) => {
		const detail = (e as CustomEvent<CommentBusEvent>).detail;
		if (detail) handler(detail);
	};
	target.addEventListener(name, listener);
	return () => target.removeEventListener(name, listener);
}
