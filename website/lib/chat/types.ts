/**
 * Chat wire types — match `hawkr_buyer/lib/features/chat/data/chat_service.dart`
 * + openapi `/chat/*` shapes.
 */

export type ChatMessageType =
	| "text"
	| "image"
	| "chat"
	| "notify"
	| "system"
	| "chat_ack"
	| "read_receipt"
	| "typing"
	| "start_typing"
	| "stop_typing"
	| "presence"
	| "unread_changed"
	| "heartbeat"
	| string;

export interface ChatMessage {
	id: string;
	senderId: string;
	receiverId: string;
	content: string;
	type: ChatMessageType;
	timestamp: number; // ms epoch
	read: boolean;
	conversationId?: string;
	// seqId 是 64-bit TSID（> JS 2^53），保字串以免精度丟失（用於時間線排序）。
	seqId?: string;
}

export interface Conversation {
	targetUserId: number | string;
	targetNickname: string;
	/** Generic peer avatar (buyer when the current user is a seller). */
	targetAvatar?: string;
	/** B-238 canonical merchant logo (seller peer conversations). */
	logo_url?: string;
	/**
	 * 賣家店面 canonical 路徑短碼（`sellers.store_code`）。wire `storeCode`。
	 *
	 * `targetUserId` 在賣家側是 store_id（內部代理鍵），賣家頁 canonical 網址走
	 * store_code。im-service 為了 storeName/logo 本來就會呼叫 GetSellerBatch，
	 * 值同源；對象是買家或取不到時為空（→ undefined，不連結）。
	 */
	storeCode?: string;
	lastMessage: string;
	lastMessageTime: number;
	lastMessageType?: string;
	unreadCount: number;
}

export type ConnectionState =
	| "disconnected"
	| "connecting"
	| "connected"
	| "reconnecting";

/* ===== WebSocket envelope ===== */

export interface WSPayload {
	type: ChatMessageType;
	from?: string | number;
	to?: string | number;
	fromUserId?: string | number;
	toUserId?: string | number;
	content?: string;
	timestamp?: number;
	id?: string | number;
	messageId?: string | number;
	conversationId?: string;
	seqId?: string | number;
	online?: boolean;
	peerRole?: number;
	lastSeenAt?: number;
	conversations?: Record<string, string | number>;
	unreadCount?: number | string;
	role?: string | number;
}

export function parseWSMessage(raw: WSPayload): ChatMessage {
	return {
		id: String(raw.id ?? raw.messageId ?? ""),
		senderId: String(raw.fromUserId ?? raw.from ?? ""),
		receiverId: String(raw.toUserId ?? raw.to ?? ""),
		content: String(raw.content ?? ""),
		type: raw.type,
		timestamp: Number(raw.timestamp ?? Date.now()),
		read: false,
		conversationId: raw.conversationId,
		seqId: raw.seqId == null ? undefined : String(raw.seqId),
	};
}
