"use client";

/**
 * Realtime notification stream hook — subscribes to
 * `/app/notifications/stream` and surfaces typed payloads (new inbox item
 * + unread-count deltas) to the caller. Authed mode rides the session cookie
 * + `X-Active-Role` selection — same auth wiring as
 * `apiClient.authed.*` (lib/sse/client.ts).
 *
 * Design book §3 (notification event schema) / §10 (lifecycle) / §10
 * (failure):
 *   - `enabled=false` keeps the SSE closed entirely (signed-out users,
 *     guest paths, role-switching grace periods).
 *   - `resetSignal` (typically `activeRole`) restarts the stream when
 *     the viewer switches workspace so the new identity's notifications
 *     replace the old without retaining the previous role's last id.
 *   - Last-Event-ID is cached per resetSignal so a hook unmount/remount
 *     during the same SPA session resumes from the most recent id.
 *
 * One open() per (resetSignal, enabled=true) cycle. Cleanup closes the
 * underlying connection so React Strict Mode's double-invoke doesn't
 * leave dangling streams.
 */

import * as React from "react";
import { normalizeNotificationStreamItem } from "@/lib/notifications/stream-item";
import type { NotificationItem } from "@/lib/notifications/types";
import { SseClient, type SseConnectionStatus } from "./client";
import { getLastEventId, setLastEventId } from "./lastEventIdCache";
import { asNum, unpackBatch } from "./utils";

export interface NotificationNewPayload {
	item: NotificationItem;
	unreadCount: number;
	eventId: string;
}

export interface NotificationUnreadChangedPayload {
	unreadCount: number;
	eventId: string;
}

export interface UseNotificationStreamHandlers {
	onNew?: (payload: NotificationNewPayload) => void;
	onUnreadChanged?: (payload: NotificationUnreadChangedPayload) => void;
	/** Fired when server `ready` carried bufferMiss=true — caller should re-fetch REST list/count. */
	onBufferMiss?: () => void;
}

export interface UseNotificationStreamOptions {
	enabled: boolean;
	/** Any value whose change should restart the stream (e.g. active role / accountId). */
	resetSignal?: unknown;
	handlers: UseNotificationStreamHandlers;
}

export interface UseNotificationStreamResult {
	status: SseConnectionStatus;
}

const cacheKey = (resetSignal: unknown) =>
	`notifications:${resetSignal == null ? "" : String(resetSignal)}`;

export function useNotificationStream({
	enabled,
	resetSignal,
	handlers,
}: UseNotificationStreamOptions): UseNotificationStreamResult {
	const [status, setStatus] = React.useState<SseConnectionStatus>("idle");
	const handlersRef = React.useRef(handlers);
	handlersRef.current = handlers;

	React.useEffect(() => {
		if (!enabled) return;
		const key = cacheKey(resetSignal);

		const dispatch = (
			eventName: string,
			data: unknown,
			id: string | null,
		): void => {
			switch (eventName) {
				case "notification.batch": {
					for (const inner of unpackBatch(data)) {
						dispatch(inner.event, inner.data, inner.id);
					}
					break;
				}
				case "notification.new": {
					const o = (data ?? {}) as { item?: unknown; unreadCount?: unknown };
					handlersRef.current.onNew?.({
						item: normalizeNotificationStreamItem(o.item),
						unreadCount: asNum(o.unreadCount),
						eventId: id ?? "",
					});
					break;
				}
				case "notification.unread_changed": {
					const o = (data ?? {}) as { unreadCount?: unknown };
					handlersRef.current.onUnreadChanged?.({
						unreadCount: asNum(o.unreadCount),
						eventId: id ?? "",
					});
					break;
				}
				default:
					// Unknown event — forward-compat.
					break;
			}
			if (id) setLastEventId(key, id);
		};

		const client = new SseClient({
			path: "/notifications/stream",
			mode: "authed",
			initialLastEventId: getLastEventId(key),
			handlers: {
				onEvent: dispatch,
				onReady: (info) => {
					if (info.bufferMiss) handlersRef.current.onBufferMiss?.();
				},
				onStatusChange: setStatus,
			},
		});

		client.open();
		return () => {
			client.close();
		};
	}, [enabled, resetSignal]);

	return { status };
}
