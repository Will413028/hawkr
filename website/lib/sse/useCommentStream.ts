"use client";

/**
 * Realtime comment stream hook — subscribes to `/public/posts/{id}/comments/stream`
 * and surfaces typed payloads to the caller.
 *
 * Design book §3 (event schema) / §13 (dedup pseudocode):
 *   - The SSE wire format carries JSON envelopes per event; we normalise
 *     them into TS-friendly snapshots here so callers don't have to
 *     defensively `Number(...) / String(...)` every field.
 *   - `comment.batch` is unpacked into per-event dispatches against the
 *     same handlers so the dedup reducer downstream stays a single code
 *     path.
 *   - Last-Event-ID is cached per post id so swiping back into a recently
 *     viewed post resumes from the most-recent event (5 min TTL — see
 *     `lastEventIdCache.ts`).
 *
 * The hook owns SSE connect lifecycle: `enabled=false` (e.g. the post is
 * mounted but isn't the active page in the snap pager) keeps the stream
 * closed so we don't burn one connection per pre-rendered post.
 *
 * The handlers ref pattern keeps `useCommentStream` insensitive to inline
 * callbacks — the SSE client is only re-opened when postId / enabled
 * actually change.
 */

import * as React from "react";
import { SseClient, type SseConnectionStatus } from "./client";
import { getLastEventId, setLastEventId } from "./lastEventIdCache";
import { asNum, asStr, unpackBatch } from "./utils";

export interface CommentSnapshot {
	/** Server emits int64; we keep it as string to avoid JS Number precision loss for full-width TSIDs. */
	commentId: string;
	postId: string;
	/** 留言者是不是這條連線的買家或店家身分（伺服器依連線者算）。 */
	isMine: boolean;
	userName: string;
	/** Bare objectKey. The SSE channel does NOT carry a composed URL (49 §6.7
	 * S4 — it does not pass through the Gateway), so renderers must go through
	 * buildImageUrl. */
	avatarKey: string;
	role: number;
	content: string;
	parentId: string;
	replyToId: string;
	replyToName: string;
	likeCount: number;
	replyCount: number;
	/** ISO 8601 UTC, e.g. 2026-06-09T10:30:00Z. */
	createdAt: string;
	updatedAt: string;
}

export interface CommentCreatedPayload {
	comment: CommentSnapshot;
	clientRequestId?: string;
	eventId: string;
}

export interface CommentUpdatedPayload {
	commentId: string;
	postId: string;
	content: string;
	updatedAt: string;
	eventId: string;
}

export interface CommentDeletedPayload {
	commentId: string;
	postId: string;
	parentCommentId: string;
	eventId: string;
}

export interface CommentLikeChangedPayload {
	commentId: string;
	postId: string;
	likeCount: number;
	action: "LIKE" | "UNLIKE";
	/** 按讚／取消的是不是這條連線的買家或店家身分。 */
	likedByViewer: boolean;
	eventId: string;
}

export interface UseCommentStreamHandlers {
	onCreated: (payload: CommentCreatedPayload) => void;
	onReplied: (payload: CommentCreatedPayload) => void;
	onUpdated: (payload: CommentUpdatedPayload) => void;
	onDeleted: (payload: CommentDeletedPayload) => void;
	onLikeChanged: (payload: CommentLikeChangedPayload) => void;
	/** Fired when server `ready` carried bufferMiss=true — caller should refresh REST list. */
	onBufferMiss?: () => void;
}

export interface UseCommentStreamOptions {
	postId: number | string | undefined;
	enabled: boolean;
	handlers: UseCommentStreamHandlers;
}

export interface UseCommentStreamResult {
	status: SseConnectionStatus;
}

const cacheKey = (postId: string | number) => `comments:${postId}`;

/** Exported so the SSE wire contract can be pinned the same way every REST
 * normalizer in this repo is (see tests/media-objectkey-contract.test.ts). */
