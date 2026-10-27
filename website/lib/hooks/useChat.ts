"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { ApiClientError } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/context";
import { resolveChatViewerId } from "@/lib/chat/chatIdentity";
import {
	fetchChatHistory,
	fetchConversations,
	markConversationRead,
	sendMessage as sendMessageHttp,
} from "@/lib/chat/repo";
import {
	reconcileOptimistic,
	sortTimeline,
	upsertMessage,
} from "@/lib/chat/timeline";
import type {
	ChatMessage,
	ConnectionState,
	Conversation,
} from "@/lib/chat/types";
import { useChatUnread } from "@/lib/chat/unread-context";
import { chatWorkspaceRole } from "@/lib/chat/workspace";
import { getChatSocket } from "@/lib/chat/ws";
import { usePathname } from "@/lib/intl/navigation";
import { uploadImageFile } from "@/lib/media/upload";
import { queryKeys } from "@/lib/query/keys";
import { storeIdOf } from "@/lib/seller/store";
import { useProfile } from "./useAccount";
import { useQueryIdentity } from "./useQueryIdentity";
import { useSellerSelf } from "./useSellerSelf";

/** IM 的 presence 以數字區分角色（買家 1、賣家 2，im-service 的 wire）；與帳號的角色集合無關。 */
const CHAT_PEER_ROLE = { buyer: 1, seller: 2 } as const;

/**
 * Native `fetch()` raises a generic `TypeError: Failed to fetch` on
 * network errors (offline / CORS / DNS) — that string would surface
 * raw to the buyer if we just took `e.message`. Translate the common
 * network-failure shapes into a friendly Chinese line; otherwise keep
 * the original message (backend-supplied errors usually read fine
 * already).
 */
function friendlyChatError(e: unknown, fallback: string): string {
	const raw = e instanceof Error ? e.message : "";
	if (
		raw === "Failed to fetch" ||
		raw === "Load failed" ||
		raw.toLowerCase().includes("network")
	) {
		return "連線失敗，請再次嘗試";
	}
	return raw || fallback;
}

function friendlyChatErrorOrNull(e: unknown, fallback: string): string | null {
	return e ? friendlyChatError(e, fallback) : null;
}

/** Client-generated nonce for the optimistic message id (clientMsgId). Guarded
 *  because `crypto.randomUUID` is secure-context-only — it's `undefined` on a
 *  plain-http LAN dev origin (phone hitting `http://192.168.x.x`), where a raw
 *  call would throw and silently drop the send. Mirrors the guards in
 *  lib/hooks/useSocial.ts and lib/products/seller-repo.ts. */
