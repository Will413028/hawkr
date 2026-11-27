"use client";

import { errorText } from "@hawkr/api/connect/errors";
import { skipToken, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { accountQueryFn } from "@/lib/auth/account-query";
import { useAccountStatus } from "@/lib/auth/use-account-status";
import {
	fetchPresence,
	listConversations,
	listLatestMessages,
	listMessagesAfter,
	markConversationRead,
	sendMessage as sendMessageRpc,
} from "@/lib/chat/repo";
import { newRequestId } from "@/lib/chat/request-id";
import type { ChatStreamEvent } from "@/lib/chat/stream";
import {
	gapAfter,
	lastSeq,
	reconcileOptimistic,
	upsertServerMessage,
} from "@/lib/chat/timeline";
import type {
	ChatMessage,
	ChatSide,
	ConnectionState,
	Conversation,
} from "@/lib/chat/types";
import { useChatUnread } from "@/lib/chat/unread-context";
import { useChatStream } from "@/lib/chat/useChatStream";
import { chatSideOf, chatWorkspaceRole } from "@/lib/chat/workspace";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { usePathname } from "@/lib/intl/navigation";
import { uploadImageFile } from "@/lib/media/upload";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

/** 目前頁面所在工作區對應的對話方（賣家中心是店家方）。 */
export function useChatSide(): ChatSide {
	const pathname = usePathname() || "";
	return chatSideOf(chatWorkspaceRole(pathname));
}

/** 對方在線狀態的重新查詢間隔。 */
export const PRESENCE_REFETCH_MS = 30_000;


export function useConversations({
	subscribeToSocket = true,
}: {
	/**
	 * 只有常駐的收件匣 layout 負責即時更新對話列表；ThreadClient 也讀同一個 query 來解析對方，
	 * 但不再多訂一份串流事件。
	 */
	subscribeToSocket?: boolean;
} = {}) {
	const side = useChatSide();
	const status = useAccountStatus();
	const queryIdentity = useQueryIdentity();
	const queryClient = useQueryClient();
	const defaults = useDefaultErrorText();
	const queryKey = queryKeys.chat.conversations(queryIdentity, { side });
	const shouldFetch = status === "signedIn";
	const query = useQuery({
		queryKey,
		queryFn: accountQueryFn(status, () => listConversations(side)),
	});

	const readySeenRef = React.useRef(false);
	useChatStream(side, shouldFetch && subscribeToSocket, (e) => {
		if (e.type === "ready") {
			if (!readySeenRef.current) {
				readySeenRef.current = true;
				return;
			}
		}
		void queryClient.invalidateQueries({ queryKey });
	});

	const items = React.useMemo(
		() => (shouldFetch && query.data) || [],
		[shouldFetch, query.data],
	);

	return {
		items,
		/** 讀取失敗與「沒有對話」分開：失敗時 `error` 有值，`items` 不能當成空清單的證據。 */
		loading: status === "unknown" || query.isPending,
		error: query.error ? errorText(query.error, defaults) : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}


export interface ThreadData {
	messages: ChatMessage[];
	peerReadSeq: number;
}

const EMPTY_THREAD: ThreadData = { messages: [], peerReadSeq: 0 };

interface UseChatThreadResult {
	/** seq 由小到大，樂觀訊息在最後。 */
	messages: ChatMessage[];
	loading: boolean;
	error: string | null;
	connectionState: ConnectionState;
	/** 對方此刻是否在線；`null` 是還不知道。 */
	peerOnline: boolean | null;
	/** 對方已讀到的 seq（我送出的訊息 seq 不大於它就是已讀）。 */
	peerReadSeq: number;
	/** 我是對話的哪一方。 */
	side: ChatSide;
	send: (text: string) => Promise<void>;
	/** 上傳圖片並以圖片訊息送出。 */
	sendImage: (file: File) => Promise<void>;
	refresh: () => Promise<void>;
}

/**
 * 管理一個對話：讀最新一頁訊息、訂閱共用串流（新訊息、可見度改變、對方已讀、Resync）、
 * 以 seq 偵測缺口並用 `after_seq` 補，送出用 `request_id` 去重。
 * `targetId` 是對方的 ID（買家方是店家 ID）；對話不存在（還沒聊過）時沒有訊息，第一則訊息送出時才開對話。
 */
export function useChatThread(
	targetId: string | undefined,
): UseChatThreadResult {
	const side = useChatSide();
	const status = useAccountStatus();
	const queryIdentity = useQueryIdentity();
	const queryClient = useQueryClient();
	const defaults = useDefaultErrorText();
	const { refresh: refreshChatUnread } = useChatUnread();
	const [sendError, setSendError] = React.useState<string | null>(null);

	const conversations = useConversations({ subscribeToSocket: false });
	const conversation: Conversation | undefined = conversations.items.find(
		(c) => c.targetUserId === targetId,
	);
	const conversationId = conversation?.id;
	const conversationsKey = queryKeys.chat.conversations(queryIdentity, {
		side,
	});
	const threadKey = React.useMemo(
		() => queryKeys.chat.thread(queryIdentity, targetId ?? "", { side }),
		[queryIdentity, targetId, side],
	);

	const threadQuery = useQuery({
		queryKey: [...threadKey, conversationId ?? ""],
		queryFn: targetId
			? accountQueryFn(status, async (): Promise<ThreadData> => {
					if (!conversationId) return EMPTY_THREAD;
					const page = await listLatestMessages(conversationId);
					return { messages: page.messages, peerReadSeq: page.peerReadSeq };
				})
			: skipToken,
	});
	const dataKey = React.useMemo(
		() => [...threadKey, conversationId ?? ""] as const,
		[threadKey, conversationId],
	);
	const data = threadQuery.data ?? EMPTY_THREAD;

	const patch = React.useCallback(
		(fn: (d: ThreadData) => ThreadData) => {
			queryClient.setQueryData<ThreadData>(dataKey, (cur) =>
				fn(cur ?? EMPTY_THREAD),
			);
		},
		[queryClient, dataKey],
	);

	const dataRef = React.useRef(data);
	dataRef.current = data;

	const fillGap = React.useCallback(
		async (convId: string, afterSeq: number) => {
			let after = afterSeq;
			try {
				for (;;) {
					const page = await listMessagesAfter(convId, after);
					patch((d) => ({
						messages: page.messages.reduce(
							(acc, m) => upsertServerMessage(acc, m),
							d.messages,
						),
						peerReadSeq: Math.max(d.peerReadSeq, page.peerReadSeq),
					}));
					if (!page.hasMore) return;
					const next = lastSeq(page.messages);
					if (next <= after) throw new Error("gap fill made no progress");
					after = next;
				}
			} catch {
				void queryClient.invalidateQueries({ queryKey: dataKey });
			}
		},
		[patch, queryClient, dataKey],
	);

	const onStreamEvent = React.useCallback(
		(e: ChatStreamEvent) => {
			if (!targetId) return;
			if (e.type === "message") {
				const m = e.message;
				if (!conversationId) {
					void queryClient.invalidateQueries({ queryKey: conversationsKey });
					return;
				}
				if (m.conversationId !== conversationId) return;
				const cur = dataRef.current.messages;
				const gap = m.seq !== null && !e.updated ? gapAfter(cur, m.seq) : null;
				patch((d) => ({ ...d, messages: upsertServerMessage(d.messages, m) }));
				if (gap !== null) void fillGap(conversationId, gap);
			} else if (e.type === "peerRead") {
				if (e.conversationId !== conversationId) return;
				patch((d) => ({
					...d,
					peerReadSeq: Math.max(d.peerReadSeq, e.peerReadSeq),
				}));
			} else if (e.type === "resync") {
				if (e.conversationId && e.conversationId !== conversationId) return;
				void queryClient.invalidateQueries({ queryKey: threadKey });
				void queryClient.invalidateQueries({ queryKey: conversationsKey });
			}
		},
		[
			targetId,
			conversationId,
			patch,
			fillGap,
			queryClient,
			conversationsKey,
			threadKey,
		],
	);
	const { state: connectionState, stream } = useChatStream(
		side,
		status === "signedIn" && Boolean(targetId),
		onStreamEvent,
	);

	const seqHere = lastSeq(data.messages);
	React.useEffect(() => {
		if (stream && conversationId && seqHere > 0) {
			stream.noteSeq(conversationId, seqHere);
		}
	}, [stream, conversationId, seqHere]);

	const peerLastSeq = React.useMemo(() => {
		let max = 0;
		for (const m of data.messages) {
			if (m.sender !== side && m.seq !== null && m.seq > max) max = m.seq;
		}
		return max;
	}, [data.messages, side]);
	const markedRef = React.useRef<{ id: string; seq: number } | null>(null);
	React.useEffect(() => {
		if (!conversationId || peerLastSeq === 0) return;
		const marked = markedRef.current;
		if (marked && marked.id === conversationId && marked.seq >= peerLastSeq) {
			return;
		}
		markedRef.current = { id: conversationId, seq: peerLastSeq };
		markConversationRead(conversationId, seqHere).then(
			refreshChatUnread,
			() => {},
		);
	}, [conversationId, peerLastSeq, seqHere, refreshChatUnread]);

	const presenceQuery = useQuery({
		queryKey: [...threadKey, "presence", conversationId ?? ""],
		queryFn: conversationId
			? accountQueryFn(status, () => fetchPresence(side, [conversationId]))
			: skipToken,
		refetchInterval: PRESENCE_REFETCH_MS,
	});
	const peerOnline =
		conversationId && presenceQuery.data
			? (presenceQuery.data.get(conversationId) ?? null)
			: null;

	const finishSend = React.useCallback(
		(optimisticId: string, server: ChatMessage) => {
			patch((d) => ({
				...d,
				messages: reconcileOptimistic(d.messages, optimisticId, server),
			}));
			if (!conversationId) {
				void queryClient.invalidateQueries({ queryKey: conversationsKey });
			}
		},
		[patch, conversationId, queryClient, conversationsKey],
	);

	const dispatchSend = React.useCallback(
		async (
			optimistic: ChatMessage,
			content: Parameters<typeof sendMessageRpc>[1],
		) => {
			if (!targetId) return;
			setSendError(null);
			const hadData = queryClient.getQueryData(dataKey) !== undefined;
			if (hadData) await queryClient.cancelQueries({ queryKey: dataKey });
			patch((d) => ({
				...d,
				messages: upsertServerMessage(d.messages, optimistic),
			}));
			try {
				const server = await sendMessageRpc(
					{ side, conversationId, peerId: targetId },
					content,
					optimistic.id,
				);
				finishSend(optimistic.id, server);
				if (!hadData) void queryClient.invalidateQueries({ queryKey: dataKey });
			} catch (e) {
				patch((d) => ({
					...d,
					messages: d.messages.filter((m) => m.id !== optimistic.id),
				}));
				setSendError(errorText(e, defaults));
				throw e;
			}
		},
		[
			targetId,
			side,
			conversationId,
			queryClient,
			dataKey,
			patch,
			finishSend,
			defaults,
		],
	);

	const send = React.useCallback(
		async (text: string) => {
			const trimmed = text.trim();
			if (!targetId || !trimmed) return;
			const requestId = newRequestId();
			await dispatchSend(
				{
					id: requestId,
					conversationId: conversationId ?? "",
					seq: null,
					sender: side,
					kind: "text",
					text: trimmed,
					hidden: false,
					erased: false,
					timeMs: Date.now(),
				},
				{ kind: "text", text: trimmed },
			);
		},
		[targetId, conversationId, side, dispatchSend],
	);

	const sendImage = React.useCallback(
		async (file: File) => {
			if (!targetId) return;
			let uploaded: Awaited<ReturnType<typeof uploadImageFile>>;
			try {
				uploaded = await uploadImageFile(file);
			} catch (e) {
				setSendError(errorText(e, defaults));
				throw e;
			}
			await dispatchSend(
				{
					id: newRequestId(),
					conversationId: conversationId ?? "",
					seq: null,
					sender: side,
					kind: "image",
					imageUrl: uploaded.previewUrl,
					hidden: false,
					erased: false,
					timeMs: Date.now(),
				},
				{ kind: "image", stagedToken: uploaded.stagedToken },
			);
		},
		[targetId, conversationId, side, dispatchSend, defaults],
	);

	const loadError = conversations.error
		? conversations.error
		: threadQuery.error
			? errorText(threadQuery.error, defaults)
			: null;

	return {
		messages: data.messages,
		loading: conversations.loading || threadQuery.isPending,
		error: sendError ?? loadError,
		connectionState,
		peerOnline,
		peerReadSeq: data.peerReadSeq,
		side,
		send,
		sendImage,
		refresh: async () => {
			await threadQuery.refetch();
		},
	};
}
