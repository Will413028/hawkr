"use client";

/**
 * Unread-count state for the navbar notification bell, fed by the realtime
 * stream that `NotificationUnreadProvider` owns (one connection per tab; this
 * hook opens none). Design book §4.7 mandates no polling fallback:
 *
 *   - mount + resetSignal change → one-shot REST `/unread-count` for the
 *     initial number (the SSE stream's `ready` doesn't itself include the
 *     count; `notification.new` / `notification.unread_changed` do).
 *   - the provider forwards `notification.new` / `notification.unread_changed`
 *     to `handlers`; the server is the source of truth for the absolute number.
 *   - on a buffer miss or a terminal stream failure the provider calls
 *     `recover`, which re-issues the REST count once so the badge isn't stuck
 *     on a stale number (no continuous polling).
 */

import * as React from "react";
import { UnauthorizedError } from "@/lib/api/client";
import type { RoleSlot } from "@/lib/auth/active-role";
import { notificationEventWorkspace } from "@/lib/notifications/event-role";
import {
	NOTIFICATION_NEW_EVENT,
	type NotificationNewEventDetail,
} from "@/lib/notifications/events";
import { fetchUnreadCount } from "@/lib/notifications/repo";
import { createNotificationRequestGeneration } from "@/lib/notifications/request-generation";
import type {
	NotificationNewPayload,
	NotificationUnreadChangedPayload,
} from "@/lib/sse/useNotificationStream";

export interface UnreadCountState {
	count: number;
	loading: boolean;
	refresh: () => Promise<void>;
	setCount: (next: number) => void;
	decrement: (by?: number) => void;
	increment: (by?: number) => void;
}

export interface UnreadCountStream {
	unread: UnreadCountState;
	/** Stream event handlers for the connection owner to wire in. */
	handlers: {
		onNew: (payload: NotificationNewPayload) => void;
		onUnreadChanged: (payload: NotificationUnreadChangedPayload) => void;
	};
	/** Re-read the count (and the active inbox lists) once after a possible gap. */
	recover: () => void;
}

interface UseUnreadCountOptions {
	enabled: boolean;
	/** Current signed-in account; protects a role-preserving account switch. */
	accountId?: string;
	/** Current workspace. The backend SSE topic is account-wide, so item events
	 * must be filtered client-side. REST recovery follows the stored active role. */
	role?: RoleSlot | null;
	/** Restart REST baseline + SSE stream when this signal changes. */
	resetSignal?: unknown;
	/** Recover notification-list variants after a buffer miss or ambiguous event. */
	onInboxRecovery?: () => void;
	/** Mark an inactive buyer/seller inbox stale when its account-wide event arrives. */
	onOtherWorkspaceNotification?: (role: RoleSlot) => void;
}

export function useUnreadCount({
	enabled,
	accountId,
	role,
	resetSignal,
	onInboxRecovery,
	onOtherWorkspaceNotification,
}: UseUnreadCountOptions): UnreadCountStream {
	const [count, setCountState] = React.useState(0);
	const [loading, setLoading] = React.useState(false);
	const requestGenerationRef = React.useRef(
		createNotificationRequestGeneration(),
	);

	const load = React.useCallback(async () => {
		const generation = requestGenerationRef.current.begin();
		if (!enabled) {
			setCountState(0);
			setLoading(false);
			return;
		}
		setLoading(true);
		try {
			const next = await fetchUnreadCount({ role: role ?? undefined });
			if (requestGenerationRef.current.isCurrent(generation)) {
				setCountState(next);
			}
		} catch (err) {
			// Match the polling hook's UX: a 401 means logged out (force 0);
			// other errors keep the last known count so the badge doesn't
			// flap on transient network blips.
			if (
				requestGenerationRef.current.isCurrent(generation) &&
				err instanceof UnauthorizedError
			) {
				setCountState(0);
			}
		} finally {
			if (requestGenerationRef.current.isCurrent(generation)) {
				setLoading(false);
			}
		}
	}, [enabled, role]);
	const inboxRecoveryPendingRef = React.useRef(false);
	const scheduleInboxRecovery = React.useCallback(() => {
		// Replayed batches can contain several ambiguous count/item events. One
		// authoritative REST read is enough for the whole synchronous burst.
		if (inboxRecoveryPendingRef.current) return;
		inboxRecoveryPendingRef.current = true;
		queueMicrotask(() => {
			inboxRecoveryPendingRef.current = false;
			void load();
			onInboxRecovery?.();
		});
	}, [load, onInboxRecovery]);

	React.useEffect(() => {
		void resetSignal;
		setCountState(0);
		void load();
		return () => {
			// Permanently obsolete the previous account/workspace response. A boolean
			// flag can be reset by the next effect before the older request settles.
			requestGenerationRef.current.invalidate();
		};
	}, [load, resetSignal]);

	const handleNew = React.useCallback(
		(payload: NotificationNewPayload) => {
			const workspace = notificationEventWorkspace(
				payload.item.userType,
				role,
				payload.item.accountId,
				accountId,
			);
			if (workspace !== "current") {
				// Missing role metadata is never guessed. Recover the current role from
				// REST; known other-role events only stale that other list cache.
				if (workspace === "unknown") {
					scheduleInboxRecovery();
				} else if (
					payload.item.userType &&
					(payload.item.accountId == null ||
						accountId == null ||
						payload.item.accountId === accountId)
				) {
					onOtherWorkspaceNotification?.(payload.item.userType);
				}
				return;
			}

			requestGenerationRef.current.invalidate();
			setLoading(false);
			setCountState(Math.max(0, payload.unreadCount));
			if (typeof window === "undefined") return;
			const detail: NotificationNewEventDetail = {
				item: payload.item,
				unreadCount: payload.unreadCount,
			};
			window.dispatchEvent(new CustomEvent(NOTIFICATION_NEW_EVENT, { detail }));
		},
		[accountId, onOtherWorkspaceNotification, role, scheduleInboxRecovery],
	);

	const handleUnreadChanged = React.useCallback(
		(payload: NotificationUnreadChangedPayload) => {
			if (role) {
				// This account-wide event has no role metadata. Its absolute count may
				// belong to the other workspace, so re-read the active role instead.
				scheduleInboxRecovery();
				return;
			}
			requestGenerationRef.current.invalidate();
			setLoading(false);
			setCountState(Math.max(0, payload.unreadCount));
		},
		[role, scheduleInboxRecovery],
	);

	const setCount = React.useCallback((next: number) => {
		requestGenerationRef.current.invalidate();
		setLoading(false);
		setCountState(Math.max(0, next));
	}, []);

	const decrement = React.useCallback((by = 1) => {
		requestGenerationRef.current.invalidate();
		setLoading(false);
		setCountState((prev) => Math.max(0, prev - by));
	}, []);

	const increment = React.useCallback((by = 1) => {
		requestGenerationRef.current.invalidate();
		setLoading(false);
		setCountState((prev) => prev + by);
	}, []);

	const handlers = React.useMemo(
		() => ({ onNew: handleNew, onUnreadChanged: handleUnreadChanged }),
		[handleNew, handleUnreadChanged],
	);

	const unread = React.useMemo<UnreadCountState>(
		() => ({
			count,
			loading,
			refresh: load,
			setCount,
			decrement,
			increment,
		}),
		[count, loading, load, setCount, decrement, increment],
	);

	return {
		unread,
		handlers,
		recover: scheduleInboxRecovery,
	};
}
