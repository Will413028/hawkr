"use client";

import { useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import type { RoleSlot } from "@/lib/auth/active-role";
import { useAuth } from "@/lib/auth/context";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { invalidateNotificationLists } from "@/lib/notifications/cache";
import {
	NOTIFICATION_NEW_EVENT,
	NOTIFICATION_STREAM_RESYNC_EVENT,
	SELLER_WORKSPACE_EVENT,
	type SellerWorkspaceEventDetail,
} from "@/lib/notifications/events";
import {
	type UnreadCountState,
	useUnreadCount,
} from "@/lib/notifications/useUnreadCount";
import type { SseConnectionStatus } from "@/lib/sse/client";
import {
	type SellerAfterSaleUpdatedPayload,
	type SellerOrderUpdatedPayload,
	type UseNotificationStreamHandlers,
	useNotificationStream,
} from "@/lib/sse/useNotificationStream";

/**
 * Single source of truth for the navbar bell's unread badge.
 *
 * Why a context (vs. a hook that owns its own state in every consumer):
 * the badge and the notifications list page need to agree instantly when
 * the user marks notifications read. Two independent hooks would each hold
 * their own `count` and only resync when the bell's stream pushed an
 * update — that's the visible "bell takes ~30s to clear after 全部標為
 * 已讀" bug from the polling era. With the count hoisted into a provider,
 * the list page's mutate path can adjust the shared count synchronously
 * (`setCount(0)` / `decrement(1)`) and the bell rerenders the same tick.
 *
 * The count itself is driven by the realtime SSE stream, with the REST
 * `/unread-count` endpoint used as the one-shot baseline on mount /
 * role-switch and as the buffer-miss recovery path. No 30s polling
 * fallback — design book §4.7.
 *
 * The provider owns the tab's ONLY `/app/notifications/stream` connection and
 * dispatches every event type from it: inbox and unread events go to
 * `useUnreadCount`; seller-workspace events and "events may have been missed"
 * (buffer miss, reconnect budget exhausted) are re-published as window events
 * (`SELLER_WORKSPACE_EVENT`, `NOTIFICATION_STREAM_RESYNC_EVENT`) for lazily
 * mounted subscribers. Connection status is read with
 * `useNotificationStreamStatus`.
 */

const NotificationUnreadContext = React.createContext<UnreadCountState | null>(
	null,
);

// Separate from the unread context so a status change does not re-render the
// count's consumers and a count change does not re-render the status's.
const NotificationStreamStatusContext =
	React.createContext<SseConnectionStatus>("idle");

function dispatchSellerWorkspaceEvent(detail: SellerWorkspaceEventDetail) {
	window.dispatchEvent(new CustomEvent(SELLER_WORKSPACE_EVENT, { detail }));
}

function dispatchStreamResync() {
	window.dispatchEvent(new CustomEvent(NOTIFICATION_STREAM_RESYNC_EVENT));
}

export function NotificationUnreadProvider({
	children,
}: {
	children: React.ReactNode;
}) {
	const { activeRole, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const resetSignal = `${identity.accountId ?? "anon"}:${activeRole ?? "none"}`;
	const activeListInvalidationPendingRef = React.useRef(false);
	const invalidateActiveLists = React.useCallback(() => {
		// notification.batch is unpacked into several notification.new DOM events
		// in the same task. Collapse that burst into one active-list refetch instead
		// of repeatedly cancelling and restarting the same infinite query.
		if (activeListInvalidationPendingRef.current) return;
		activeListInvalidationPendingRef.current = true;
		queueMicrotask(() => {
			activeListInvalidationPendingRef.current = false;
			void invalidateNotificationLists(queryClient, identity, "active");
		});
	}, [identity, queryClient]);
	const invalidateOtherWorkspaceLists = React.useCallback(
		(role: RoleSlot) => {
			if (identity.accountId == null) return;
			void invalidateNotificationLists(
				queryClient,
				{ accountId: identity.accountId, activeRole: role },
				"none",
			);
		},
		[identity.accountId, queryClient],
	);
	const streamEnabled =
		isLoggedIn && identity.accountId !== null && activeRole !== null;
	const { unread, handlers, recover } = useUnreadCount({
		enabled: streamEnabled,
		accountId: identity.accountId ?? undefined,
		role: activeRole,
		resetSignal,
		onInboxRecovery: invalidateActiveLists,
		onOtherWorkspaceNotification: invalidateOtherWorkspaceLists,
	});

	const streamHandlers = React.useMemo<UseNotificationStreamHandlers>(
		() => ({
			onNew: handlers.onNew,
			onUnreadChanged: handlers.onUnreadChanged,
			onSellerOrderUpdated: (payload: SellerOrderUpdatedPayload) =>
				dispatchSellerWorkspaceEvent({
					kind: "order",
					orderNo: payload.orderNo,
					status: payload.status,
				}),
			onSellerAfterSaleUpdated: (payload: SellerAfterSaleUpdatedPayload) =>
				dispatchSellerWorkspaceEvent({
					kind: "after_sale",
					orderNo: payload.orderNo,
					afterSaleCaseNo: payload.afterSaleCaseNo,
					status: payload.status,
				}),
			onBufferMiss: () => {
				recover();
				dispatchStreamResync();
			},
		}),
		[handlers, recover],
	);
	const { status: streamStatus } = useNotificationStream({
		enabled: streamEnabled,
		resetSignal,
		handlers: streamHandlers,
	});
	const unavailableRecoveryRef = React.useRef<string | null>(null);
	React.useEffect(() => {
		if (streamStatus !== "unavailable") {
			unavailableRecoveryRef.current = null;
			return;
		}
		if (unavailableRecoveryRef.current === resetSignal) return;
		unavailableRecoveryRef.current = resetSignal;
		// One recovery per terminal stream failure; this is not polling.
		recover();
		dispatchStreamResync();
	}, [resetSignal, recover, streamStatus]);

	React.useEffect(() => {
		if (!isLoggedIn) return;
		const handleNewNotification = () => {
			// notification.new is emitted after the row commits. Refetch only active
			// list variants; inactive roles stay stale until their next mount.
			invalidateActiveLists();
		};
		window.addEventListener(NOTIFICATION_NEW_EVENT, handleNewNotification);
		return () =>
			window.removeEventListener(NOTIFICATION_NEW_EVENT, handleNewNotification);
	}, [invalidateActiveLists, isLoggedIn]);

	return (
		<NotificationUnreadContext.Provider value={unread}>
			<NotificationStreamStatusContext.Provider value={streamStatus}>
				{children}
			</NotificationStreamStatusContext.Provider>
		</NotificationUnreadContext.Provider>
	);
}

/** Status of the tab's single realtime connection ("idle" outside the provider). */
export function useNotificationStreamStatus(): SseConnectionStatus {
	return React.useContext(NotificationStreamStatusContext);
}

/**
 * Read the shared unread count. Outside the provider returns a no-op
 * snapshot (count 0) so server-rendered fallback paths don't crash.
 */
export function useNotificationUnread(): UnreadCountState {
	const ctx = React.useContext(NotificationUnreadContext);
	if (ctx) return ctx;
	return {
		count: 0,
		loading: false,
		refresh: async () => {},
		setCount: () => {},
		decrement: () => {},
		increment: () => {},
	};
}
