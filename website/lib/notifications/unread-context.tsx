"use client";

import { StateRefreshHint } from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import { useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { invalidateNotificationLists } from "@/lib/notifications/cache";
import {
	NOTIFICATION_SIGNAL_EVENT,
	type NotificationSignalEventDetail,
	SELLER_WORKSPACE_SIGNAL_EVENT,
	type SellerWorkspaceSignalEventDetail,
} from "@/lib/notifications/events";
import { audienceForRole } from "@/lib/notifications/inbox";
import {
	type UnreadCountState,
	useUnreadCount,
} from "@/lib/notifications/useUnreadCount";
import {
	type NotificationSignal,
	watchNotifications,
} from "@/lib/notifications/watch";
import type { SseConnectionStatus } from "@/lib/sse/client";

/**
 * Single source of truth for the navbar bell's unread badge.
 *
 * Why a context (vs. a hook that owns its own state in every consumer):
 * the badge and the notifications list page need to agree instantly when
 * the user marks notifications read. With the count hoisted into a provider,
 * the list page's mutate path can adjust the shared count synchronously
 * (`setCount(0)` / `decrement(1)`) and the bell rerenders the same tick.
 *
 * The provider owns the tab's `WatchNotifications` stream for the active
 * workspace's audience (`lib/notifications/watch.ts`). The stream only says
 * "the inbox changed" and carries the absolute unread count:
 *
 *   - inbox signal → set the count, refetch the active notification lists and
 *     forward the signal on `NOTIFICATION_SIGNAL_EVENT` (toasts, invoice hooks).
 *   - reconnect after an abnormal end (signals may have been missed) → re-read
 *     the unread baseline and refetch the active lists once.
 *   - the stream giving up (`unavailable`) → one recovery read, not polling.
 *
 * Connection status is read with `useNotificationStreamStatus`. The seller
 * workspace signals are forwarded on `SELLER_WORKSPACE_SIGNAL_EVENT` (no second stream).
 */

const ALL_STATE_REFRESH_HINTS: number[] = [
	StateRefreshHint.ORDER,
	StateRefreshHint.AFTER_SALE,
	StateRefreshHint.SELLER_STATUS,
];

function dispatchSellerWorkspace(
	kind: SellerWorkspaceSignalEventDetail["kind"],
) {
	const detail: SellerWorkspaceSignalEventDetail = { kind };
	window.dispatchEvent(
		new CustomEvent(SELLER_WORKSPACE_SIGNAL_EVENT, { detail }),
	);
}

const NotificationUnreadContext = React.createContext<UnreadCountState | null>(
	null,
);

// Separate from the unread context so a status change does not re-render the
// count's consumers and a count change does not re-render the status's.
const NotificationStreamStatusContext =
	React.createContext<SseConnectionStatus>("idle");

export function NotificationUnreadProvider({
	children,
}: {
	children: React.ReactNode;
}) {
	const { activeRole, status } = useAuth();
	const signedIn = status === "signedIn";
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const resetSignal = `${identity.accountId ?? "anon"}:${activeRole ?? "none"}`;
	const streamEnabled =
		signedIn && identity.accountId !== null && activeRole !== null;
	const audience = audienceForRole(activeRole ?? "buyer");
	const unread = useUnreadCount({
		enabled: streamEnabled,
		audience,
		resetSignal,
	});
	const unreadRef = React.useRef(unread);
	unreadRef.current = unread;

	const activeListInvalidationPendingRef = React.useRef(false);
	const invalidateActiveLists = React.useCallback(() => {
		// The first frame after a reconnect arrives as a reconnect signal and an
		// inbox signal in the same task; collapse them into one active-list refetch
		// instead of repeatedly cancelling and restarting the same infinite query.
		if (activeListInvalidationPendingRef.current) return;
		activeListInvalidationPendingRef.current = true;
		queueMicrotask(() => {
			activeListInvalidationPendingRef.current = false;
			void invalidateNotificationLists(queryClient, identity, "active");
		});
	}, [identity, queryClient]);

	// 串流 effect 不依賴它：身分物件換了參照不該重連。
	const invalidateRef = React.useRef(invalidateActiveLists);
	invalidateRef.current = invalidateActiveLists;

	const [streamStatus, setStreamStatus] =
		React.useState<SseConnectionStatus>("idle");
	React.useEffect(() => {
		void resetSignal;
		if (!streamEnabled) {
			setStreamStatus("idle");
			return;
		}
		const handle = watchNotifications(
			audience,
			(signal: NotificationSignal) => {
				if (signal.type === "inbox") {
					unreadRef.current.setCount(signal.unreadCount);
					invalidateRef.current();
					const detail: NotificationSignalEventDetail = {
						unreadCount: signal.unreadCount,
						latestInboxItemId: signal.latestInboxItemId,
						newInboxItemIds: signal.newInboxItemIds,
						newItemsTruncated: signal.newItemsTruncated,
						hints: signal.hints,
					};
					window.dispatchEvent(
						new CustomEvent(NOTIFICATION_SIGNAL_EVENT, { detail }),
					);
				} else if (signal.type === "sellerWorkspace") {
					dispatchSellerWorkspace("change");
				} else if (signal.type === "reconnected" && signal.afterAbnormalEnd) {
					void unreadRef.current.refresh();
					invalidateRef.current();
					// Signals during the gap may be lost: tell every state-refresh
					// subscriber to re-read, as the backend does on reconnect.
					const detail: NotificationSignalEventDetail = {
						hints: ALL_STATE_REFRESH_HINTS,
					};
					window.dispatchEvent(
						new CustomEvent(NOTIFICATION_SIGNAL_EVENT, { detail }),
					);
					dispatchSellerWorkspace("reconnected");
				}
			},
			setStreamStatus,
		);
		const resume = () => {
			if (document.hidden) return;
			handle.resume();
		};
		window.addEventListener("focus", resume);
		window.addEventListener("online", resume);
		document.addEventListener("visibilitychange", resume);
		return () => {
			window.removeEventListener("focus", resume);
			window.removeEventListener("online", resume);
			document.removeEventListener("visibilitychange", resume);
			handle.stop();
		};
	}, [audience, resetSignal, streamEnabled]);

	const unavailableRecoveryRef = React.useRef<string | null>(null);
	React.useEffect(() => {
		if (streamStatus !== "unavailable") {
			unavailableRecoveryRef.current = null;
			return;
		}
		if (unavailableRecoveryRef.current === resetSignal) return;
		unavailableRecoveryRef.current = resetSignal;
		// One recovery per terminal stream failure; this is not polling.
		void unreadRef.current.refresh();
		invalidateActiveLists();
	}, [invalidateActiveLists, resetSignal, streamStatus]);

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
		baselineReady: false,
		refresh: async () => {},
		setCount: () => {},
		decrement: () => {},
		increment: () => {},
	};
}