function clientMsgId(): string {
	if (
		typeof crypto !== "undefined" &&
		typeof crypto.randomUUID === "function"
	) {
		return crypto.randomUUID();
	}
	return `cmid-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

/* ===== Conversation list ===== */

export function useConversations({
	subscribeToSocket = true,
}: {
	/**
	 * Only the persistent inbox layout owns live conversation-list refreshes.
	 * ThreadClient also reads this query to resolve peer identity, but subscribing
	 * there would register a second socket listener for the same inbound message.
	 */
	subscribeToSocket?: boolean;
} = {}) {
	const { hydrated, isLoggedIn } = useAuth();
	const queryIdentity = useQueryIdentity();
	const queryClient = useQueryClient();
	const queryKey = queryKeys.chat.conversations(queryIdentity);
	const shouldFetch = hydrated && isLoggedIn;
	const query = useQuery({
		queryKey,
		enabled: shouldFetch,
		placeholderData: [] as Conversation[],
		queryFn: () => fetchConversations({ pageSize: 50 }),
	});

	// Soft refresh on inbound message
	React.useEffect(() => {
		if (!shouldFetch || !subscribeToSocket) return;
		const sock = getChatSocket();
		const unsub = sock.onMessage(() => {
			void queryClient.invalidateQueries({ queryKey });
		});
		return unsub;
	}, [shouldFetch, subscribeToSocket, queryClient, queryKey]);

	return {
		items: shouldFetch ? (query.data ?? []) : [],
		loading:
			!hydrated ||
			query.isLoading ||
			(query.isFetching && query.isPlaceholderData),
		error: friendlyChatErrorOrNull(query.error, "載入會話失敗"),
		refresh: async () => {
			await query.refetch();
		},
	};
}

/* ===== Chat thread ===== */

interface UseChatThreadResult {
	messages: ChatMessage[];
	loading: boolean;
	error: string | null;
	connectionState: ConnectionState;
	peerTyping: boolean;
	/** Peer's live presence: `true` online, `false` offline, `null` unknown. */
	peerOnline: boolean | null;
	/** Peer's last-seen timestamp (ms epoch); 0 when never reported. */
	peerLastSeen: number;
	send: (content: string, type?: string) => Promise<void>;
	/** Upload an image file and send it as an `image` message. */
	sendImage: (file: File) => Promise<void>;
	notifyTyping: (typing: boolean) => void;
	refresh: () => Promise<void>;
	/** The viewer's own chat wire id (resolved buyer_id / store_id), for
	 *  "is this message mine" checks. `""` until the profile fetch resolves.
	 *  Exposed so consumers compare against the SAME value this hook filters
	 *  and stamps optimistic sends with, instead of re-deriving it. */
	myId: string;
}

/**
 * Manages a single conversation: lifts WebSocket connection (ref-counted by
 * the singleton), pulls REST history, fans WS messages into local state with
 * dedup, and exposes an async `send()` that posts via HTTP for persistence.
 */
export function useChatThread(
	targetId: string | undefined,
): UseChatThreadResult {
	const pathname = usePathname() || "";
	const localRole = chatWorkspaceRole(pathname);
	const { activeRole, hydrated, isLoggedIn } = useAuth();
	const queryIdentity = useQueryIdentity();
	const { refresh: refreshChatUnread } = useChatUnread();
	const queryClient = useQueryClient();
	// Chat wire ids are CURRENT-stint surrogates (buyer_id / store_id), never the
	// raw session accountId — see lib/chat/chatIdentity.ts. This is the single
	// derivation point: `myId` gates the inbound WS filter below, stamps
	// optimistic sends, and is handed to consumers via the return value.
	const isSeller = activeRole === "seller";
	const { profile: buyerProfile } = useProfile({ enabled: !isSeller });
	const { data: sellerSelf } = useSellerSelf();
	const myId = resolveChatViewerId({
		isSeller,
		storeId: storeIdOf(sellerSelf),
		buyerId: buyerProfile?.id,
	});
	const [connectionState, setConnState] =
		React.useState<ConnectionState>("disconnected");
	const [peerTyping, setPeerTyping] = React.useState(false);
	const [peerOnline, setPeerOnline] = React.useState<boolean | null>(null);
	const [peerLastSeen, setPeerLastSeen] = React.useState(0);
	const [sendError, setSendError] = React.useState<string | null>(null);
	const typingTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);

	// The peer holds the opposite role to the local user: a buyer chats with
	// a seller (role 2) and vice-versa (role 1). Presence subscription is
	// keyed by this role server-side.
	const peerRole =
		localRole === "seller" ? CHAT_PEER_ROLE.buyer : CHAT_PEER_ROLE.seller;
	const threadKey = React.useMemo(
		() => queryKeys.chat.thread(queryIdentity, targetId ?? ""),
		[queryIdentity, targetId],
	);
	const threadQuery = useQuery({
		queryKey: threadKey,
		// Gate on hydration + login so the history fetch never fires under the
		// `anon` identity slot pre-hydration (would cache a spurious 401).
		enabled: Boolean(targetId) && hydrated && isLoggedIn,
		placeholderData: [] as ChatMessage[],
		queryFn: async () => {
			if (!targetId) return [];
			const messages = await fetchChatHistory({ targetId, pageSize: 50 }).catch(
				(error) => {
					if (error instanceof ApiClientError && error.status === 404) {
						return [] as ChatMessage[];
					}
					throw error;
				},
			);
			return sortTimeline(messages);
		},
	});
	const messages = threadQuery.data ?? [];

	// Mark read when the thread is opened. Independent of history loading: a
	// failed history fetch must not leave the unread badge stuck, and the
	// backend mark-read does not depend on the history response.
	React.useEffect(() => {
		if (!targetId) return;
		markConversationRead(targetId).then(refreshChatUnread, () => {});
	}, [targetId, refreshChatUnread]);

	// WebSocket lifecycle: connect on mount, listen, disconnect on unmount.
	React.useEffect(() => {
		if (!targetId) return;
		// Presence is unknown until the server reports it for this peer.
		setPeerOnline(null);
		setPeerLastSeen(0);
		const sock = getChatSocket();
		sock.connect();
		const unsubState = sock.onState(setConnState);
		const unsubPresence = sock.onPresence((ev) => {
			if (String(ev.peerId) !== String(targetId)) return;
			setPeerOnline(ev.online);
			if (ev.lastSeenAtMs) setPeerLastSeen(ev.lastSeenAtMs);
		});
		const unsubMsg = sock.onMessage((m) => {
			// Only inject messages tied to this thread.
			const involved =
				(m.senderId === targetId && m.receiverId === myId) ||
				(m.senderId === myId && m.receiverId === targetId);
			if (!involved) return;
			// Insert by seqId (not append): a sync replay after reconnect can deliver
			// older messages out of order, and the echo of our own send upgrades the
			// optimistic bubble with its authoritative seqId.
			queryClient.setQueryData<ChatMessage[]>(threadKey, (current) =>
				upsertMessage(current, m),
			);
			if (m.senderId === targetId) {
				// Active-thread auto-mark-read: the inbound message lands in the
				// open thread, so mark it read. The backend then pushes an
				// authoritative unread_changed (badge SETs from it); this refresh
				// is a defensive resync in case that event is missed.
				markConversationRead(targetId).then(refreshChatUnread, () => {});
			}
		});
		const unsubTyping = sock.onTyping(({ from, typing }) => {
			if (from !== targetId) return;
			setPeerTyping(typing);
			if (typing) {
				if (typingTimer.current) clearTimeout(typingTimer.current);
				typingTimer.current = setTimeout(() => setPeerTyping(false), 4_000);
			}
		});

		return () => {
			unsubState();
			unsubPresence();
			unsubMsg();
			unsubTyping();
			// Don't disconnect singleton — other tabs/components may share it.
		};
	}, [targetId, myId, refreshChatUnread, queryClient, threadKey]);

	// Presence subscription must ride a live socket — `send()` no-ops until the
	// handshake completes. Re-subscribe whenever the connection (re)establishes
	// so the header recovers after a reconnect, and tell the server to stop
	// pushing this peer's status when we leave the thread.
	React.useEffect(() => {
		if (!targetId || connectionState !== "connected") return;
		const sock = getChatSocket();
		sock.subscribePresence(targetId, peerRole);
		return () => {
			sock.unsubscribePresence(targetId, peerRole);
		};
	}, [targetId, peerRole, connectionState]);

	// After a *re*connect, refetch the thread to pick up anything missed while the
	// socket was offline. At 1:1 chat scale a full REST refetch is simpler and more
	// robust than incremental seq-based gap-fill (and the latest 50 cover any gap).
	// Skips the first connect — mount already loaded fresh history.
	const hasConnectedRef = React.useRef(false);
	React.useEffect(() => {
		if (connectionState !== "connected") return;
		if (!hasConnectedRef.current) {
			hasConnectedRef.current = true;
			return;
		}
		if (!targetId) return;
		void queryClient.invalidateQueries({ queryKey: threadKey });
	}, [connectionState, targetId, threadKey, queryClient]);

	const send = React.useCallback(
		async (content: string, type: string = "text") => {
			if (!targetId || !content.trim()) return;
			const optimistic: ChatMessage = {
				// clientMsgId nonce as the optimistic id — never `tmp_${Date.now()}`
				// (two sends in the same ms would collide). Doubles as the dedup key
				// the future server-side dedup (P2) will key on.
				id: clientMsgId(),
				senderId: myId,
				receiverId: targetId,
				content,
				type,
				timestamp: Date.now(),
				read: false,
			};
			setSendError(null);
			// Cancel in-flight thread refetches so a late history GET can't drop
			// the optimistic message we're about to append.
			await queryClient.cancelQueries({ queryKey: threadKey });
			queryClient.setQueryData<ChatMessage[]>(threadKey, (current) =>
				upsertMessage(current, optimistic),
			);
			try {
				const result = await sendMessageHttp({
					toUserId: targetId,
					content,
					type,
				});
				queryClient.setQueryData<ChatMessage[]>(threadKey, (current) =>
					reconcileOptimistic(current, optimistic.id, {
						id: result.messageId,
						timestamp: result.timestamp,
						seqId: result.seqId,
					}),
				);
			} catch (e) {
				queryClient.setQueryData<ChatMessage[]>(threadKey, (current = []) =>
					current.filter((m) => m.id !== optimistic.id),
				);
				setSendError(friendlyChatError(e, "傳送失敗"));
				throw e;
			}
		},
		[targetId, myId, queryClient, threadKey],
	);

	// Mirrors the APP's chat image flow (chat_room_screen `onImagePick`):
	// upload → show the preview optimistically → POST the staged token as an
	// `image` message → swap the temporary preview URL for the permanent CDN
	// URL the server echoes back (the tmp object is deleted on finalize).
	const sendImage = React.useCallback(
		async (file: File) => {
			if (!targetId) return;
			const { previewUrl, stagedToken } = await uploadImageFile(file);
			const optimistic: ChatMessage = {
				id: clientMsgId(),
				senderId: myId,
				receiverId: targetId,
				content: previewUrl,
				type: "image",
				timestamp: Date.now(),
				read: false,
			};
			setSendError(null);
			// Cancel in-flight thread refetches so a late history GET can't drop
			// the optimistic image we're about to append.
			await queryClient.cancelQueries({ queryKey: threadKey });
			queryClient.setQueryData<ChatMessage[]>(threadKey, (current) =>
				upsertMessage(current, optimistic),
			);
			try {
				const result = await sendMessageHttp({
					toUserId: targetId,
					content: stagedToken,
					type: "image",
				});
				queryClient.setQueryData<ChatMessage[]>(threadKey, (current) =>
					reconcileOptimistic(current, optimistic.id, {
						id: result.messageId,
						timestamp: result.timestamp,
						content: result.content,
						seqId: result.seqId,
					}),
				);
			} catch (e) {
				queryClient.setQueryData<ChatMessage[]>(threadKey, (current = []) =>
					current.filter((m) => m.id !== optimistic.id),
				);
				setSendError(friendlyChatError(e, "圖片傳送失敗"));
				throw e;
			}
		},
		[targetId, myId, queryClient, threadKey],
	);

	const notifyTyping = React.useCallback(
		(typing: boolean) => {
			if (!targetId) return;
			getChatSocket().sendTypingIndicator(targetId, typing);
		},
		[targetId],
	);

	return {
		messages,
		loading: threadQuery.isLoading,
		error:
			sendError ?? friendlyChatErrorOrNull(threadQuery.error, "載入訊息失敗"),
		connectionState,
		peerTyping,
		peerOnline,
		peerLastSeen,
		send,
		sendImage,
		notifyTyping,
		refresh: async () => {
			await threadQuery.refetch();
		},
		myId,
	};
}
