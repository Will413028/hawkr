"use client";

import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import { fetchUnreadChatCount } from "@/lib/chat/repo";
import {
	type UnreadPollingResult,
	useChatUnreadStream,
} from "@/lib/chat/useChatUnreadStream";

/**
 * Shared chat-unread badge state.
 *
 * Mirrors `NotificationUnreadProvider`'s pattern: hoist the REST count
 * into a context so that the navbar chat bubble, the seller dashboard
 * card, and any future consumer all read the same number — and the
 * `useChatThread` mutate path (entering a conversation marks it read)
 * can sync the badge instantly.
 *
 * The count is now driven by WS `unread_changed` events (authoritative
 * SET from the backend) with a REST re-baseline on mount and each WS
 * reconnect. The former 30s polling loop (`useChatUnreadPolling`) and
 * the manual client-side `+1` on inbound messages have both been
 * retired: the backend event carries the correct absolute count.
 */

const ChatUnreadContext = React.createContext<UnreadPollingResult | null>(null);

export function ChatUnreadProvider({
	children,
}: {
	children: React.ReactNode;
}) {
	const { activeRole, isLoggedIn } = useAuth();
	const value = useChatUnreadStream({
		fetcher: fetchUnreadChatCount,
		enabled: isLoggedIn,
		resetSignal: activeRole,
		activeRole,
	});

	return (
		<ChatUnreadContext.Provider value={value}>
			{children}
		</ChatUnreadContext.Provider>
	);
}

/**
 * Read the shared chat-unread count. Outside the provider returns a
 * no-op snapshot (count 0) so server-rendered fallback paths don't crash.
 */
export function useChatUnread(): UnreadPollingResult {
	const ctx = React.useContext(ChatUnreadContext);
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