export function normLikeChanged(
	raw: unknown,
	eventId: string,
): CommentLikeChangedPayload {
	const o = (raw ?? {}) as Record<string, unknown>;
	const action = asStr(o.action, "LIKE").toUpperCase() as "LIKE" | "UNLIKE";
	return {
		commentId: asStr(o.commentId),
		postId: asStr(o.postId),
		likeCount: asNum(o.likeCount),
		action,
		likedByViewer: o.likedByViewer === true,
		eventId,
	};
}

export function normSnapshot(raw: unknown): CommentSnapshot {
	const o = (raw ?? {}) as Record<string, unknown>;
	return {
		commentId: asStr(o.commentId),
		postId: asStr(o.postId),
		isMine: o.isMine === true,
		userName: asStr(o.userName),
		avatarKey: asStr(o.avatarKey),
		role: asNum(o.role),
		content: asStr(o.content),
		parentId: asStr(o.parentId, "0"),
		replyToId: asStr(o.replyToId, "0"),
		replyToName: asStr(o.replyToName),
		likeCount: asNum(o.likeCount),
		replyCount: asNum(o.replyCount),
		createdAt: asStr(o.createdAt),
		updatedAt: asStr(o.updatedAt),
	};
}

export function useCommentStream({
	postId,
	enabled,
	handlers,
}: UseCommentStreamOptions): UseCommentStreamResult {
	const [status, setStatus] = React.useState<SseConnectionStatus>("idle");
	const handlersRef = React.useRef(handlers);
	handlersRef.current = handlers;

	React.useEffect(() => {
		if (!enabled) return;
		if (postId == null || postId === "") return;
		const key = cacheKey(postId);

		const dispatch = (
			eventName: string,
			data: unknown,
			id: string | null,
		): void => {
			switch (eventName) {
				case "comment.batch": {
					for (const inner of unpackBatch(data)) {
						dispatch(inner.event, inner.data, inner.id);
					}
					break;
				}
				case "comment.created": {
					const payload = (data ?? {}) as {
						comment?: unknown;
						clientRequestId?: string;
					};
					handlersRef.current.onCreated({
						comment: normSnapshot(payload.comment),
						clientRequestId: payload.clientRequestId,
						eventId: id ?? "",
					});
					break;
				}
				case "comment.replied": {
					const payload = (data ?? {}) as {
						comment?: unknown;
						clientRequestId?: string;
					};
					handlersRef.current.onReplied({
						comment: normSnapshot(payload.comment),
						clientRequestId: payload.clientRequestId,
						eventId: id ?? "",
					});
					break;
				}
				case "comment.updated": {
					const o = (data ?? {}) as Record<string, unknown>;
					handlersRef.current.onUpdated({
						commentId: asStr(o.commentId),
						postId: asStr(o.postId),
						content: asStr(o.content),
						updatedAt: asStr(o.updatedAt),
						eventId: id ?? "",
					});
					break;
				}
				case "comment.deleted": {
					const o = (data ?? {}) as Record<string, unknown>;
					handlersRef.current.onDeleted({
						commentId: asStr(o.commentId),
						postId: asStr(o.postId),
						parentCommentId: asStr(o.parentCommentId, "0"),
						eventId: id ?? "",
					});
					break;
				}
				case "comment.like_changed": {
					handlersRef.current.onLikeChanged(normLikeChanged(data, id ?? ""));
					break;
				}
				default:
					// Unknown event — leave it for forward-compat.
					break;
			}
			if (id) setLastEventId(key, id);
		};

		const client = new SseClient({
			path: `/posts/${encodeURIComponent(String(postId))}/comments/stream`,
			mode: "public",
			initialLastEventId: getLastEventId(key),
			handlers: {
				onEvent: dispatch,
				onReady: (info) => {
					if (info.bufferMiss) handlersRef.current.onBufferMiss?.();
				},
				onStatusChange: setStatus,
			},
		});

		// Debounce the open: the snap pager flips postId rapidly while the
		// user wheels through posts, and each flip would otherwise open (and
		// immediately tear down) a gateway stream — exactly the burst shape
		// that trips the per-account connection cap (429). Connect only after
		// the pager has settled on a post.
		const openTimer = window.setTimeout(() => client.open(), 600);
		return () => {
			window.clearTimeout(openTimer);
			client.close();
		};
	}, [postId, enabled]);

	return { status };
}
