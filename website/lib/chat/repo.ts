/**
 * 聊天的資料層（`ChatAppService`）。買家方與店家方依集合分成兩組方法，由 `side` 選；
 * 讀取失敗一律拋出，不回空陣列（「讀不到」與「沒有對話」是兩件事）。
 */

import { chatAppClient } from "@/lib/connect/chat";
import { conversationFromProto, messageFromProto } from "./map";
import type { ChatMessage, ChatSide, Conversation } from "./types";

type Client = () => ReturnType<typeof chatAppClient>;

/** 對話列表一頁的筆數與最多讀幾頁（上限 500 個對話）。 */
export const CONVERSATIONS_PAGE_SIZE = 50;
const CONVERSATIONS_MAX_PAGES = 10;
/** 訊息一頁的筆數。 */
export const MESSAGES_PAGE_SIZE = 50;

export async function listConversations(
	side: ChatSide,
	client: Client = chatAppClient,
): Promise<Conversation[]> {
	const out: Conversation[] = [];
	let pageToken = "";
	for (let page = 0; page < CONVERSATIONS_MAX_PAGES; page++) {
		const req = { pageSize: CONVERSATIONS_PAGE_SIZE, pageToken };
		const res =
			side === "store"
				? await client().listStoreConversations(req)
				: await client().listConversations(req);
		out.push(...res.conversations.map((c) => conversationFromProto(c, side)));
		pageToken = res.nextPageToken;
		if (!pageToken) break;
	}
	return out;
}

export interface MessagePage {
	/** seq 由小到大（舊的在前）。 */
	messages: ChatMessage[];
	hasMore: boolean;
	peerReadSeq: number;
}

/** 最新一頁（預設）。 */
export async function listLatestMessages(
	conversationId: string,
	client: Client = chatAppClient,
): Promise<MessagePage> {
	const res = await client().listMessages({
		conversationId,
		pageSize: MESSAGES_PAGE_SIZE,
	});
	return {
		messages: res.messages.map(messageFromProto).reverse(),
		hasMore: res.hasMore,
		peerReadSeq: Number(res.peerReadSeq),
	};
}

/** 斷線補洞：seq 大於 `afterSeq` 的訊息，伺服器已是由小到大。 */
export async function listMessagesAfter(
	conversationId: string,
	afterSeq: number,
	client: Client = chatAppClient,
): Promise<MessagePage> {
	const res = await client().listMessages({
		conversationId,
		pageSize: MESSAGES_PAGE_SIZE,
		afterSeq: BigInt(afterSeq),
	});
	return {
		messages: res.messages.map(messageFromProto),
		hasMore: res.hasMore,
		peerReadSeq: Number(res.peerReadSeq),
	};
}

/** 送出的內容：文字，或已上傳的圖片（staged token）。 */
export type OutgoingContent =
	| { kind: "text"; text: string }
	| { kind: "image"; stagedToken: string };

function contentOf(c: OutgoingContent) {
	return c.kind === "text"
		? { content: { case: "text" as const, value: c.text } }
		: { content: { case: "imageStagedToken" as const, value: c.stagedToken } };
}

export interface SendTarget {
	side: ChatSide;
	/** 既有的對話 ID；沒有時以對方的 ID 開新對話。 */
	conversationId?: string;
	/** 對方的 ID（買家方是店家 ID，店家方是買家 ID）。 */
	peerId: string;
}

/**
 * 送出一則訊息。`requestId` 由呼叫端產生（UUIDv4），同一次送出重送回第一次寫入的那則（去重）。
 */
export async function sendMessage(
	target: SendTarget,
	content: OutgoingContent,
	requestId: string,
	client: Client = chatAppClient,
): Promise<ChatMessage> {
	const body = { requestId, content: contentOf(content) };
	let res: { message?: Parameters<typeof messageFromProto>[0] };
	if (target.conversationId) {
		res = await client().sendMessage({
			conversationId: target.conversationId,
			...body,
		});
	} else if (target.side === "store") {
		res = await client().sendMessageToBuyer({
			buyerId: target.peerId,
			...body,
		});
	} else {
		res = await client().sendMessageToStore({
			storeId: target.peerId,
			...body,
		});
	}
	if (!res.message) throw new Error("chat: send returned no message");
	return messageFromProto(res.message);
}

export async function markConversationRead(
	conversationId: string,
	uptoSeq: number,
	client: Client = chatAppClient,
): Promise<void> {
	await client().markConversationRead({
		conversationId,
		uptoSeq: BigInt(uptoSeq),
	});
}

export async function markAllRead(
	side: ChatSide,
	client: Client = chatAppClient,
): Promise<void> {
	if (side === "store") await client().markAllStoreRead({});
	else await client().markAllRead({});
}

export async function fetchUnreadCount(
	side: ChatSide,
	client: Client = chatAppClient,
): Promise<number> {
	const res =
		side === "store"
			? await client().getStoreUnreadCount({})
			: await client().getUnreadCount({});
	return Number(res.unreadCount);
}

/** 對話的對方此刻是否在線（一次最多 50 個對話）。 */
export async function fetchPresence(
	side: ChatSide,
	conversationIds: string[],
	client: Client = chatAppClient,
): Promise<Map<string, boolean>> {
	const req = { conversationIds };
	const res =
		side === "store"
			? await client().getStorePresence(req)
			: await client().getPresence(req);
	return new Map(res.presences.map((p) => [p.conversationId, p.online]));
}
