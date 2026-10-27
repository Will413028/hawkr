"use client";

import { create } from "zustand";
import { devtools } from "zustand/middleware";

interface ChatDrawerState {
	/** Active chat peer (buyer or seller account id), or null when closed. */
	targetId: string | null;
	/**
	 * Optional header hints for first contact, before any conversation exists.
	 * Needed on the seller → buyer direction, where the public-profile fallback
	 * can't resolve a name (ThreadClient only fetches `/public/sellers/{id}`).
	 */
	peerName?: string;
	peerAvatar?: string;
	openChat: (
		targetId: string | number,
		hints?: { name?: string; avatar?: string },
	) => void;
	closeChat: () => void;
}

/**
 * App-wide chat drawer state. A single `<ChatDrawer>` reads this in the root
 * layout, so any "發訊息 / 聯絡買家 / 聊天" button — seller-center or buyer
 * storefront — opens the same in-place chat via `openChat(targetId)` instead of
 * navigating to the full-page thread.
 */
export const useChatDrawerStore = create<ChatDrawerState>()(
	devtools(
		(set) => ({
			targetId: null,
			peerName: undefined,
			peerAvatar: undefined,
			openChat: (targetId, hints) =>
				set(
					{
						targetId: String(targetId),
						peerName: hints?.name || undefined,
						peerAvatar: hints?.avatar || undefined,
					},
					false,
					"chat-drawer/open",
				),
			closeChat: () =>
				set(
					{ targetId: null, peerName: undefined, peerAvatar: undefined },
					false,
					"chat-drawer/close",
				),
		}),
		{
			name: "chat-drawer-store",
			enabled: process.env.NODE_ENV === "development",
		},
	),
);
