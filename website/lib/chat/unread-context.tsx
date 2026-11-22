"use client";

import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import { fetchUnreadCount } from "@/lib/chat/repo";
import {
	type UnreadPollingResult,
	useChatUnreadStream,
} from "@/lib/chat/useChatUnreadStream";
import { chatSideOf } from "@/lib/chat/workspace";

/**
 * 共用的聊天未讀數：全站的導覽徽章、賣家儀表板與聊天室（進入對話即已讀）讀同一個數字。
 * 基準由 `GetUnreadCount`／`GetStoreUnreadCount` 讀，之後由聊天串流的絕對值覆寫（見 `useChatUnreadStream`）。
 */

const ChatUnreadContext = React.createContext<UnreadPollingResult | null>(null);

export function ChatUnreadProvider({
	children,
}: {
	children: React.ReactNode;
}) {
	const { activeRole, status } = useAuth();
	const side = chatSideOf(activeRole);
	const fetcher = React.useCallback(() => fetchUnreadCount(side), [side]);
	const value = useChatUnreadStream({
		fetcher,
		enabled: status === "signedIn",
		resetSignal: activeRole,
		side,
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
