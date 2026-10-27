"use client";

import { useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { Drawer } from "@/components/shared/Drawer";
import ChatDrawerThread from "@/features/messages/components/ChatDrawerThread";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { usePathname } from "@/lib/intl/navigation";
import { queryKeys } from "@/lib/query/keys";
import { useChatDrawerStore } from "@/lib/state/chat-drawer-store";

/**
 * App-wide chat drawer. A single instance lives in the root layout (under the
 * auth + query providers) so any "發訊息 / 聯絡買家 / 聊天" button opens the same
 * in-place chat via `openChat(targetId)` instead of jumping to the full-page
 * thread. `ThreadClient` reads the buyer/seller role from the current pathname
 * (`/seller-center` → seller, else buyer), so one drawer serves both sides.
 *
 * The `/account/messages/{id}` and `/seller-center/messages/{id}` routes stay
 * available as deep links (chat notifications, shared URLs).
 */
export default function ChatDrawer() {
	const targetId = useChatDrawerStore((state) => state.targetId);
	const peerName = useChatDrawerStore((state) => state.peerName);
	const peerAvatar = useChatDrawerStore((state) => state.peerAvatar);
	const closeChat = useChatDrawerStore((state) => state.closeChat);
	const queryClient = useQueryClient();
	const queryIdentity = useQueryIdentity();
	const pathname = usePathname() ?? "";

	// A global overlay must not outlive the route that opened it: close when the
	// user follows a link inside the thread (order / product card → its page).
	// Opening the drawer doesn't change the pathname, so this only fires on real
	// navigation. Mirrors AppOverlayHost's navigation dismissal.
	// biome-ignore lint/correctness/useExhaustiveDependencies: pathname is an intentional re-run trigger
	React.useEffect(() => {
		closeChat();
	}, [pathname, closeChat]);

	const handleClose = React.useCallback(() => {
		closeChat();
		// Refresh the conversation list so unread badges + last-message previews
		// catch up after leaving the thread — the seller messages workspace list
		// shares this query key (mirrors the old per-page drawer's refresh).
		void queryClient.invalidateQueries({
			queryKey: queryKeys.chat.conversations(queryIdentity),
		});
	}, [closeChat, queryClient, queryIdentity]);

	// 走全站共用 {@link Drawer}；聊天是全域 store 驅動、關閉時自行 invalidate 對話列表
	// （非「父頁清單編輯」模式），故不用 onMutated，刷新邏輯留在 onClose(handleClose)。
	return (
		<Drawer
			open={targetId !== null}
			onClose={handleClose}
			side="right"
			closeLabel="關閉聊天室"
			className="seller-message-chat-sheet max-w-3xl"
		>
			{targetId ? (
				<ChatDrawerThread
					targetId={targetId}
					peerNameHint={peerName}
					peerAvatarHint={peerAvatar}
				/>
			) : null}
		</Drawer>
	);
}
